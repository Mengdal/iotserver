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

func IsValidDataFormat(value string) bool {
	return value == string(Standard)
}

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

type ProductNodeType string

const (
	ProductNodeTypeUnKnow    ProductNodeType = "其他"
	ProductNodeTypeGateway   ProductNodeType = "网关"
	ProductNodeTypeSubDevice ProductNodeType = "网关子设备"
	ProductNodeTypeDevice    ProductNodeType = "直连设备"
)

type PlanformType string

const (
	PlanformLocal PlanformType = "本地"
	PlanformCloud PlanformType = "云平台"
)

type Protocol string

const (
	MQTT Protocol = "MQTT"
)
