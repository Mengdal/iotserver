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

package yiqiweixin

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/kirinlabs/HttpRequest"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/i18n"
	"iotserver/internal/pkg/logger"
)

// 告警通知正文模板：中文原文即词条 ID，英文译文在 internal/pkg/i18n/locales/en.go
const (
	tplDataAlert = `设备已触发数据告警
         >设备名称:<font color=\"comment\">{{.p1}}</font>
         >告警规则:<font color=\"comment\">{{.p2}}</font>
		 >属性名称:<font color=\"comment\">{{.p3}}</font>
 		 >触发时间:<font color=\"comment\">{{.p4}}</font>
      `
	tplDataAlertWithValue = `设备已触发数据告警
         >设备名称:<font color=\"comment\">{{.p1}}</font>
         >告警规则:<font color=\"comment\">{{.p2}}</font>
		 >属性名称:<font color=\"comment\">{{.p3}}</font>
         >当前属性值:<font color=\"comment\">{{.p4}}</font>
 		 >触发时间:<font color=\"comment\">{{.p5}}</font>
      `
	tplEventAlert = `设备已触发事件告警
         >设备名称:<font color=\"comment\">{{.p1}}</font>
         >告警规则:<font color=\"comment\">{{.p2}}</font>
		 >事件名称:<font color=\"comment\">{{.p3}}</font>
 	     >触发时间:<font color=\"comment\">{{.p4}}</font>
         `
	tplEventAlertWithValue = `设备已触发事件告警
         >设备名称:<font color=\"comment\">{{.p1}}</font>
         >告警规则:<font color=\"comment\">{{.p2}}</font>
		 >事件名称:<font color=\"comment\">{{.p3}}</font>
         >事件内容:<font color=\"comment\">{{.p4}}</font>
 	     >触发时间:<font color=\"comment\">{{.p5}}</font>
         `
	tplStatusAlert = `设备已触发状态告警
         >设备名称:<font color=\"comment\">{{.p1}}</font>
         >告警规则:<font color=\"comment\">{{.p2}}</font>
		 >设备状态:<font color=\"comment\">{{.p3}}</font>`
)

type WeixinClient struct {
	lc logger.LoggingClient
	p  *di.Container
}

func NewWeiXinClient(lc logger.LoggingClient, p *di.Container) *WeixinClient {
	return &WeixinClient{
		lc: lc,
		p:  p,
	}
}

type QiYeWeiXinTemplate struct {
	Msgtype  string   `json:"msgtype"`
	Markdown Markdown `json:"markdown"`
}

type Markdown struct {
	Content string `json:"content"`
}

func (d *WeixinClient) generateWeiXinTemplate(rule models.Rule, device models.Device, product models.Product, ruleName string, lang string) QiYeWeiXinTemplate {
	var qt QiYeWeiXinTemplate
	qt.Msgtype = "markdown"
	now := time.Now().Format("2006-01-02 15:04:05")
	switch rule.Trigger {
	case constants.DeviceDataTrigger:
		var code string
		var propertyName string
		var data interface{}
		var unit string
		if rule.Option["code"] != "" {
			for _, property := range product.Properties {
				if property.Code == rule.Option["code"] {
					code = property.Code
					propertyName = property.Name
					break
				}
			}
			var persisterReq dtos.ThingModelPropertyDataRequest
			persisterReq.ThingModelDataBaseRequest.Last = true
			persisterReq.DeviceId = device.Id
			persisterReq.Code = rule.Option["code"]
		}
		property := propertyName + "[" + code + "]"
		if data == nil {
			qt.Markdown.Content = i18n.Trans(lang, tplDataAlert, map[string]interface{}{
				"p1": device.Name,
				"p2": ruleName,
				"p3": property,
				"p4": now,
			})
		} else {
			qt.Markdown.Content = i18n.Trans(lang, tplDataAlertWithValue, map[string]interface{}{
				"p1": device.Name,
				"p2": ruleName,
				"p3": property,
				"p4": fmt.Sprintf("%v(%s)", data, unit),
				"p5": now,
			})
		}
	case constants.DeviceEventTrigger:
		var code string
		var eventName string
		var eventType string
		var data interface{}
		if rule.Option["code"] != "" {
			for _, event := range product.Events {
				if event.Code == rule.Option["code"] {
					code = event.Code
					eventName = event.Name
					eventType = event.EventType
					break
				}
			}
			var persisterReq dtos.ThingModelEventDataRequest
			persisterReq.ThingModelDataBaseRequest.Last = true
			persisterReq.DeviceId = device.Id
			persisterReq.EventCode = rule.Option["code"]
			persisterReq.EventType = eventType
		}
		event := eventName + "[" + code + "]"
		if data == nil {
			qt.Markdown.Content = i18n.Trans(lang, tplEventAlert, map[string]interface{}{
				"p1": device.Name,
				"p2": ruleName,
				"p3": event,
				"p4": now,
			})
		} else {
			qt.Markdown.Content = i18n.Trans(lang, tplEventAlertWithValue, map[string]interface{}{
				"p1": device.Name,
				"p2": ruleName,
				"p3": event,
				"p4": data,
				"p5": now,
			})
		}
	case constants.DeviceStatusTrigger:
		qt.Markdown.Content = i18n.Trans(lang, tplStatusAlert, map[string]interface{}{
			"p1": device.Name,
			"p2": ruleName,
			"p3": i18n.Trans(lang, string(device.Status), nil),
		})
	}
	return qt
}

func (d *WeixinClient) Send(webhook string, rule models.Rule, device models.Device, product models.Product, ruleName string, lang string) {
	if webhook == "" {
		return
	}
	req := HttpRequest.NewRequest()
	req.JSON()
	context, _ := json.Marshal(d.generateWeiXinTemplate(rule, device, product, ruleName, lang))
	d.lc.Info("weixin sent context", string(context))
	resp, err := req.Post(webhook, context)
	if err != nil {
		d.lc.Errorf("weixin send alert message error:", err.Error())
	}
	body, err := resp.Body()
	if err != nil {
		return
	}
	d.lc.Debug("weixin send message", string(body))
}
