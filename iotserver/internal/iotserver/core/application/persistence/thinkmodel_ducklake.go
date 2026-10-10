package persistence

import (
	"context"
	"encoding/json"

	"github.com/winc-link/edge-driver-proto/thingmodel"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/application/messagestore"
	resourceContainer "iotserver/internal/iotserver/core/container"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
)

// 本文件是 DuckLake 数据层的编排逻辑：
// 写入按「设备 -> 行」展开，查询复用与其它后端一致的物模型/产品元数据校验，
// 具体 SQL 全部在 internal/tools/datadb/ducklake 内。

// saveDeviceThingModelToDuckLake 把上报消息写入 DuckLake 湖仓
func (pst *persistApp) saveDeviceThingModelToDuckLake(req dtos.ThingModelMessage) error {
	switch req.GetOpType() {
	case thingmodel.OperationType_PROPERTY_REPORT:
		propertyMsg, err := req.TransformMessageDataByProperty()
		if err != nil {
			return err
		}
		for code, reportData := range propertyMsg.Data {
			data := map[string]interface{}{code: reportData}
			if err = pst.dataDbClient.Insert(context.Background(), req.Cid, data); err != nil {
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
		return pst.dataDbClient.Insert(context.Background(), "event:"+req.Cid, data)

	case thingmodel.OperationType_SERVICE_EXECUTE:
		serviceMsg, err := req.TransformMessageDataByService()
		if err != nil {
			return err
		}
		payload, _ := serviceMsg.Marshal()
		data := map[string]interface{}{serviceMsg.Code: string(payload)}
		return pst.dataDbClient.Insert(context.Background(), "service:"+req.Cid, data)

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

// searchDeviceThingModelHistoryPropertyDataFromDuckLake 查询属性历史
func (pst *persistApp) searchDeviceThingModelHistoryPropertyDataFromDuckLake(req dtos.ThingModelPropertyDataRequest) (interface{}, int, error) {
	deviceInfo, err := pst.dbClient.DeviceById(req.DeviceId)
	if err != nil {
		return nil, 0, err
	}
	productInfo, err := pst.dbClient.ProductById(deviceInfo.ProductId)
	if err != nil {
		return nil, 0, err
	}
	var response []dtos.ReportData
	var count int
	for _, property := range productInfo.Properties {
		if property.Code != req.Code {
			continue
		}
		response, count, err = pst.dataDbClient.GetDeviceProperty(req, deviceInfo)
		if err != nil {
			pst.lc.Errorf("GetDeviceProperty(ducklake) error %+v", err)
		}
		break
	}
	return response, count, nil
}

// searchDeviceThingModelPropertyDataFromDuckLake 查询属性最新值
func (pst *persistApp) searchDeviceThingModelPropertyDataFromDuckLake(req dtos.ThingModelPropertyDataRequest) (interface{}, error) {
	deviceInfo, err := pst.dbClient.DeviceById(req.DeviceId)
	if err != nil {
		return nil, err
	}
	productInfo, err := pst.dbClient.ProductById(deviceInfo.ProductId)
	if err != nil {
		return nil, err
	}
	response := make([]dtos.ThingModelDataResponse, 0)
	for _, property := range productInfo.Properties {
		if req.Code != "" && property.Code != req.Code {
			continue
		}
		code := property.Code
		latestReq := req
		latestReq.Code = code
		latestReq.Last = true
		ksv, _, err := pst.dataDbClient.GetDeviceProperty(latestReq, deviceInfo)
		if err != nil {
			pst.lc.Errorf("GetDeviceProperty(ducklake) error %+v", err)
			continue
		}
		var reportData dtos.ReportData
		if len(ksv) > 0 {
			reportData = ksv[0]
		}
		response = append(response, dtos.ThingModelDataResponse{
			ReportData: reportData,
			Code:       code,
			DataType:   string(property.TypeSpec.Type),
			Name:       property.Name,
			Unit:       propertyUnit(property),
			AccessMode: property.AccessMode,
		})
	}
	return response, nil
}

// searchDeviceThingModelEventDataFromDuckLake 查询事件历史
func (pst *persistApp) searchDeviceThingModelEventDataFromDuckLake(req dtos.ThingModelEventDataRequest) (dtos.ThingModelEventDataResponseArray, int, error) {
	var response dtos.ThingModelEventDataResponseArray
	deviceInfo, err := pst.dbClient.DeviceById(req.DeviceId)
	if err != nil {
		return response, 0, err
	}
	productInfo, err := pst.dbClient.ProductById(deviceInfo.ProductId)
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

// searchDeviceThingModelServiceDataFromDuckLake 查询服务调用历史
// （与 leveldb 的实现同为后端无关的编排逻辑，底层走 dataDbClient）
func (pst *persistApp) searchDeviceThingModelServiceDataFromDuckLake(req dtos.ThingModelServiceDataRequest) ([]dtos.ThingModelServiceDataResponse, int, error) {
	return pst.searchDeviceThingModelServiceDataFromLevelDB(req)
}

// searchDeviceMsgCountFromDuckLake 统计消息总数
func (pst *persistApp) searchDeviceMsgCountFromDuckLake(startTime, endTime int64) (int, error) {
	devices, _, err := pst.dbClient.DevicesSearch(0, -1, dtos.DeviceSearchQueryRequest{})
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
