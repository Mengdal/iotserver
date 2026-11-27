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

package dtos

import (
	"encoding/json"
	"github.com/LMgateways/iotserver-driver-proto/drivercommon"
	"github.com/LMgateways/iotserver-driver-proto/thingmodel"
)

type ThingModelMessage struct {
	BaseRequest *drivercommon.BaseRequestMessage
	Cid         string `json:"cid"`     // 下发的目标设备id
	OpType      int32  `json:"op_type"` // 消息类型
	Data        string `json:"data"`    // 云端下发消息内容
	// TenantId 消息所属租户。消息总线是所有租户共享的 topic，
	// 规则引擎需要按它做强制过滤，避免跨租户读数据。
	TenantId string `json:"tenantId,omitempty"`
}

func (m *ThingModelMessage) String() string {
	b, _ := json.Marshal(m)
	return string(b)
}

type MessageBus struct {
	DeviceId    string `json:"deviceId"`
	DeviceName  string `json:"deviceName,omitempty"`
	ProductId   string `json:"productId,omitempty"`
	ProductName string `json:"productName,omitempty"`
	TenantId    string `json:"tenantId,omitempty"`
	MessageType string `json:"messageType"`
	// Time 消息时间（毫秒）：属性上报取所有测点时间的最大值，事件取 eventTime，
	// 设备状态取状态时间，批报取批次时间。告警/规则直接用它作为上报时间。
	Time int64       `json:"time,omitempty"`
	Data interface{} `json:"data"`
}

func (m *ThingModelMessage) TransformMessageBus() []byte {
	var messageBus MessageBus
	messageBus.DeviceId = m.Cid
	messageBus.TenantId = m.TenantId
	messageBus.MessageType = thingmodel.OperationType_name[m.OpType]
	data := make(map[string]interface{})
	err := json.Unmarshal([]byte(m.Data), &data)
	if err != nil {
		return nil
	}
	// 驱动/网关的属性报文是 data.<code> = {value,time}；iotp 采集链路是扁平
	// data.<code> = value。消息总线必须只有一种形状，规则 SQL 才能同时覆盖两种来源。
	switch m.GetOpType() {
	case thingmodel.OperationType_PROPERTY_REPORT:
		messageBus.Data, messageBus.Time = flattenPropertyReport(data["data"])
	case thingmodel.OperationType_EVENT_REPORT:
		messageBus.Data = data["data"]
		messageBus.Time = nestedMillis(data["data"], "eventTime")
	case thingmodel.OperationType_DATA_BATCH_REPORT:
		// 批报的属性/事件仍在 data 里，消息时间取批次时间。
		messageBus.Data = data["data"]
		messageBus.Time = millis(data["time"])
	default:
		messageBus.Data = data["data"]
	}
	b, _ := json.Marshal(messageBus)
	return b
}

// millis 把 JSON 数字转成毫秒时间戳；缺失或非法时返回 0。
func millis(v interface{}) int64 {
	if f, ok := v.(float64); ok && f > 0 {
		return int64(f)
	}
	return 0
}

// nestedMillis 读取对象字段里的毫秒时间。
func nestedMillis(raw interface{}, field string) int64 {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return 0
	}
	return millis(m[field])
}

// flattenPropertyReport 把 {code:{value,time}} 压平成 {code:value}。
// 已是标量的字段原样保留；没有任何测点时间时返回 0。
func flattenPropertyReport(raw interface{}) (map[string]interface{}, int64) {
	report, ok := raw.(map[string]interface{})
	if !ok {
		return map[string]interface{}{}, 0
	}
	var reportTime int64
	// 原地压平：report 是本次 json.Unmarshal 新建的 map，不会被调用方复用。
	for code, value := range report {
		item, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		fieldValue, ok := item["value"]
		if !ok {
			continue
		}
		report[code] = fieldValue
		if ts, ok := item["time"].(float64); ok && int64(ts) > reportTime {
			reportTime = int64(ts)
		}
	}
	return report, reportTime
}

func (m *ThingModelMessage) GetOpType() thingmodel.OperationType {
	return thingmodel.OperationType(m.OpType)
}

func (m *ThingModelMessage) IsPersistent() bool {
	var isPersistent bool
	switch m.GetOpType() {
	case thingmodel.OperationType_PROPERTY_REPORT, thingmodel.OperationType_EVENT_REPORT:
		isPersistent = true
	default:
		return isPersistent
	}
	return isPersistent
}

func (m *ThingModelMessage) TransformMessageDataBySetProperty() (DevicePropertySetResponse, error) {
	var dataMsg DevicePropertySetResponse
	err := json.Unmarshal([]byte(m.Data), &dataMsg)
	return dataMsg, err
}

func (m *ThingModelMessage) TransformMessageDataByGetProperty() (DeviceGetPropertyResponse, error) {
	var dataMsg DeviceGetPropertyResponse
	err := json.Unmarshal([]byte(m.Data), &dataMsg)
	return dataMsg, err
}

func (m *ThingModelMessage) TransformMessageDataByProperty() (DevicePropertyReport, error) {
	var dataMsg DevicePropertyReport
	err := json.Unmarshal([]byte(m.Data), &dataMsg)
	return dataMsg, err
}

func (m *ThingModelMessage) TransformMessageDataByEvent() (EdgeXDeviceEventReport, error) {
	var dataMsg EdgeXDeviceEventReport
	err := json.Unmarshal([]byte(m.Data), &dataMsg)
	return dataMsg, err
}

