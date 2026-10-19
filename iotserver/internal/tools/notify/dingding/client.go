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

package dingding

import (
	"bytes"

	"github.com/kirinlabs/HttpRequest"

	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/i18n"
	"iotserver/internal/pkg/logger"
)

// dingDingAlertTemplate 告警通知正文。
// 中文原文就是词条 ID（见 internal/pkg/i18n），英文译文在 locales/en.go；
// lang 为空时按默认语言输出中文。
const dingDingAlertTemplate = "# 设备触发告警 \n\n设备名称：{{.p1}} \n\n告警规则名称：{{.p2}} \n\n触发方式：{{.p3}} \n\n"

type DingDingClient struct {
	lc logger.LoggingClient
}

func NewDingDingClient(lc logger.LoggingClient) *DingDingClient {
	return &DingDingClient{
		lc: lc,
	}
}

func dingDingTemplate(trigger constants.Trigger, deviceName string, ruleName string, lang string) string {
	return i18n.Trans(lang, dingDingAlertTemplate, map[string]interface{}{
		"p1": deviceName,
		"p2": ruleName,
		"p3": i18n.Trans(lang, string(trigger), nil),
	})
}

func (d *DingDingClient) Send(webhook string, rule models.Rule, device models.Device, product models.Product, ruleName string, lang string) {
	req := HttpRequest.NewRequest()
	req.JSON()
	resp, err := req.Post(webhook, bytes.NewBuffer([]byte(dingDingTemplate(rule.Trigger, device.Name, ruleName, lang))))
	if err != nil {
		d.lc.Errorf("dingding send alert message error:", err.Error())
	}
	body, err := resp.Body()
	if err != nil {
		return
	}
	d.lc.Debug("dingding send message", string(body))
}
