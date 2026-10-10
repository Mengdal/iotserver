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
	"net/http"
	"strings"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/logger"
)

const (
	ApiRuleInfoRoute    = "/rules/%s"
	ApiRuleStatusRoute  = "/rules/%s/status"
	ApiRuleCreateRoute  = "/rules"
	ApiRuleStopRoute    = "/rules/%s/stop"
	ApiRuleStartRoute   = "/rules/%s/start"
	ApiRuleRestartRoute = "/rules/%s/restart"
	ApiRuleDeleteRoute  = "/rules/%s"
	ApiRuleUpdateRoute  = "/rules/%s"
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

//RuleExist 规则是否存在
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

//CreateRule 创建规则
func (c *ekuiperClient) CreateRule(ctx context.Context, actions []dtos.Actions, ruleId string, sql string) error {
	var createRule dtos.CreateRule
	cr := createRule.BuildCreateRuleParam(actions, ruleId, sql)
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

//UpdateRule  更新规则
func (c *ekuiperClient) UpdateRule(ctx context.Context, actions []dtos.Actions, ruleId string, sql string) error {
	var createRule dtos.CreateRule
	cr := createRule.BuildCreateRuleParam(actions, ruleId, sql)
	b, _ := json.Marshal(cr)
	status, body, err := c.do(ctx, http.MethodPut, fmt.Sprintf(ApiRuleUpdateRoute, ruleId), b)
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

//GetRuleStats 获取规则状态
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

//StartRule 启动规则
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

//StopRule 停止规则
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

//RestartRule 重启规则
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

//DeleteRule 删除规则（规则不存在视为幂等成功）
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
