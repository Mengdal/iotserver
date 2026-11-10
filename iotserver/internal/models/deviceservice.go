//
// Copyright (C) 2020 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package models

import (
	"gorm.io/gorm"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/utils"
	"strings"
	//"gitlab.com/tedge/edgex/internal/pkg/constants"
)

// DeviceService and its properties are defined in the APIv2 specification:
// https://app.swaggerhub.com/apis-docs/EdgeXFoundry1/core-metadata/2.x#/DeviceService
// Model fields are same as the DTOs documented by this swagger. Exceptions, if any, are noted below.
type DeviceService struct {
	Timestamps        `gorm:"embedded"`
	Id                string             `gorm:"id;primaryKey;not null;type:string;size:255;comment:主键"`
	Name              string             `gorm:"type:string;size:255;comment:名字"`
	InstanceName      string             `gorm:"column:instance_name;type:string;size:255;index;comment:插件实例名称"`
	BaseAddress       string             `gorm:"type:string;size:255;comment:地址"`
	DeviceLibraryId   string             `gorm:"type:string;size:255;comment:插件包ID"`
	Config            MapStringInterface `gorm:"type:string;size:255;comment:配置"`
	DockerContainerId string             `gorm:"type:string;size:255;comment:docker容器ID"`
	// Runtime 实例运行时：native / docker / builtin
	Runtime string `gorm:"column:runtime;type:string;size:32;default:'docker';comment:运行时"`
	// InstallPath 原生插件的安装目录
	InstallPath string `gorm:"column:install_path;type:string;size:512;comment:安装目录"`
	// Pid 原生进程 PID（0 表示未运行）
	Pid int `gorm:"column:pid;type:int;comment:进程PID"`
	// DesiredState 期望状态：running / stopped
	DesiredState string `gorm:"column:desired_state;type:string;size:32;comment:期望状态"`
	// ActualState 实际状态：starting / running / stopping / stopped / error
	ActualState string `gorm:"column:actual_state;type:string;size:32;comment:实际状态"`
	// HealthStatus 健康状态：healthy / unhealthy / unknown
	HealthStatus string `gorm:"column:health_status;type:string;size:32;comment:健康状态"`
	// LastHealthAt 最近一次健康检查时间（毫秒）
	LastHealthAt int64 `gorm:"column:last_health_at;type:bigint;comment:最近健康检查时间"`
	// LastError 最近一次错误
	LastError string `gorm:"column:last_error;type:string;size:1000;comment:最近错误"`
	// ConfigVersion 配置版本，每次应用配置 +1
	ConfigVersion int `gorm:"column:config_version;type:int;comment:配置版本"`
	// ResourceUsage 资源使用快照（JSON）
	ResourceUsage      string             `gorm:"column:resource_usage;type:text;comment:资源使用快照"`
	ExpertMode         bool               `gorm:"comment:扩展模式"`
	ExpertModeContent  string             `gorm:"comment:扩展内容"`
	DockerParamsSwitch bool               `gorm:"comment:docker启动参数开关"`
	DockerParams       string             `gorm:"type:text;comment:docker启动参数"`
	ContainerName      string             `gorm:"type:string;size:255;comment:容器名字"`
	LogLevel           constants.LogLevel `gorm:"default:1;comment:日志等级"`
	DriverType         int                `gorm:"default:1;not null;comment:驱动类别，1：驱动，2：三方应用"`
	RunStatus          int                `gorm:"-"`
	ImageExist         bool               `gorm:"-"`
	Platform           constants.IotPlatform
}

func (d *DeviceService) TableName() string {
	return "device_service"
}

func (d *DeviceService) Get() interface{} {
	return *d
}

func (d *DeviceService) IsRunning() bool {
	return d.RunStatus == constants.RunStatusStarted
}

func (d *DeviceService) IsStopped() bool {
	return d.RunStatus == constants.RunStatusStopped
}

func (d *DeviceService) GetBaseAddress() string {
	if d.BaseAddress == "" {
		return constants.DefaultDriverBaseAddress
	}
	return d.BaseAddress
}

func (d *DeviceService) GetPort() string {
	tmpAddr := strings.Split(d.BaseAddress, ":")
	if len(tmpAddr) >= 2 {
		return tmpAddr[1]
	}
	return ""
}

func (d *DeviceService) IsDriver() bool {
	return d.DriverType == constants.DriverLibTypeDefault
}

type DeviceServiceExtendConf struct {
	ConfigFilePath string
	Mount          []string
	Port           int
}

func (d *DeviceService) BeforeCreate(tx *gorm.DB) (err error) {
	var mqttAuth MqttAuth
	mqttAuth.Id = utils.RandomNum()
	mqttAuth.ResourceType = constants.DriverResource
	mqttAuth.ResourceId = d.Id
	mqttAuth.ClientId = utils.GenUUID()
	mqttAuth.UserName = "edge-driver" + d.Id
	mqttAuth.Password = utils.GenUUID()
	return tx.Model(&MqttAuth{}).Create(&mqttAuth).Error
}

func (d *DeviceService) BeforeDelete(tx *gorm.DB) (err error) {
	var mqttAuth MqttAuth
	return tx.Model(&MqttAuth{}).Where("resource_type = ? and resource_id = ?", constants.DriverResource, d.Id).Delete(&mqttAuth).Error
}
