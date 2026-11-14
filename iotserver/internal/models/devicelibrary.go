package models

import (
	"database/sql/driver"
	"encoding/json"
)

// 插件类别。
const (
	PluginCategoryDriver    = "driver"
	PluginCategorySystem    = "system"
	PluginCategoryConnector = "connector"
)

// 插件运行时。
const (
	PluginRuntimeNative  = "native"
	PluginRuntimeDocker  = "docker"
	PluginRuntimeBuiltin = "builtin"
	// PluginRuntimeIotpGW 嵌入式 iotpGW 的 .so 驱动库。
	PluginRuntimeIotpGW = "iotpgw-so"
)

// 驱动库 / 插件包。
type DeviceLibrary struct {
	Timestamps      `gorm:"embedded"`
	Id              string          `gorm:"id;primaryKey;not null;type:string;size:255;comment:主键"`
	Name            string          `gorm:"type:string;size:255;comment:名字"`
	Description     string          `gorm:"type:text;comment:描述"`
	Protocol        string          `gorm:"type:string;size:255;comment:驱动协议"`
	Version         string          `gorm:"type:string;size:255;comment:当前安装版本"`
	ContainerName   string          `gorm:"type:string;size:255;comment:容器名字"`
	DockerConfigId  string          `gorm:"type:string;size:255;comment:镜像仓库配置表id"`
	DockerRepoName  string          `gorm:"type:string;size:255;comment:镜像名称"`
	DockerImageId   string          `gorm:"type:string;size:255;comment:镜像ID"`
	SupportVersions SupportVersions `gorm:"type:text;not null;comment:可用版本"`
	IsInternal      bool            `gorm:"default:0;not null;comment:是否内置，云端内置驱动"`
	Language        string          `gorm:"type:string;size:255;comment:代码语言"`
	// Category 插件类别：driver / system / connector
	Category string `gorm:"column:category;type:string;size:32;default:'driver';index;comment:插件类别"`
	// Runtime 运行时：native / docker / builtin
	Runtime string `gorm:"column:runtime;type:string;size:32;default:'docker';comment:运行时"`
	// Arch 适用的 CPU 架构：armv7 / arm64 / amd64 / any
	Arch string `gorm:"column:arch;type:string;size:32;default:'amd64';comment:CPU架构"`
	// Manifest 插件清单 JSON（入口、配置 schema、健康检查、权限等）
	Manifest string `gorm:"column:manifest;type:text;comment:插件清单JSON"`
	// PackagePath / PackageHash / PackageSize 安装包信息（本地导入时使用）
	PackagePath string `gorm:"column:package_path;type:string;size:512;comment:安装包路径"`
	PackageHash string `gorm:"column:package_hash;type:string;size:128;comment:安装包SHA256"`
	PackageSize int64  `gorm:"column:package_size;type:bigint;comment:安装包大小"`
	// Signature 安装包签名
	Signature string `gorm:"column:signature;type:string;size:512;comment:安装包签名"`
	// Essential 是否系统必需组件（不可卸载）
	Essential bool `gorm:"column:essential;type:bool;comment:是否系统必需组件"`
	// MinCoreVersion 最低核心版本
	MinCoreVersion string `gorm:"column:min_core_version;type:string;size:64;comment:最低核心版本"`
	// Source 来源：builtin / upload / market
	Source string `gorm:"column:source;type:string;size:32;default:'market';comment:来源"`
	DeviceMarket
	OperateStatus string `gorm:"-"`
}

type DeviceMarket struct {
	Manual     string `gorm:"type:string;size:255;comment:驱动市场使用说明手册"`
	Icon       string `gorm:"type:text;comment:图标"`
	ClassifyId int    `gorm:"comment:分类"`
}

func (d DeviceLibrary) ConfigBody() string {
	return ""
}

func (d DeviceLibrary) GetConfig() (DeviceLibraryConfig, error) {
	body := d.ConfigBody()
	var dc DeviceLibraryConfig
	if err := json.Unmarshal([]byte(body), &dc); err != nil {
		return DeviceLibraryConfig{}, err
	}
	return dc, nil
}

func (d DeviceLibrary) GetConfigMap() (map[string]interface{}, error) {
	body := d.ConfigBody()
	var dc map[string]interface{}
	if err := json.Unmarshal([]byte(body), &dc); err != nil {
		return nil, err
	}
	return dc, nil
}

type SupportVersions []SupportVersion

type SupportVersion struct {
	Version            string
	ConfigFile         string
	IsDefault          bool // 是否为默认版本
	DockerParamsSwitch bool
	DockerParams       string
	ExpertMode         bool
	ExpertModeContent  string
	ConfigJson         string
}

func (c SupportVersions) Value() (driver.Value, error) {
	return GormValueWrap(c)
}

func (c *SupportVersions) Scan(value interface{}) error {
	return GormScanWrap(value, c)
}

func (table *DeviceLibrary) TableName() string {
	return "device_library"
}

func (table *DeviceLibrary) Get() interface{} {
	return *table
}

func (dl DeviceLibrary) DefaultVersion() SupportVersion {
	for _, v := range dl.SupportVersions {
		if v.Version == dl.Version {
			return v
		}
	}
	return SupportVersion{}
}

type DeviceLibraryConfig struct {
	DeviceServer map[string][]struct {
		Name         string      `json:"name"`
		Display      string      `json:"display"`
		Type         string      `json:"type"` // type 支持 int、string、float、bool、select、object、array
		DefaultValue string      `json:"defaultValue"`
		Options      interface{} `json:"options"`
	} `json:"deviceServer"`
	DeviceProtocols interface{}   `json:"deviceProtocols"`
	DeviceDpAttrs   []interface{} `json:"deviceDpAttrs"`
}

func GetLibrarySimpleBaseConfig() string {
	return `{"deviceServer": {},"deviceProtocols": {},"deviceDpAttrs": []}`
}
