/*******************************************************************************
 * Copyright 2017 Dell Inc.
 * Copyright (c) 2019 Intel Corporation
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
package models

import (
	"crypto/hmac"
	"crypto/sha1"
	"database/sql/driver"
	"encoding/hex"
	"github.com/LMgateways/iotserver-driver-proto/driverdevice"
	"iotserver/internal/pkg/constants"
	"strings"
)

type Device struct {
	Timestamps  `gorm:"embedded"`
	TenantField `gorm:"embedded"`
	Id          string `gorm:"id;primaryKey;not null;type:string;size:255;comment:主键"`
	// DeptId 所属组织节点（工厂/车间/产线），用于组织维度归属
	DeptId string `gorm:"column:dept_id;type:string;size:64;index;comment:组织节点ID"`
	// GatewayId 所属工业网关（阶段二：第三方网关接入时绑定）
	GatewayId string `gorm:"column:gateway_id;type:string;size:64;index;comment:所属网关ID"`
	// DeviceKey 网关内设备标识：网关上报时用它映射到本设备
	DeviceKey string `gorm:"column:device_key;type:string;size:255;index;comment:网关内设备标识"`
	// ProvisionSource 设备来源，取值见 DeviceSource*
	ProvisionSource string `gorm:"column:provision_source;type:string;size:32;comment:设备来源"`
	// ProvisionState 供应状态，取值见 DeviceState*（清单摘除不删行，只改状态）
	ProvisionState  string                 `gorm:"column:provision_state;type:string;size:32;index;comment:供应状态"`
	CloudDeviceId   string                 `gorm:"type:string;size:255;comment:云设备ID"`
	CloudProductId  string                 `gorm:"type:string;size:255;comment:云产品ID"`
	CloudInstanceId string                 `gorm:"index;type:string;size:255;comment:云实例ID"`
	DriveInstanceId string                 `gorm:"index;type:string;size:255;comment:驱动实例ID"`
	Name            string                 `gorm:"type:string;size:255;comment:名字"`
	DeviceSn        string                 `gorm:"type:string;size:255;comment:设备唯一编码"`
	Status          constants.DeviceStatus `gorm:"type:string;size:50;index;comment:设备状态"`
	Description     string                 `gorm:"type:text;comment:描述"`
	ProductId       string                 `gorm:"type:string;size:255;comment:产品ID"`
	Secret          string                 `gorm:"type:string;size:255;comment:密钥"`
	Platform        constants.IotPlatform  `gorm:"type:string;size:50;comment:平台名称"`
	InstallLocation string                 `gorm:"type:string;size:255;comment:安装地址"`
	LastSyncTime    int64                  `gorm:"comment:最后一次同步时间"`
	LastOnlineTime  int64                  `gorm:"comment:最后一次在线时间"`
	Product         Product                `gorm:"foreignKey:ProductId"`
}

// ProtocolProperties contains the device connection information in key/value pair
type ProtocolProperties map[string]string

type Protocols map[string]ProtocolProperties

func (c Protocols) Value() (driver.Value, error) {
	return GormValueWrap(c)
}

func (c *Protocols) Scan(value interface{}) error {
	return GormScanWrap(value, c)
}

// Device 来源（Device.ProvisionSource）
const (
	DeviceSourceManual  = "manual"  // 平台手工创建
	DeviceSourceGateway = "gateway" // 随网关清单自动注册
	DeviceSourceDriver  = "driver"  // 经驱动实例接入
)

// Device 供应状态（Device.ProvisionState）
const (
	DeviceStateActive = "active" // 正常
	// DeviceStateAbsent 网关全量清单里已不含该设备。
	// 只改状态不删行：时序历史在时序库里，但删除元数据会断掉告警规则/组态的引用。
	DeviceStateAbsent = "absent"
)

// TableDevice 设备表名
const TableDevice = "device"

func (table *Device) TableName() string {
	return "device"
}

func (table *Device) Get() interface{} {
	return *table
}

func (table *Device) TransformToDriverDevice() *driverdevice.Device {
	driverDevice := new(driverdevice.Device)
	driverDevice.Id = table.Id
	driverDevice.Name = table.Name
	driverDevice.ProductId = table.ProductId
	driverDevice.Description = table.Description
	driverDevice.Status = table.Status.TransformToDriverDeviceStatus()
	driverDevice.ProductId = table.ProductId
	driverDevice.Secret = table.Secret
	driverDevice.Platform = table.Platform.TransformToDriverDevicePlatform()
	return driverDevice
}

func HmacSha1(keyStr, value string) string {
	key := []byte(keyStr)
	mac := hmac.New(sha1.New, key)
	mac.Write([]byte(value))
	res := hex.EncodeToString(mac.Sum(nil))
	return strings.ToUpper(res)
}