func (m *ThingModelMessage) TransformMessageDataByService() (SaveServiceIssueData, error) {
	var dataMsg SaveServiceIssueData
	err := json.Unmarshal([]byte(m.Data), &dataMsg)
	return dataMsg, err
}

func (m *ThingModelMessage) TransformMessageDataByServiceExec() (ServiceExecResponse, error) {
	var dataMsg ServiceExecResponse
	err := json.Unmarshal([]byte(m.Data), &dataMsg)
	return dataMsg, err
}

func (m *ThingModelMessage) TransformMessageDataByBatchReport() (DeviceBatchReport, error) {
	var dataMsg DeviceBatchReport
	err := json.Unmarshal([]byte(m.Data), &dataMsg)
	return dataMsg, err
}

func ThingModelMessageFromThingModelMsg(msg *thingmodel.ThingModelMsg) ThingModelMessage {
	return ThingModelMessage{
		BaseRequest: msg.BaseRequest,
		Cid:         msg.DeviceId,
		OpType:      int32(msg.OperationType),
		Data:        msg.Data,
	}
}

type DevicePropertyReport struct {
	MsgId   string `json:"msgId"`
	Version string `json:"version"`
	//Time    int64  `json:"time"`
	Sys struct {
		Ack int `json:"ack"`
	} `json:"sys"`
	Data map[string]ReportData `json:"data"`
}

type DeviceGetPropertyResponse struct {
	MsgId string                  `json:"msgId"`
	Data  []EffectivePropertyData `json:"data"`
}

type DevicePropertySetResponse struct {
	MsgId string                `json:"msgId"`
	Data  DevicePropertySetData `json:"data"`
}

type DevicePropertySetData struct {
	ErrorMessage string `json:"errorMessage"`
	Code         uint32 `json:"code"`
	Success      bool   `json:"success"`
}

type DeviceBatchReport struct {
	MsgId   string `json:"msgId"`
	Version string `json:"version"`
	Time    int64  `json:"time"`
	Sys     struct {
		Ack int `json:"ack"`
	} `json:"sys"`
	Data BatchData `json:"data"`
}

type BatchData struct {
	Properties map[string]BatchProperty `json:"properties"`
	Events     map[string]BatchEvent    `json:"events"`
}

type BatchProperty struct {
	Value interface{} `json:"value"`
	// Time 测点采集时间（毫秒）。断点续传补历史曲线时按测点带时间；
	// 为 0 时回落到批次的 time。
	Time int64 `json:"time"`
}
type BatchEvent struct {
	// EventTime 事件发生时间（毫秒），语义同 BatchProperty.Time
	EventTime    int64                  `json:"eventTime"`
	OutputParams map[string]interface{} `json:"outputParams"`
}

type ReportData struct {
	Value interface{} `json:"value"`
	Time  int64       `json:"time"`
}

func (r *ReportData) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type EdgeXDeviceEventReport struct {
	MsgId   string `json:"msgId"`
	Version string `json:"version"`
	Sys     struct {
		Ack int `json:"ack"`
	} `json:"sys"`
	Data EventData `json:"data"`
}

type EventData struct {
	EventCode    string                 `json:"eventCode"`
	EventTime    int64                  `json:"eventTime"`
	OutputParams map[string]interface{} `json:"outputParams"`
}

func (r *EventData) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

// operationType:SERVICE_EXECUTE data:"topic:\"/sys/hq85KDcqGI6/test_008/thing/service/DeleteAlgorithmModel\"
// message:\"{\\\"id\\\":\\\"2130175210\\\",\\\"version\\\":\\\"1.0.0\\\",\\\"code\\\":\\\"DeleteAlgorithmModel\\\",\\\"params\\\":{\\\"ForceDelete\\\":3}}\""
type InvokeDeviceService struct {
	MsgId   string      `json:"msgId"`
	Version string      `json:"version"`
	Time    int64       `json:"time"`
	Data    ServiceData `json:"data"`
}

type ServiceData struct {
	Code        string                 `json:"code"`
	InputParams map[string]interface{} `json:"inputParams"`
}

func (r *ServiceData) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func (r *InvokeDeviceService) ToString() string {
	s, _ := json.Marshal(r)
	return string(s)
}

type DeviceGetPropertyData struct {
	MsgId   string   `json:"msgId"`
	Version string   `json:"version"`
	Time    int64    `json:"time"`
	Data    []string `json:"data"`
}

func (r *DeviceGetPropertyData) ToString() string {
	s, _ := json.Marshal(r)
	return string(s)
}

type SaveServiceIssueData struct {
	MsgId        string                 `json:"msgId"`
	Code         string                 `json:"code"`
	Time         int64                  `json:"time"`
	InputParams  map[string]interface{} `json:"inputParams"`
	OutputParams map[string]interface{} `json:"outputParams"`
}

func (r *SaveServiceIssueData) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type ServiceExecResponse struct {
	MsgId        string                 `json:"msgId"`
	Code         string                 `json:"code"`
	Time         int64                  `json:"time"`
	OutputParams map[string]interface{} `json:"data"`
}

func (r *ServiceExecResponse) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type PropertySet struct {
	Version string                 `json:"version"`
	MsgId   string                 `json:"msgId"`
	Time    int64                  `json:"time"`
	Params  map[string]interface{} `json:"data"`
}

func (r *PropertySet) ToString() string {
	s, _ := json.Marshal(r)
	return string(s)
}
