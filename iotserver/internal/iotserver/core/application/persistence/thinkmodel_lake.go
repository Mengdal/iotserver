package persistence

import (
	"context"
	"encoding/json"
	"time"

	"github.com/LMgateways/iotserver-driver-proto/thingmodel"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/application/messagestore"
	resourceContainer "iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/tenant"
)

// 本文件是时序数据层的编排逻辑：写入按「设备 -> 行」展开，查询复用统一的物模型/产品
// 元数据校验，具体 SQL 全部在 internal/tools/datadb/duckdb 内。
//
// 数据层使用 DuckDB 单文件列存，
// 因此这里没有按后端分发的分支。

const (
	// defaultDeviceTenantTTL 设备->租户 缓存有效期（兜底设备改属租户后的滞后）
	defaultDeviceTenantTTL = 10 * time.Minute
	// defaultDeviceTenantCacheMax 缓存容量上限
	defaultDeviceTenantCacheMax = 100_000
)

// resolveTenant 由设备归属推导租户。
//
// 这里用**平台作用域**查设备是刻意的：本函数在「推导这份数据属于谁」，不是在做授权，
// 而上行消息（MQTT/网关）本身不带租户上下文，若用请求租户去过滤会查不到设备。
// 授权发生在读路径 —— 那里用请求自身的 ctx 查设备，跨租户会直接查不到。
func (pst *persistApp) resolveTenant(ctx context.Context, deviceID string) string {
	if tid, ok := pst.tenantCache.get(deviceID); ok {
		return tid
	}
	if pst.dbClient == nil {
		// 精简装配（单测）可能没有元数据库客户端，退回上下文里的租户
		return tenant.FromContext(ctx).TenantID
	}
	device, err := pst.dbClient.WithCtx(tenant.Platform(ctx)).DeviceById(deviceID)
	if err != nil {
		pst.lc.Warnf("persistence: 解析设备 %s 的租户失败，本次写入 tenant_id 为空: %v", deviceID, err)
		return ""
	}
	pst.tenantCache.set(deviceID, device.TenantId)
	return device.TenantId
}

// saveDeviceThingModelData 把上报消息写入时序库
func (pst *persistApp) saveDeviceThingModelData(ctx context.Context, req dtos.ThingModelMessage) error {
	if ctx == nil {
		ctx = context.Background()
	}
	// 上行消息不带租户：用设备归属补上，让时序行的 tenant_id 落到真实归属
	if tid := pst.resolveTenant(ctx, req.Cid); tid != "" {
		ctx = tenant.With(ctx, tenant.Context{TenantID: tid})
	}

	switch req.GetOpType() {
	case thingmodel.OperationType_PROPERTY_REPORT:
		propertyMsg, err := req.TransformMessageDataByProperty()
		if err != nil {
			return err
		}
		for code, reportData := range propertyMsg.Data {
			data := map[string]interface{}{code: reportData}
			if err = pst.dataDbClient.Insert(ctx, req.Cid, data); err != nil {
				return err
			}
		}

	case thingmodel.OperationType_EVENT_REPORT:
		eventMsg, err := req.TransformMessageDataByEvent()
		if err != nil {
			return err
		}
		payload, _ := json.Marshal(eventMsg.Data)
		data := map[string]interface{}{eventMsg.Data.EventCode: string(payload)}
		return pst.dataDbClient.Insert(ctx, "event:"+req.Cid, data)

	case thingmodel.OperationType_SERVICE_EXECUTE:
		serviceMsg, err := req.TransformMessageDataByService()
		if err != nil {
			return err
		}
		payload, _ := serviceMsg.Marshal()
		data := map[string]interface{}{serviceMsg.Code: string(payload)}
		return pst.dataDbClient.Insert(ctx, "service:"+req.Cid, data)

	case thingmodel.OperationType_DATA_BATCH_REPORT:
		// 网关批量上报 / 断点续传补传：每个测点带自己的时间戳，落库到原始采集时刻
		msg, err := req.TransformMessageDataByBatchReport()
		if err != nil {
			return err
		}
		if len(msg.Data.Properties) > 0 {
			data := make(map[string]interface{}, len(msg.Data.Properties))
			for code, property := range msg.Data.Properties {
				ts := property.Time
				if ts == 0 {
					ts = msg.Time
				}
				data[code] = dtos.ReportData{Value: property.Value, Time: ts}
			}
			if err = pst.dataDbClient.Insert(ctx, req.Cid, data); err != nil {
				return err
			}
		}
		for code, event := range msg.Data.Events {
			ts := event.EventTime
			if ts == 0 {
				ts = msg.Time
			}
			body, _ := json.Marshal(map[string]interface{}{
				"eventCode":    code,
				"eventTime":    ts,
				"outputParams": event.OutputParams,
			})
			payload := map[string]interface{}{code: string(body)}
			if err = pst.dataDbClient.Insert(ctx, "event:"+req.Cid, payload); err != nil {
				return err
			}
		}

	case thingmodel.OperationType_PROPERTY_GET_RESPONSE:
		msg, err := req.TransformMessageDataByGetProperty()
		if err != nil {
			return err
		}
		messageStore := resourceContainer.MessageStoreItfFrom(pst.dic.Get)
		ack, ok := messageStore.LoadMsgChan(msg.MsgId)
		if !ok {
			return nil
		}
		if v, ok := ack.(*messagestore.MsgAckChan); ok {
			v.TrySendDataAndCloseChan(msg.Data)
			messageStore.DeleteMsgId(msg.MsgId)
		}
	}
	return nil
}

