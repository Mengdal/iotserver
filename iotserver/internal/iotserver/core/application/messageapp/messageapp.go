/*******************************************************************************
 * Copyright 2017 Dell Inc.
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

package messageapp

import (
	"context"
	"encoding/json"
	"github.com/LMgateways/iotserver-driver-proto/drivercommon"
	"github.com/kirinlabs/HttpRequest"
	"iotserver/internal/dtos"
	coreContainer "iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"strconv"
	"strings"
	"time"

	pkgMQTT "iotserver/internal/tools/mqttclient"
)

type MessageApp struct {
	dic               *di.Container
	lc                logger.LoggingClient
	dbClient          interfaces.DBClient
	ekuiperMqttClient pkgMQTT.MQTTClient
	ekuiperaddr       string
}

func NewMessageApp(dic *di.Container, ekuiperaddr string) *MessageApp {
	lc := container.LoggingClientFrom(dic.Get)
	dbClient := coreContainer.DBClientFrom(dic.Get)
	msgApp := &MessageApp{
		dic:         dic,
		dbClient:    dbClient,
		lc:          lc,
		ekuiperaddr: ekuiperaddr,
	}
	mqttClient := msgApp.connectMQTT()
	msgApp.ekuiperMqttClient = mqttClient
	msgApp.initeKuiperStreams()
	return msgApp
}

func (tmq *MessageApp) initeKuiperStreams() {
	req := HttpRequest.NewRequest()
	r := make(map[string]string)
	r["sql"] = "CREATE STREAM mqtt_stream () WITH (DATASOURCE=\"eventbus/in\", FORMAT=\"JSON\",SHARED = \"true\")"
	b, _ := json.Marshal(r)
	resp, err := req.Post(tmq.ekuiperaddr, b)
	if err != nil {
		tmq.lc.Errorf("init ekuiper stream failed error:%+v", err.Error())
		return
	}

	if resp.StatusCode() == 201 {
		body, err := resp.Body()
		if err != nil {
			tmq.lc.Errorf("init ekuiper stream failed error:%+v", err.Error())
			return
		}
		if strings.Contains(string(body), "created") {
			tmq.lc.Infof("init ekuiper stream success")
			return
		}
	} else if resp.StatusCode() == 400 {
		body, err := resp.Body()
		tmq.lc.Infof("init ekuiper stream body", string(body))
		if err != nil {
			tmq.lc.Errorf("init ekuiper stream failed error:%+v", err.Error())
			return
		}

		if strings.Contains(string(body), "already exists") {
			tmq.lc.Infof("init ekuiper stream plug success")
			return
		}
	}
}

// tenantIDFor 解析消息所属租户：优先取上下文，上下文没有时按设备归属补查。
func (tmq *MessageApp) tenantIDFor(ctx context.Context, deviceID string) string {
	if tc := tenant.FromContext(ctx); tc.TenantID != "" {
		return tc.TenantID
	}
	if deviceID == "" {
		return ""
	}
	device, err := tmq.dbClient.WithCtx(tenant.Platform(context.Background())).DeviceById(deviceID)
	if err != nil {
		return ""
	}
	return device.TenantId
}

func (tmq *MessageApp) DeviceStatusToMessageBus(ctx context.Context, deviceId, deviceStatus string) {
	var messageBus dtos.MessageBus
	messageBus.DeviceId = deviceId
	messageBus.TenantId = tmq.tenantIDFor(ctx, deviceId)
	messageBus.MessageType = "DEVICE_STATUS"
	messageBus.Data = map[string]interface{}{
		"status": deviceStatus,
		"time":   time.Now().UnixMilli(),
	}
	b, _ := json.Marshal(messageBus)
	tmq.pushMsgToMessageBus(b)

}
func (tmq *MessageApp) ThingModelMsgReport(ctx context.Context, msg dtos.ThingModelMessage) (*drivercommon.CommonResponse, error) {
	msg.TenantId = tmq.tenantIDFor(ctx, msg.Cid)
	tmq.pushMsgToMessageBus(msg.TransformMessageBus())
	persistItf := coreContainer.PersistItfFrom(tmq.dic.Get)
	err := persistItf.SaveDeviceThingModelData(ctx, msg)
	if err != nil {
		tmq.lc.Error("saveDeviceThingModelData error:", err.Error())
	}
	response := new(drivercommon.CommonResponse)
	if err != nil {
		response.Success = false
		response.Code = strconv.Itoa(errort.KindDatabaseError)
		response.ErrorMessage = err.Error()
	} else {
		response.Code = "0"
		response.Success = true
	}
	return response, nil
}
