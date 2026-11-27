/*******************************************************************************
 * Copyright 2017.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *******************************************************************************/

package ekuiperclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/logger"
)

const (
	ApiRuleInfoRoute       = "/rules/%s"
	ApiRuleStatusRoute     = "/rules/%s/status"
	ApiRuleCreateRoute     = "/rules"
	ApiRuleStopRoute       = "/rules/%s/stop"
	ApiRuleStartRoute      = "/rules/%s/start"
	ApiRuleRestartRoute    = "/rules/%s/restart"
	ApiRuleDeleteRoute     = "/rules/%s"
	ApiRuleUpdateRoute     = "/rules/%s"
	ApiRuleValidateRoute   = "/rules/validate"
	ApiRuleTestCreateRoute = "/ruletest"
	ApiRuleTestStartRoute  = "/ruletest/%s/start"
	ApiRuleTestStopRoute   = "/ruletest/%s"
	// RuleTestPort eKuiper 试运行 SSE 默认端口（/ruletest 未返回端口时使用）
	RuleTestPort = 10081
)

// requestTimeout 单次调用 eKuiper 的超时上限。
// 调用方传入的 ctx 若预算更小则以其为准（例如状态同步的 3s 预算）。
const requestTimeout = 5 * time.Second

type ekuiperClient struct {
	baseUrl string
	lc      logger.LoggingClient
}

func New(baseUrl string, lc logger.LoggingClient) EkuiperClient {
	return &ekuiperClient{
		lc:      lc,
		baseUrl: strings.TrimRight(baseUrl, "/"),
	}
}

// do 统一的请求入口：真正使用 ctx（可取消/可超时），并限制响应体读取大小。
// 历史实现用的 kirinlabs/HttpRequest 既不使用 ctx 也无法正确设置超时。
func (c *ekuiperClient) do(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseUrl+path, reader)
	if err != nil {
		return 0, nil, err
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	// eKuiper 的响应体很小，这里限制到 1MB 防止异常响应撑爆内存
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, respBody, nil
}

// RuleExist 规则是否存在
func (c *ekuiperClient) RuleExist(ctx context.Context, ruleId string) (bool, error) {
	status, body, err := c.do(ctx, http.MethodGet, fmt.Sprintf(ApiRuleInfoRoute, ruleId), nil)
	if err != nil {
		return false, err
	}
	switch status {
	case http.StatusOK:
		return true, nil
	case http.StatusBadRequest:
		return false, errort.NewCommonEdgeX(errort.AlertRuleNotExist, string(body), nil)
	case http.StatusNotFound:
		return false, nil
	}
	return false, errort.NewCommonEdgeX(errort.SystemErrorCode, "", nil)
}

// CreateRule 创建规则（兼容旧调用方，不含 options/tags）
func (c *ekuiperClient) CreateRule(ctx context.Context, actions []dtos.Actions, ruleId string, sql string) error {
	return c.CreateRuleDefinition(ctx, dtos.EkuiperRule{Id: ruleId, Name: ruleId, Sql: sql, Actions: actions})
}

// UpdateRule 更新规则（兼容旧调用方，不含 options/tags）
func (c *ekuiperClient) UpdateRule(ctx context.Context, actions []dtos.Actions, ruleId string, sql string) error {
	return c.UpdateRuleDefinition(ctx, dtos.EkuiperRule{Id: ruleId, Name: ruleId, Sql: sql, Actions: actions})
}

// CreateRuleDefinition 创建完整规则（SQL + 多动作 + options + tags）。
func (c *ekuiperClient) CreateRuleDefinition(ctx context.Context, rule dtos.EkuiperRule) error {
	cr := dtos.CreateRule{
		Triggered: false,
		Id:        rule.Id,
		Name:      rule.Name,
		Sql:       rule.Sql,
		Actions:   rule.Actions,
		Options:   rule.Options,
		Tags:      rule.Tags,
	}
	if cr.Name == "" {
		cr.Name = rule.Id
	}
	b, _ := json.Marshal(cr)
	status, body, err := c.do(ctx, http.MethodPost, ApiRuleCreateRoute, b)
	if err != nil {
		return err
	}
	switch status {
	case http.StatusCreated:
		if strings.Contains(string(body), "successfully") {
			return nil
		}
	case http.StatusBadRequest:
		return errort.NewCommonEdgeX(errort.InvalidRuleJson, string(body), nil)
	case http.StatusNotFound:
		return errort.NewCommonEdgeX(errort.EkuiperNotFindRule, "Rule engine not found rule", nil)
	}
	return errort.NewCommonEdgeX(errort.SystemErrorCode, "", nil)
}

