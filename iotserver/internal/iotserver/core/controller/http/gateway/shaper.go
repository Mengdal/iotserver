//
// Copyright (c) 2024 IOTServer
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
)

// Shaper 相关配置都支持用环境变量覆盖，便于不把密钥/环境地址写进仓库
const (
	shaperBaseUrlEnv        = "IOTSERVER_SHAPER_BASEURL"
	shaperBrowserBaseUrlEnv = "IOTSERVER_SHAPER_BROWSERBASEURL"
	shaperApiKeyEnv         = "IOTSERVER_SHAPER_APIKEY"
	shaperDashboardIdEnv    = "IOTSERVER_SHAPER_DASHBOARDID"
	shaperHomeBoardIdEnv    = "IOTSERVER_SHAPER_HOMEDASHBOARDID"
)

// shaperSetting 解析顺序：环境变量 > configuration.toml > 本地未入库文件（Shaper.LocalEnvFile）
func (ctl *controller) shaperSetting(envKey, cfgValue string) string {
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		return v
	}
	if v := strings.TrimSpace(cfgValue); v != "" {
		return v
	}
	return ctl.shaperLocalEnv()[envKey]
}

// shaperLocalEnv 读取本地未入库的 KEY=VALUE 文件（默认 .env.shaper）。
// 这样无论谁以什么方式重启后端，都能拿到 Shaper 的 API Key / 看板 ID，不依赖启动时的环境变量。
func (ctl *controller) shaperLocalEnv() map[string]string {
	path := strings.TrimSpace(ctl.cfg.Shaper.LocalEnvFile)
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	out := make(map[string]string)
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key != "" {
			out[key] = value
		}
	}
	return out
}

// ShaperEmbed 返回前端嵌入 Shaper 看板所需的信息：实例地址、看板 ID、短期 JWT。
// API Key 只留在服务端，通过 Shaper 的 /api/auth/token 换取 JWT 后下发。
// @Tags    可视化-Shaper
// @Summary 查询 Shaper 看板嵌入信息
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/shaper/embed [get]
// @Security ApiKeyAuth
func (ctl *controller) ShaperEmbed(c *gin.Context) {
	lc := ctl.lc
	cfg := ctl.cfg.Shaper

	// 服务端用它去换 JWT（容器里可能是 http://shaper:5454）
	baseUrl := strings.TrimRight(ctl.shaperSetting(shaperBaseUrlEnv, cfg.BaseUrl), "/")
	// 浏览器用它加载 SDK（可能是 http://127.0.0.1:5454），没配就与 baseUrl 相同
	browserBaseUrl := strings.TrimRight(ctl.shaperSetting(shaperBrowserBaseUrlEnv, cfg.BrowserBaseUrl), "/")
	if browserBaseUrl == "" {
		browserBaseUrl = baseUrl
	}
	apiKey := ctl.shaperSetting(shaperApiKeyEnv, cfg.ApiKey)
	dashboardId := strings.TrimSpace(c.Query("dashboardId"))
	if dashboardId == "" {
		// 看板管理里设了「默认看板」就优先用它，其次才是配置
		dashboardId = ctl.activeVizBoardId("default")
	}
	if dashboardId == "" {
		dashboardId = ctl.shaperSetting(shaperDashboardIdEnv, cfg.DashboardId)
	}
	homeDashboardId := ctl.activeVizBoardId("home")
	if homeDashboardId == "" {
		homeDashboardId = ctl.shaperSetting(shaperHomeBoardIdEnv, cfg.HomeDashboardId)
	}

	enabled := cfg.Enabled && baseUrl != "" && apiKey != "" && dashboardId != ""
	resp := gin.H{
		"enabled":         enabled,
		"baseUrl":         browserBaseUrl,
		"dashboardId":     dashboardId,
		"homeDashboardId": homeDashboardId,
	}
	if !enabled {
		// 未配置时返回 enabled=false，前端展示引导文案而不是报错
		httphelper.ResultSuccess(resp, c.Writer, lc)
		return
	}

	// 支持看板变量（vars 以 JSON 字符串传入），可用于按用户/组织过滤数据
	var variables map[string]interface{}
	if raw := strings.TrimSpace(c.Query("vars")); raw != "" {
		if err := json.Unmarshal([]byte(raw), &variables); err != nil {
			httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("invalid vars: %s", err.Error())), c.Writer, lc)
			return
		}
	}

	jwtToken, err := ctl.shaperEmbedToken(baseUrl, apiKey, dashboardId, variables)
	if err != nil {
		ctl.lc.Error(fmt.Sprintf("shaper embed token failed: %s", err.Error()))
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, err), c.Writer, lc)
		return
	}

	resp["jwt"] = jwtToken
	httphelper.ResultSuccess(resp, c.Writer, lc)
}

// shaperEmbedToken 调用 Shaper 的 /api/auth/token，用 API Key 换指定看板的短期 JWT
func (ctl *controller) shaperEmbedToken(baseUrl, apiKey, dashboardId string, variables map[string]interface{}) (string, error) {
	payload := map[string]interface{}{
		"token":       apiKey,
		"dashboardId": dashboardId,
	}
	if len(variables) > 0 {
		payload["variables"] = variables
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, baseUrl+"/api/auth/token", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("shaper returned %s: %s", res.Status, strings.TrimSpace(string(raw)))
	}

	var out struct {
		Jwt string `json:"jwt"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.Jwt == "" {
		return "", fmt.Errorf("shaper returned empty jwt")
	}
	return out.Jwt, nil
}
