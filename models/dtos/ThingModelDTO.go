package dtos

import (
	"iotServer/models/constants"
)

type ThingModelDTO struct {
	Id             int64                    `json:"id" description:"物模型ID"`
	ProductId      int64                    `json:"product_id" example:"1939557978394857472" description:"产品ID"`
	ThingModelType constants.ThingModelType `json:"thing_model_type" example:"property" description:"物模型类型：property/event/action"`
	Name           string                   `json:"name" example:"温度" description:"名称"`
	Code           string                   `json:"code" example:"temperature" description:"编码"`
	Description    string                   `json:"description" example:"设备当前环境温度" description:"描述"`
	Tag            constants.TagType        `json:"tag" example:"自定义" description:"标签"`
	Property       *ThingModelProperties    `json:"property"`
	Event          *ThingModelEvents        `json:"event"`
	Action         *ThingModelActions       `json:"action"`
	Type           string                   `json:"type"`
}

type ThingModelProperties struct {
	AccessModel constants.AccessModel `json:"access_model" example:"R" description:"访问模式：R/RW"`
	Require     bool                  `json:"require" example:"true" description:"是否必填"`
	DataType    constants.SpecsType   `json:"type" example:"int" description:"数据类型：int/float/string/bool/double/enum..."`
	TypeSpec    interface{}           `json:"specs" description:"{\"min\":\"-40\",\"max\":\"120\",\"step\":\"0.01\",\"unit\":\"℃\",\"unitName\":\"摄氏度\",\"valueType\":\"(S:瞬时量，L:累积量，K:开关量, C:产量，YL:用量)\"}" description:"字段规格(JSON字符串)"`
}

type ThingModelEvents struct {
	EventType   constants.EventType     `json:"event_type" example:"string" description:"事件类型"`
	OutPutParam []ThingModelEventAction `json:"output_param"`
}

type ThingModelActions struct {
	CallType    constants.CallType      `json:"call_type" example:"SYNC" description:"调用类型：SYNC/ASYNC"`
	InPutParam  []ThingModelEventAction `json:"input_param"`
	OutPutParam []ThingModelEventAction `json:"output_param"`
}

type ThingModelEventAction struct {
	Code     string              `json:"code" example:"offline_event" description:"参数编码"`
	Name     string              `json:"name" example:"设备掉线告警事件" description:"参数名称"`
	DataType constants.SpecsType `json:"type" example:"int" description:"数据类型"`
	TypeSpec interface{}         `json:"specs" description:"规格对象" example:"[{\"code\":\"204751\",\"name\":\"user\",\"type_spec\":{\"type\":\"int\",\"specs\":\"{\"max\":\"255\",\"min\":\"0\",\"step\":\"1\"}\"}}]" `
}

type ThingModelTemplateResponse struct {
	Id             string      `json:"id"`
	CategoryName   string      `json:"category_name"` // 品类名称
	CategoryKey    string      `json:"category_key"`
	ThingModelJSON string      `json:"thing_model_json"`
	Properties     interface{} `json:"properties"`
	Events         interface{} `json:"events"`
	Actions        interface{} `json:"actions"`
}