// UpdateRuleDefinition 更新完整规则。
//
// eKuiper 的 PUT /rules/{id} 会按请求体里的 triggered 决定更新后是否继续运行；
// 固定传 false 会把正在运行的规则停掉（告警/场景编辑时用户会看到规则“自动关闭”）。
// 更新定义本身不应该改变运行状态，所以先读取当前状态，再原样带上。
func (c *ekuiperClient) UpdateRuleDefinition(ctx context.Context, rule dtos.EkuiperRule) error {
	triggered := false
	if stats, err := c.GetRuleStats(ctx, rule.Id); err == nil {
		if status, _ := stats["status"].(string); status == "running" || status == "starting" {
			triggered = true
		}
	}
	cr := dtos.CreateRule{
		Triggered: triggered,
		Id:        rule.Id,
		Name:      rule.Name,
		Sql:       rule.Sql,
		Actions:   rule.Actions,
		Options:   rule.Options,
		Tags:      rule.Tags,
	}
	if cr.Name == "" {
		cr.Name = rule.Id
	}
	b, _ := json.Marshal(cr)
	status, body, err := c.do(ctx, http.MethodPut, fmt.Sprintf(ApiRuleUpdateRoute, rule.Id), b)
	if err != nil {
		return err
	}
	switch status {
	case http.StatusOK:
		if strings.Contains(string(body), "successfully") {
			return nil
		}
	case http.StatusBadRequest:
		return errort.NewCommonEdgeX(errort.InvalidRuleJson, string(body), nil)
	case http.StatusNotFound:
		return errort.NewCommonEdgeX(errort.EkuiperNotFindRule, "Rule engine not found rule", nil)
	}
	return errort.NewCommonEdgeX(errort.SystemErrorCode, "", nil)
}

// ValidateRule 调用 eKuiper /rules/validate 做 SQL 语法/语义校验。
func (c *ekuiperClient) ValidateRule(ctx context.Context, sql string) error {
	body, _ := json.Marshal(map[string]interface{}{
		"id":      "__rule_validate__",
		"sql":     sql,
		"actions": []map[string]interface{}{{"log": map[string]interface{}{}}},
	})
	status, respBody, err := c.do(ctx, http.MethodPost, ApiRuleValidateRoute, body)
	if err != nil {
		return err
	}
	switch status {
	case http.StatusOK:
		return nil
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return errort.NewCommonEdgeX(errort.InvalidRuleJson, string(respBody), nil)
	default:
		return errort.NewCommonEdgeX(errort.SystemErrorCode, string(respBody), nil)
	}
}

// CreateRuleTest 创建 eKuiper 试运行规则。
func (c *ekuiperClient) CreateRuleTest(ctx context.Context, req dtos.RuleTestRequest) (string, int, error) {
	body, _ := json.Marshal(req)
	status, respBody, err := c.do(ctx, http.MethodPost, ApiRuleTestCreateRoute, body)
	if err != nil {
		return "", 0, err
	}
	if status != http.StatusOK {
		return "", 0, errort.NewCommonEdgeX(errort.InvalidRuleJson, string(respBody), nil)
	}
	var resp dtos.RuleTestResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", 0, errort.NewCommonEdgeX(errort.InvalidRuleJson, string(respBody), nil)
	}
	return resp.Id, resp.Port, nil
}

// StartRuleTest 启动试运行规则。
func (c *ekuiperClient) StartRuleTest(ctx context.Context, id string) error {
	status, respBody, err := c.do(ctx, http.MethodPost, fmt.Sprintf(ApiRuleTestStartRoute, id), nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return errort.NewCommonEdgeX(errort.SystemErrorCode, string(respBody), nil)
	}
	return nil
}

// DeleteRuleTest 停止并删除试运行规则。
func (c *ekuiperClient) DeleteRuleTest(ctx context.Context, id string) error {
	status, respBody, err := c.do(ctx, http.MethodDelete, fmt.Sprintf(ApiRuleTestStopRoute, id), nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusNotFound {
		return errort.NewCommonEdgeX(errort.SystemErrorCode, string(respBody), nil)
	}
	return nil
}

// OpenRuleTestStream 打开 eKuiper 试运行 SSE 输出（独立测试端口，长连接不设总超时）。
// port 取 /ruletest 创建试运行规则时返回的端口；<=0 时回落到 RuleTestPort。
func (c *ekuiperClient) OpenRuleTestStream(ctx context.Context, id string, port int) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.testBaseURL(port)+"/test/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, errort.NewCommonEdgeX(errort.SystemErrorCode, fmt.Sprintf("rule test stream http %d", resp.StatusCode), nil)
	}
	return resp.Body, nil
}

