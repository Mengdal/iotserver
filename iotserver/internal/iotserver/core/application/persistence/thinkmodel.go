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

package persistence

import (
	"context"

	"iotserver/internal/dtos"
	resourceContainer "iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/tools/datadb/aggregate"
)

// persistApp 时序数据持久化。
//
// 数据层使用 DuckDB（单文件列存），共用同一套编排逻辑，
// 因此这里不再按 GetDataDBType() 分发。
//
// 租户作用域：所有读写都从传入的 ctx 取租户（见 pkg/tenant）。
//   - 读：仓储层用该 ctx 查设备/产品，GORM 租户插件会自动追加 tenant_id 条件，
//     跨租户访问设备会直接查不到，无需在时序库里再拼一层过滤；
//   - 写：上行消息不带租户，由设备归属推导后注入 ctx（见 thinkmodel_lake.go）。
type persistApp struct {
	dic          *di.Container
	lc           logger.LoggingClient
	dbClient     interfaces.DBClient
	dataDbClient interfaces.DataDBClient
	tenantCache  *deviceTenantCache
}

func NewPersistApp(dic *di.Container) *persistApp {
	lc := container.LoggingClientFrom(dic.Get)
	dbClient := resourceContainer.DBClientFrom(dic.Get)
	dataDbClient := resourceContainer.DataDBClientFrom(dic.Get)
	return &persistApp{
		lc:           lc,
		dic:          dic,
		dbClient:     dbClient,
		dataDbClient: dataDbClient,
		tenantCache:  newDeviceTenantCache(defaultDeviceTenantTTL, defaultDeviceTenantCacheMax),
	}
}

// SaveDeviceThingModelData 落库设备上行数据（属性/事件/服务/批量补传）
func (pst *persistApp) SaveDeviceThingModelData(ctx context.Context, req dtos.ThingModelMessage) error {
	return pst.saveDeviceThingModelData(ctx, req)
}

// SearchDeviceThingModelPropertyData 查询设备各属性最新值
func (pst *persistApp) SearchDeviceThingModelPropertyData(ctx context.Context, req dtos.ThingModelPropertyDataRequest) (interface{}, error) {
	return pst.searchDeviceLatestPropertyData(ctx, req)
}

// SearchDeviceThingModelHistoryPropertyData 查询属性历史（支持聚合）
func (pst *persistApp) SearchDeviceThingModelHistoryPropertyData(ctx context.Context, req dtos.ThingModelPropertyDataRequest) (interface{}, int, error) {
	// 聚合参数校验与归一化（数据层共享）
	interval, method, err := aggregate.Validate(req.AggInterval, req.AggMethod, req.Range)
	if err != nil {
		return nil, 0, errort.NewCommonErr(errort.DefaultReqParamsError, err)
	}
	req.AggInterval, req.AggMethod = interval, method
	return pst.searchDeviceHistoryPropertyData(ctx, req)
}

// SearchDeviceThingModelServiceData 查询服务调用历史
func (pst *persistApp) SearchDeviceThingModelServiceData(ctx context.Context, req dtos.ThingModelServiceDataRequest) ([]dtos.ThingModelServiceDataResponse, int, error) {
	return pst.searchDeviceServiceData(ctx, req)
}

// SearchDeviceThingModelEventData 查询事件历史
func (pst *persistApp) SearchDeviceThingModelEventData(ctx context.Context, req dtos.ThingModelEventDataRequest) ([]dtos.ThingModelEventDataResponse, int, error) {
	return pst.searchDeviceEventData(ctx, req)
}

// SearchDeviceMsgCount 统计消息总数（属性 + 事件 + 服务）
//
// 租户作用域由 ctx 决定：平台作用域统计全平台，租户作用域只统计该租户。
func (pst *persistApp) SearchDeviceMsgCount(ctx context.Context, startTime, endTime int64) (int, error) {
	return pst.searchDeviceMsgCount(ctx, startTime, endTime)
}

func getEventTypeAndName(events []models.Events, code string) (string, string) {
	for _, event := range events {
		if event.Code == code {
			return event.EventType, event.Name
		}
	}
	return "", ""
}

func getServiceName(events []models.Actions, code string) string {
	for _, event := range events {
		if event.Code == code {
			return event.Name
		}
	}
	return ""
}
