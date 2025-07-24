package constants

type ThingModelType string

const (
	ModelTypeProperty ThingModelType = "property"
	ModelTypeEvent    ThingModelType = "event"
	ModelTypeAction   ThingModelType = "action"
)

type CallType string

const (
	CallTypeSync  CallType = "SYNC"  //同步
	CallTypeAsync CallType = "ASYNC" //异步
)

type AccessModel string

const (
	AccessR  AccessModel = "R"
	AccessRW AccessModel = "RW"
)

type TagType string

const (
	Customize TagType = "自定义"
	System    TagType = "系统"
)

type DataFormat string

const (
	Standard DataFormat = "标准物模型" //同步
)

type EventType string

const (
	EventTypeInfo  EventType = "info"
	EventTypeAlert EventType = "alert"
	EventTypeError EventType = "error"
)

type ProductStatus int

const (
	ProductRelease   ProductStatus = 1
	ProductUnRelease ProductStatus = 0
)
