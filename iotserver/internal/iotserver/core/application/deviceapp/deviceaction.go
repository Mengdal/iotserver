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

package deviceapp

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/distribution/uuid"
	"github.com/LMgateways/iotserver-driver-proto/thingmodel"
	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/container"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/rpcclient"
	"time"
)

// ui控制台、定时任务、场景联动、云平台api 都可以调用
func (p *deviceApp) DeviceAction(jobAction dtos.JobAction) dtos.DeviceExecRes {
	defer func() {
		if err := recover(); err != nil {
			p.lc.Error("Panic:", err)
		}
	}()

	// 平台作用域：系统路径，不按租户过滤
	device, err := p.dbClient.WithCtx(tenant.Platform(context.Background())).DeviceById(jobAction.DeviceId)
	if err != nil {
		return dtos.DeviceExecRes{
			Result:  false,
			Message: "device not found",
		}
	}
	deviceService, err := p.dbClient.WithCtx(tenant.Platform(context.Background())).DeviceServiceById(device.DriveInstanceId)
	if err != nil {
		return dtos.DeviceExecRes{
			Result:  false,
			Message: "driver not found",
		}
	}

	driverService := container.DriverServiceAppFrom(di.GContainer.Get)
	status := driverService.GetState(deviceService.Id)
	if status == constants.RunStatusStarted {
		client, errX := rpcclient.NewDriverRpcClient(deviceService.BaseAddress, false, "", deviceService.Id, p.lc)
		if errX != nil {
			return dtos.DeviceExecRes{
				Result:  false,
				Message: errX.Error(),
			}
		}
		defer client.Close()
		var rpcRequest thingmodel.ThingModelIssueMsg
		rpcRequest.DeviceId = jobAction.DeviceId
		rpcRequest.OperationType = thingmodel.OperationType_PROPERTY_SET
		var data dtos.PropertySet
		data.Version = "v1.0"
		data.MsgId = uuid.Generate().String()
		data.Time = time.Now().UnixMilli()
		param := make(map[string]interface{})
		param[jobAction.Code] = jobAction.Value
		data.Params = param
		rpcRequest.Data = data.ToString()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		_, err = client.ThingModelDownServiceClient.ThingModelMsgIssue(ctx, &rpcRequest)

		if err == nil {
			messageStore := container.MessageStoreItfFrom(p.dic.Get)
			ch := messageStore.GenAckChan(data.MsgId)

			select {
			case <-time.After(10 * time.Second):
				ch.TryCloseChan()
				return dtos.DeviceExecRes{
					Result:  false,
					Message: "wait response timeout",
				}
			case <-ctx.Done():
				return dtos.DeviceExecRes{
					Result:  false,
					Message: "wait response timeout",
				}
			case resp := <-ch.DataChan:
				if v, ok := resp.(dtos.DevicePropertySetData); ok {
					message, _ := json.Marshal(v)
					return dtos.DeviceExecRes{
						Result:  true,
						Message: string(message),
					}
				}
			}
		} else {
			return dtos.DeviceExecRes{
				Result:  false,
				Message: err.Error(),
			}
		}
	}
	return dtos.DeviceExecRes{
		Result:  false,
		Message: "driver status stop",
	}
}

func (p *deviceApp) SetDeviceProperty(req dtos.OpenApiSetDeviceThingModel) (err error) {
	// 注意：必须用命名返回值，否则 panic 被 recover 后函数返回 nil，
	// 调用方（场景联动）会把它记成「执行成功」，造成审计与实际不符
	defer func() {
		if r := recover(); r != nil {
			p.lc.Error("Panic:", r)
			err = fmt.Errorf("设备属性设置异常: %v", r)
		}
	}()

	// 平台作用域：系统路径，不按租户过滤
	device, err := p.dbClient.WithCtx(tenant.Platform(context.Background())).DeviceById(req.DeviceId)
	if err != nil {
		return err
	}

	_, err = p.dbClient.WithCtx(tenant.Platform(context.Background())).ProductById(device.ProductId)
	if err != nil {
		return err
	}

	deviceService, err := p.dbClient.WithCtx(tenant.Platform(context.Background())).DeviceServiceById(device.DriveInstanceId)
	if err != nil {
		return err
	}

	driverService := container.DriverServiceAppFrom(di.GContainer.Get)
	status := driverService.GetState(deviceService.Id)
	if status == constants.RunStatusStarted {
		client, errX := rpcclient.NewDriverRpcClient(deviceService.BaseAddress, false, "", deviceService.Id, p.lc)
		if errX != nil {
			return err
		}
		defer client.Close()

		var rpcRequest thingmodel.ThingModelIssueMsg
		rpcRequest.DeviceId = req.DeviceId
		rpcRequest.OperationType = thingmodel.OperationType_PROPERTY_SET
		var data dtos.PropertySet
		data.Version = "v1.0"
		data.MsgId = uuid.Generate().String()
		data.Time = time.Now().UnixMilli()
		data.Params = req.Item
		rpcRequest.Data = data.ToString()

		messageStore := container.MessageStoreItfFrom(p.dic.Get)
		ch := messageStore.GenAckChan(data.MsgId)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		_, err = client.ThingModelDownServiceClient.ThingModelMsgIssue(ctx, &rpcRequest)

		if err != nil {
			ch.TryCloseChan()
			return errort.NewCommonErr(errort.DefaultSystemError, err)
		}

		select {
		case <-time.After(10 * time.Second):
			ch.TryCloseChan()
			return errort.NewCommonErr(errort.DeviceLibraryResponseTimeOut, fmt.Errorf("driver id(%s) time out", deviceService.Id))
		case <-ctx.Done():
			return errort.NewCommonErr(errort.DeviceLibraryResponseTimeOut, fmt.Errorf("driver id(%s) time out", deviceService.Id))
		case resp := <-ch.DataChan:
			if v, ok := resp.(dtos.DevicePropertySetData); ok {
				if v.Success {
					return nil
				} else {
					return errort.NewCommonErr(v.Code, fmt.Errorf("%s", v.ErrorMessage))
				}
			}
		}

	}
	return errort.NewCommonErr(errort.DeviceServiceNotStarted, fmt.Errorf("driver id(%s) not start", deviceService.Id))
}