// searchDeviceHistoryPropertyData 查询属性历史。
// 用传入的 ctx 查设备：跨租户访问会在这一层直接查不到设备。
func (pst *persistApp) searchDeviceHistoryPropertyData(ctx context.Context, req dtos.ThingModelPropertyDataRequest) (interface{}, int, error) {
	deviceInfo, err := pst.dbClient.WithCtx(ctx).DeviceById(req.DeviceId)
	if err != nil {
		return nil, 0, err
	}
	defs, err := pst.devicePropertyDefs(ctx, deviceInfo)
	if err != nil {
		return nil, 0, err
	}
	var response []dtos.ReportData
	var count int
	for _, def := range defs {
		if def.Code != req.Code {
			continue
		}
		response, count, err = pst.dataDbClient.GetDeviceProperty(req, deviceInfo)
		if err != nil {
			pst.lc.Errorf("GetDeviceProperty error %+v", err)
		}
		break
	}
	return response, count, nil
}

// searchDeviceLatestPropertyData 查询属性最新值
func (pst *persistApp) searchDeviceLatestPropertyData(ctx context.Context, req dtos.ThingModelPropertyDataRequest) (interface{}, error) {
	deviceInfo, err := pst.dbClient.WithCtx(ctx).DeviceById(req.DeviceId)
	if err != nil {
		return nil, err
	}
	defs, err := pst.devicePropertyDefs(ctx, deviceInfo)
	if err != nil {
		return nil, err
	}
	response := make([]dtos.ThingModelDataResponse, 0, len(defs))
	for _, def := range defs {
		if req.Code != "" && def.Code != req.Code {
			continue
		}
		latestReq := req
		latestReq.Code = def.Code
		latestReq.Last = true
		ksv, _, err := pst.dataDbClient.GetDeviceProperty(latestReq, deviceInfo)
		if err != nil {
			pst.lc.Errorf("GetDeviceProperty error %+v", err)
			continue
		}
		var reportData dtos.ReportData
		if len(ksv) > 0 {
			reportData = ksv[0]
		}
		response = append(response, dtos.ThingModelDataResponse{
			ReportData: reportData,
			Code:       def.Code,
			DataType:   def.DataType,
			Name:       def.Name,
			Unit:       def.Unit,
			AccessMode: def.AccessMode,
		})
	}
	return response, nil
}

// propertyDef 读取侧的测点定义（把「产品物模型」与「设备级测点」两种来源归一）
type propertyDef struct {
	Code       string
	Name       string
	DataType   string
	Unit       string
	AccessMode string
}

// devicePropertyDefs 取设备的测点定义：**产品物模型优先，无产品/无物模型时回落到设备级测点表**。
//
// 存在的必要性：随网关清单自动接入的子设备可以没有产品（参考系统的上行协议里
// 根本没有产品维度），而原来的读路径无条件 `ProductById(device.ProductId)` 并遍历
// 其物模型——空产品会让整个查询直接报错，界面上一个测点都显示不出来。
// 见 docs/device-access-design.md 第 1.5 节。
func (pst *persistApp) devicePropertyDefs(ctx context.Context, device models.Device) ([]propertyDef, error) {
	if device.ProductId != "" {
		productInfo, err := pst.dbClient.WithCtx(ctx).ProductById(device.ProductId)
		// 产品查不到不算错误：可能是没有产品，也可能产品已被删除。
		// 此时回落到设备级测点表，而不是让整个查询失败。
		if err == nil && len(productInfo.Properties) > 0 {
			out := make([]propertyDef, 0, len(productInfo.Properties))
			for _, p := range productInfo.Properties {
				out = append(out, propertyDef{
					Code:       p.Code,
					Name:       p.Name,
					DataType:   string(p.TypeSpec.Type),
					Unit:       propertyUnit(p),
					AccessMode: p.AccessMode,
				})
			}
			return out, nil
		}
	}
	tags, err := pst.dbClient.WithCtx(ctx).ListDeviceTags(device.Id)
	if err != nil {
		return nil, err
	}
	out := make([]propertyDef, 0, len(tags))
	for _, t := range tags {
		out = append(out, propertyDef{
			Code:       t.Code,
			Name:       t.Name,
			DataType:   tagDataType(t.Type),
			Unit:       t.Unit,
			AccessMode: t.AccessMode,
		})
	}
	return out, nil
}

// tagDataType 把设备级测点的类型映射到物模型的数据类型取值。
// 自动发现时未知类型回落 text——不猜数值，猜错会让前端按数值格式化字符串。
func tagDataType(t string) string {
	switch t {
	case string(constants.SpecsTypeInt):
		return string(constants.SpecsTypeInt)
	case string(constants.SpecsTypeFloat):
		return string(constants.SpecsTypeFloat)
	case string(constants.SpecsTypeBool):
		return string(constants.SpecsTypeBool)
	default:
		return string(constants.SpecsTypeText)
	}
}

