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

package interfaces

import (
	"context"
	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
)

type DataDBClient interface {
	GetDataDBType() constants.DataType
	CloseSession()

	Insert(ctx context.Context, table string, data map[string]interface{}) (err error)
	GetDeviceProperty(req dtos.ThingModelPropertyDataRequest, device models.Device) ([]dtos.ReportData, int, error)
	GetDeviceService(req dtos.ThingModelServiceDataRequest, device models.Device, product models.Product) ([]dtos.SaveServiceIssueData, int, error)
	GetDeviceEvent(req dtos.ThingModelEventDataRequest, device models.Device, product models.Product) ([]dtos.EventData, int, error)

	CreateTable(ctx context.Context, stable, table string) (err error)
	DropTable(ctx context.Context, table string) (err error)

	CreateStable(ctx context.Context, product models.Product) (err error)
	DropStable(ctx context.Context, table string) (err error)

	AddDatabaseField(ctx context.Context, tableName string, specsType constants.SpecsType, code string, name string) (err error)
	DelDatabaseField(ctx context.Context, tableName, code string) (err error)
	ModifyDatabaseField(ctx context.Context, tableName string, specsType constants.SpecsType, code string, name string) (err error)

	GetDevicePropertyCount(dtos.ThingModelPropertyDataRequest) (int, error)
	GetDeviceEventCount(req dtos.ThingModelEventDataRequest) (int, error)
	GetDeviceMsgCountByGiveTime(deviceId string, startTime, endTime int64) (int, error)
}

// MsgCounter 可选能力：数据层能用**一条 SQL** 统计时间段内的消息总数。
//
// 时间序列消息量按设备数线性增长，按设备逐台汇总会退化成 N+1 查询，
// 因此支持该能力的数据层走快路径；未实现的形态退回按设备汇总
// （设备列表本身已由仓储层做租户过滤，语义仍然正确）。
type MsgCounter interface {
	// CountAllMsgByTimeRange 统计消息总数；tenantID 为空表示不限租户（平台作用域）
	CountAllMsgByTimeRange(tenantID string, startTime, endTime int64) (int, error)
}

// DeviceMsgCounter 可选能力：按设备分组统计时间窗内的消息数（看板排行榜用）。
//
// 一次 GROUP BY 完成，避免逐设备统计的 N+1；tenantID 为空表示不限租户（平台作用域）。
// 时间窗内没有消息的设备不会出现在结果里。
type DeviceMsgCounter interface {
	CountMsgGroupByDevice(tenantID string, startTime, endTime int64) (map[string]int, error)
}

// DataPurger 可选能力：数据层支持按时间清理时序数据。
//
// 保留策略由应用层驱动（见 application/retention.go）：按租户读取各自的保留天数，
// 逐租户调用 PurgeTenantBefore，这样改配置不必重启，也才能做到分级保留。
// 未实现该能力的数据层（如 ducklake）保留自带的全局维护逻辑。
type DataPurger interface {
	// PurgeBefore 清理所有租户 cutoff（毫秒）之前的数据
	PurgeBefore(ctx context.Context, cutoffMillis int64) (int64, error)
	// PurgeTenantBefore 只清理指定租户 cutoff（毫秒）之前的数据
	PurgeTenantBefore(ctx context.Context, tenantID string, cutoffMillis int64) (int64, error)
}

// DatabaseExporter 可选能力：数据层支持导出**一致的在线快照**用于备份。
//
// 数据库文件被运行中的进程独占，直接拷贝不安全；实现该能力的数据层
// 会在导出前先 CHECKPOINT，由引擎自身保证快照一致。
type DatabaseExporter interface {
	// ExportDatabase 把当前库导出到 dir
	ExportDatabase(ctx context.Context, dir string) error
}