func (p *deviceApp) DeviceEffectivePropertyData(deviceEffectivePropertyDataReq dtos.DeviceEffectivePropertyDataReq) (dtos.DeviceEffectivePropertyDataResponse, error) {
	defer func() {
		if err := recover(); err != nil {
			p.lc.Error("Panic:", err)
		}
	}()

	// 平台作用域：系统路径，不按租户过滤
	device, err := p.dbClient.WithCtx(tenant.Platform(context.Background())).DeviceById(deviceEffectivePropertyDataReq.DeviceId)
	if err != nil {
		return dtos.DeviceEffectivePropertyDataResponse{}, err
	}

	_, err = p.dbClient.WithCtx(tenant.Platform(context.Background())).ProductById(device.ProductId)
	if err != nil {
		return dtos.DeviceEffectivePropertyDataResponse{}, err
	}

	deviceService, err := p.dbClient.WithCtx(tenant.Platform(context.Background())).DeviceServiceById(device.DriveInstanceId)
	if err != nil {
		return dtos.DeviceEffectivePropertyDataResponse{}, err
	}

	driverService := container.DriverServiceAppFrom(di.GContainer.Get)
	status := driverService.GetState(deviceService.Id)
	if status == constants.RunStatusStarted {

		client, errX := rpcclient.NewDriverRpcClient(deviceService.BaseAddress, false, "", deviceService.Id, p.lc)
		if errX != nil {
			return dtos.DeviceEffectivePropertyDataResponse{}, err
		}
		defer client.Close()
		var rpcRequest thingmodel.ThingModelIssueMsg
		rpcRequest.DeviceId = deviceEffectivePropertyDataReq.DeviceId
		rpcRequest.OperationType = thingmodel.OperationType_PROPERTY_GET
		var data dtos.DeviceGetPropertyData
		data.Version = "v1.0"
		data.MsgId = uuid.Generate().String()
		data.Time = time.Now().UnixMilli()
		data.Data = deviceEffectivePropertyDataReq.Codes
		rpcRequest.Data = data.ToString()

		messageStore := container.MessageStoreItfFrom(p.dic.Get)
		ch := messageStore.GenAckChan(data.MsgId)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		_, err = client.ThingModelDownServiceClient.ThingModelMsgIssue(ctx, &rpcRequest)

		if err != nil {
			ch.TryCloseChan()
			return dtos.DeviceEffectivePropertyDataResponse{}, errort.NewCommonErr(errort.DefaultSystemError, fmt.Errorf("system error"))
		}
		select {
		case <-time.After(10 * time.Second):
			ch.TryCloseChan()
			return dtos.DeviceEffectivePropertyDataResponse{}, errort.NewCommonErr(errort.DeviceLibraryResponseTimeOut, fmt.Errorf("driver id(%s) time out", deviceService.Id))
		case <-ctx.Done():
			return dtos.DeviceEffectivePropertyDataResponse{}, errort.NewCommonErr(errort.DeviceLibraryResponseTimeOut, fmt.Errorf("driver id(%s) time out", deviceService.Id))
		case resp := <-ch.DataChan:
			if v, ok := resp.([]dtos.EffectivePropertyData); ok {
				return dtos.DeviceEffectivePropertyDataResponse{
					Data: v,
				}, nil
			}
		}
	}
	return dtos.DeviceEffectivePropertyDataResponse{}, errort.NewCommonErr(errort.DeviceServiceNotStarted, fmt.Errorf("driver id(%s) not start", deviceService.Id))
}

