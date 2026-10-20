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
package constants

import (
	"github.com/LMgateways/iotserver-driver-proto/driverdevice"
)

type DeviceStatus string

const (
	DeviceOnline  = "online"
	DeviceOffline = "offline"
)

// 设备状态入库用稳定代码，不存展示文案：切换中英文时只翻译展示层，
// 数据库与接口参数不随语言变化。
const (
	DeviceStatusUnKnow   DeviceStatus = "unknown"
	DeviceStatusOnline   DeviceStatus = "online"
	DeviceStatusOffline  DeviceStatus = "offline"
	DeviceStatusUnActive DeviceStatus = "inactive"
	DeviceStatusDisable  DeviceStatus = "disabled"
)

func (i DeviceStatus) TransformToDriverDeviceStatus() driverdevice.DeviceStatus {
	switch i {
	case DeviceStatusUnKnow:
		return driverdevice.DeviceStatus_UnKnowStatus
	case DeviceStatusOnline:
		return driverdevice.DeviceStatus_OnLine
	case DeviceStatusOffline:
		return driverdevice.DeviceStatus_OffLine
	case DeviceStatusUnActive:
		return driverdevice.DeviceStatus_UnActive
	case DeviceStatusDisable:
		return driverdevice.DeviceStatus_Disable
	default:
		return driverdevice.DeviceStatus_UnKnowStatus
	}
}