// searchDeviceEventData 查询事件历史
func (pst *persistApp) searchDeviceEventData(ctx context.Context, req dtos.ThingModelEventDataRequest) ([]dtos.ThingModelEventDataResponse, int, error) {
	var response dtos.ThingModelEventDataResponseArray
	deviceInfo, err := pst.dbClient.WithCtx(ctx).DeviceById(req.DeviceId)
	if err != nil {
		return response, 0, err
	}
	productInfo, err := pst.dbClient.WithCtx(ctx).ProductById(deviceInfo.ProductId)
	if err != nil {
		return response, 0, err
	}
	data, count, err := pst.dataDbClient.GetDeviceEvent(req, deviceInfo, productInfo)
	if err != nil {
		return response, count, err
	}
	for _, item := range data {
		eventType, name := getEventTypeAndName(productInfo.Events, item.EventCode)
		response = append(response, dtos.ThingModelEventDataResponse{
			EventCode:  item.EventCode,
			EventType:  eventType,
			OutputData: item.OutputParams,
			ReportTime: item.EventTime,
			Name:       name,
		})
	}
	return response, count, nil
}

// searchDeviceServiceData 查询服务调用历史
func (pst *persistApp) searchDeviceServiceData(ctx context.Context, req dtos.ThingModelServiceDataRequest) ([]dtos.ThingModelServiceDataResponse, int, error) {
	var count int
	deviceInfo, err := pst.dbClient.WithCtx(ctx).DeviceById(req.DeviceId)
	if err != nil {
		return nil, count, err
	}
	productInfo, err := pst.dbClient.WithCtx(ctx).ProductById(deviceInfo.ProductId)
	if err != nil {
		return nil, count, err
	}
	var response dtos.ThingModelServiceDataResponseArray
	if req.Code == "" {
		for _, action := range productInfo.Actions {
			req.Code = action.Code
			var ksv []dtos.SaveServiceIssueData
			ksv, count, err = pst.dataDbClient.GetDeviceService(req, deviceInfo, productInfo)
			if err != nil {
				continue
			}
			for _, data := range ksv {
				response = append(response, dtos.ThingModelServiceDataResponse{
					ServiceName: action.Name,
					Code:        data.Code,
					InputData:   data.InputParams,
					OutputData:  data.OutputParams,
					ReportTime:  data.Time,
				})
			}
		}
	} else {
		var ksv []dtos.SaveServiceIssueData
		ksv, count, err = pst.dataDbClient.GetDeviceService(req, deviceInfo, productInfo)
		if err != nil {
			return nil, count, err
		}
		for _, data := range ksv {
			name := getServiceName(productInfo.Actions, data.Code)
			response = append(response, dtos.ThingModelServiceDataResponse{
				ServiceName: name,
				Code:        data.Code,
				InputData:   data.InputParams,
				OutputData:  data.OutputParams,
				ReportTime:  data.Time,
			})
		}
	}
	return response, count, nil
}

// searchDeviceMsgCount 统计消息总数。
//
// 优先走数据层的「一条 SQL」快路径（MsgCounter 可选能力），避免按设备 N+1；
// 未实现该能力的形态退回按设备汇总——那条路径的设备列表本身已由仓储层
// 按租户过滤，所以租户语义仍然正确。
//
// 租户作用域由 ctx 决定：平台作用域统计全平台，租户作用域只统计该租户；
// 完全没有租户上下文时交给仓储层策略决定（Enforce 下会得到空集）。
func (pst *persistApp) searchDeviceMsgCount(ctx context.Context, startTime, endTime int64) (int, error) {
	tc := tenant.FromContext(ctx)
	if counter, ok := pst.dataDbClient.(interfaces.MsgCounter); ok {
		switch {
		case tc.Platform:
			return counter.CountAllMsgByTimeRange("", startTime, endTime)
		case tc.TenantID != "":
			return counter.CountAllMsgByTimeRange(tc.TenantID, startTime, endTime)
		}
		// 无租户上下文：不走快路径，交给下面的仓储层策略处理
	}

	devices, _, err := pst.dbClient.WithCtx(ctx).DevicesSearch(0, -1, dtos.DeviceSearchQueryRequest{})
	if err != nil {
		return 0, err
	}
	count := 0
	for _, device := range devices {
		n, err := pst.dataDbClient.GetDeviceMsgCountByGiveTime(device.Id, startTime, endTime)
		if err != nil {
			return count, err
		}
		count += n
	}
	return count, nil
}

// propertyUnit 从物模型类型定义里取单位（数值型取 unit，枚举型取匹配到的描述）
func propertyUnit(property models.Properties) string {
	switch property.TypeSpec.Type {
	case constants.SpecsTypeInt, constants.SpecsTypeFloat:
		var spec models.TypeSpecIntOrFloat
		_ = json.Unmarshal([]byte(property.TypeSpec.Specs), &spec)
		return spec.Unit
	default:
		return ""
	}
}