func (p *deviceApp) DeviceInvokeThingService(ctx context.Context, invokeDeviceServiceReq dtos.InvokeDeviceServiceReq) (map[string]interface{}, error) {
	defer func() {
		if err := recover(); err != nil {
			p.lc.Error("Panic:", err)
		}
	}()

	// 平台作用域：系统路径，不按租户过滤
	device, err := p.dbClient.WithCtx(tenant.Platform(context.Background())).DeviceById(invokeDeviceServiceReq.DeviceId)
	if err != nil {
		return nil, err
	}

	_, err = p.dbClient.WithCtx(tenant.Platform(context.Background())).ProductById(device.ProductId)
	if err != nil {
		return nil, err

	}
	deviceService, err := p.dbClient.WithCtx(tenant.Platform(context.Background())).DeviceServiceById(device.DriveInstanceId)
	if err != nil {
		return nil, err

	}

	driverService := container.DriverServiceAppFrom(di.GContainer.Get)
	status := driverService.GetState(deviceService.Id)
	if status == constants.RunStatusStarted {
		client, errX := rpcclient.NewDriverRpcClient(deviceService.BaseAddress, false, "", deviceService.Id, p.lc)
		if errX != nil {
			return nil, err
		}
		defer client.Close()
		var rpcRequest thingmodel.ThingModelIssueMsg
		rpcRequest.DeviceId = invokeDeviceServiceReq.DeviceId
		rpcRequest.OperationType = thingmodel.OperationType_SERVICE_EXECUTE
		var data dtos.InvokeDeviceService
		data.Version = "v1.0"
		data.MsgId = uuid.Generate().String()
		data.Time = time.Now().UnixMilli()
		data.Data.Code = invokeDeviceServiceReq.Code
		data.Data.InputParams = invokeDeviceServiceReq.Items
		rpcRequest.Data = data.ToString()
		messageStore := container.MessageStoreItfFrom(p.dic.Get)
		ch := messageStore.GenAckChan(data.MsgId)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		_, err = client.ThingModelDownServiceClient.ThingModelMsgIssue(ctx, &rpcRequest)
		if err != nil {
			ch.TryCloseChan()
			return nil, errort.NewCommonErr(errort.DefaultSystemError, fmt.Errorf("system error"))
		}
		//******
		persistItf := container.PersistItfFrom(p.dic.Get)
		var saveServiceInfo dtos.ThingModelMessage
		saveServiceInfo.OpType = int32(thingmodel.OperationType_SERVICE_EXECUTE)
		saveServiceInfo.Cid = device.Id
		var saveData dtos.SaveServiceIssueData
		saveData.MsgId = data.MsgId
		saveData.Code = invokeDeviceServiceReq.Code
		saveData.Time = data.Time
		saveData.InputParams = make(map[string]interface{})
		saveData.InputParams["code"] = invokeDeviceServiceReq.Code
		saveData.InputParams["inputParams"] = invokeDeviceServiceReq.Items
		//******

		select {
		case <-time.After(10 * time.Second):
			ch.TryCloseChan()
			saveData.OutputParams = map[string]interface{}{
				"result":  false,
				"message": "wait response timeout",
			}
			s, _ := json.Marshal(saveData)
			saveServiceInfo.Data = string(s)
			_ = persistItf.SaveDeviceThingModelData(ctx, saveServiceInfo)
			return nil, errort.NewCommonErr(errort.DeviceLibraryResponseTimeOut, fmt.Errorf("driver id(%s) time out", deviceService.Id))
		case <-ctx.Done():
			saveData.OutputParams = map[string]interface{}{
				"result":  false,
				"message": "wait response timeout",
			}
			s, _ := json.Marshal(saveData)
			saveServiceInfo.Data = string(s)
			_ = persistItf.SaveDeviceThingModelData(ctx, saveServiceInfo)
			return nil, errort.NewCommonErr(errort.DeviceLibraryResponseTimeOut, fmt.Errorf("driver id(%s) time out", deviceService.Id))
		case resp := <-ch.DataChan:
			if v, ok := resp.(map[string]interface{}); ok {
				saveData.OutputParams = v
				s, _ := json.Marshal(saveData)
				saveServiceInfo.Data = string(s)
				_ = persistItf.SaveDeviceThingModelData(ctx, saveServiceInfo)

				return v, nil
			}
		}
	}
	return nil, errort.NewCommonErr(errort.DeviceServiceNotStarted, fmt.Errorf("driver id(%s) not start", deviceService.Id))
}