// testBaseURL 将 eKuiper REST 地址换成试运行 SSE 端口（默认 10081）。
func (c *ekuiperClient) testBaseURL(port int) string {
	if port <= 0 {
		port = RuleTestPort
	}
	u, err := url.Parse(c.baseUrl)
	if err != nil {
		return c.baseUrl
	}
	u.Host = net.JoinHostPort(u.Hostname(), strconv.Itoa(port))
	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/")
}

// GetRuleStats 获取规则状态
func (c *ekuiperClient) GetRuleStats(ctx context.Context, ruleId string) (map[string]interface{}, error) {
	ruleStatusResponse := make(map[string]interface{})
	status, body, err := c.do(ctx, http.MethodGet, fmt.Sprintf(ApiRuleStatusRoute, ruleId), nil)
	if err != nil {
		return ruleStatusResponse, err
	}
	if status == http.StatusOK {
		if err = json.Unmarshal(body, &ruleStatusResponse); err != nil {
			return ruleStatusResponse, errort.NewCommonEdgeX(errort.InvalidRuleJson, string(body), nil)
		}
		return ruleStatusResponse, nil
	}
	return ruleStatusResponse, errort.NewCommonEdgeX(errort.SystemErrorCode, "", nil)
}

// StartRule 启动规则
func (c *ekuiperClient) StartRule(ctx context.Context, ruleId string) error {
	status, body, err := c.do(ctx, http.MethodPost, fmt.Sprintf(ApiRuleStartRoute, ruleId), nil)
	if err != nil {
		return err
	}
	switch status {
	case http.StatusOK:
		if strings.Contains(string(body), "started") {
			return nil
		}
	case http.StatusBadRequest:
		return errort.NewCommonEdgeX(errort.InvalidRuleJson, string(body), nil)
	case http.StatusNotFound:
		return errort.NewCommonEdgeX(errort.EkuiperNotFindRule, "Rule engine not found rule", nil)
	}
	return errort.NewCommonEdgeX(errort.SystemErrorCode, "", nil)
}

// StopRule 停止规则
func (c *ekuiperClient) StopRule(ctx context.Context, ruleId string) error {
	status, body, err := c.do(ctx, http.MethodPost, fmt.Sprintf(ApiRuleStopRoute, ruleId), nil)
	if err != nil {
		return err
	}
	switch status {
	case http.StatusOK:
		if strings.Contains(string(body), "stopped") {
			return nil
		}
	case http.StatusBadRequest:
		return errort.NewCommonEdgeX(errort.InvalidRuleJson, string(body), nil)
	case http.StatusNotFound:
		return errort.NewCommonEdgeX(errort.EkuiperNotFindRule, "Rule engine not found rule", nil)
	}
	return errort.NewCommonEdgeX(errort.SystemErrorCode, "", nil)
}

// RestartRule 重启规则
func (c *ekuiperClient) RestartRule(ctx context.Context, ruleId string) error {
	status, body, err := c.do(ctx, http.MethodPost, fmt.Sprintf(ApiRuleRestartRoute, ruleId), nil)
	if err != nil {
		return err
	}
	switch status {
	case http.StatusOK:
		if strings.Contains(string(body), "restarted") {
			return nil
		}
	case http.StatusBadRequest:
		return errort.NewCommonEdgeX(errort.InvalidRuleJson, string(body), nil)
	case http.StatusNotFound:
		return errort.NewCommonEdgeX(errort.EkuiperNotFindRule, "Rule engine not found rule", nil)
	}
	return errort.NewCommonEdgeX(errort.SystemErrorCode, "", nil)
}

// DeleteRule 删除规则（规则不存在视为幂等成功）
func (c *ekuiperClient) DeleteRule(ctx context.Context, ruleId string) error {
	status, body, err := c.do(ctx, http.MethodDelete, fmt.Sprintf(ApiRuleDeleteRoute, ruleId), nil)
	if err != nil {
		return err
	}
	switch status {
	case http.StatusOK:
		if strings.Contains(string(body), "dropped") {
			return nil
		}
	case http.StatusBadRequest:
		return errort.NewCommonEdgeX(errort.InvalidRuleJson, string(body), nil)
	case http.StatusNotFound:
		return nil
	}
	return errort.NewCommonEdgeX(errort.SystemErrorCode, "", nil)
}
