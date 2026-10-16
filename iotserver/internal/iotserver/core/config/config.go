/*******************************************************************************
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

package config

import (
	"fmt"

	"go.uber.org/atomic"

	bootstrapConfig "iotserver/internal/pkg/config"
)

var (
	DefaultEnv = "pro"
)

// Struct used to parse the JSON configuration file
type ConfigurationStruct struct {
	Writable            WritableInfo
	MessageQueue        MessageQueueInfo
	Clients             map[string]bootstrapConfig.ClientInfo
	Databases           map[string]map[string]bootstrapConfig.Database
	Registry            bootstrapConfig.RegistryInfo
	Service             bootstrapConfig.ServiceInfo
	RpcServer           bootstrapConfig.RPCServiceInfo
	SecretStore         bootstrapConfig.SecretStoreInfo
	WebServer           bootstrapConfig.ServiceInfo
	DockerManage        DockerManage
	ApplicationSettings ApplicationSettings
	Viz                 VizInfo
	Pprof               PprofInfo
	Tenant              TenantInfo
	Topics              struct {
		CommandTopic TopicInfo
	}
}

// PprofInfo 调试端点配置。
// 默认关闭：/debug/pprof 会暴露 goroutine 栈、堆内存（可能含令牌/密码）与可消耗 CPU 的 profile 接口。
// 开启后仍需 x-token 鉴权并具备 system:debug:pprof 权限（超级管理员直通）。
type PprofInfo struct {
	Enabled bool
}

// VizInfo 画布看板（布局 + 文本组件）配置
type VizInfo struct {
	// BoardsFile 历史字段：看板早已改为落库（viz_board 表）并按租户隔离，
	// 内置看板也改成了编译期种子（internal/pkg/seed/boards）。
	// 这里保留仅为兼容旧配置文件，不再读取。
	BoardsFile string
}

// TenantInfo 多租户隔离配置
type TenantInfo struct {
	// Enforce 无租户上下文的调用是否 fail-closed。
	//   false（默认，迁移期）：不打租户条件，仅打告警日志（日志级别可控制）—— 未改造完的调用点仍可工作；
	//   true（上线/测试）：租户表查询在缺少租户上下文时返回空集，写入直接报错。
	// 建议在业务接口全部完成 WithCtx 改造、隔离测试跑通后再置 true。
	Enforce bool
}

type WritableInfo struct {
	PersistData     atomic.Bool  `toml:"-"`
	PersistPeriod   atomic.Int32 `toml:"-"`
	LogLevel        string
	LogPath         string
	InsecureSecrets bootstrapConfig.InsecureSecrets
	DebugProfile    bool
	IsNewModel      bool
	LimitMethods    []string
}

type TopicInfo struct {
	Topic string
}

type DockerManage struct {
	ContainerConfigPath string
	HostRootDir         string
	DockerApiVersion    string
	Privileged          bool
}

// MessageQueueInfo provides parameters related to connecting to a message bus
type MessageQueueInfo struct {
	// Host is the hostname or IP address of the broker, if applicable.
	Host string
	// Port defines the port on which to access the message queue.
	Port int
	// Protocol indicates the protocol to use when accessing the message queue.
	Protocol string
	// Indicates the message queue platform being used.
	Type string
	// Indicates the topic the data is published/subscribed
	SubscribeTopics []string
	// Indicates the topic prefix the data is published to. Note that /<device-profile-name>/<device-name> will be
	// added to this Publish Topic prefix as the complete publish topic
	PublishTopicPrefix string
	// Provides additional configuration properties which do not fit within the existing field.
	// Typically the key is the name of the configuration property and the value is a string representation of the
	// desired value for the configuration property.
	Optional map[string]string
}

func (m MessageQueueInfo) Enable() bool {
	return !(m.Host == "" || m.Port <= 0)
}

type ApplicationSettings struct {
	DeviceIds       string
	ResDir          string
	DriverDir       string
	IsScreenControl bool
	OTADir          string
	CloseAuthToken  bool
	GatewayEdition  string // 网关版本标识
	WebBuildPath    string // 前端路径
	TedgeNumber     string
}

// URL constructs a URL from the protocol, host and port and returns that as a string.
func (m MessageQueueInfo) URL() string {
	return fmt.Sprintf("%s://%s:%v", m.Protocol, m.Host, m.Port)
}

// UpdateFromRaw converts configuration received from the registry to a service-specific configuration struct which is
// then used to overwrite the service's existing configuration struct.
func (c *ConfigurationStruct) UpdateFromRaw(rawConfig interface{}) bool {
	configuration, ok := rawConfig.(*ConfigurationStruct)
	if ok {
		// Check that information was successfully read from Registry
		if configuration.Service.Port == 0 {
			return false
		}
		*c = *configuration
	}
	return ok
}

// EmptyWritablePtr returns a pointer to a service-specific empty WritableInfo struct.  It is used by the bootstrap to
// provide the appropriate structure to registry.C's WatchForChanges().
func (c *ConfigurationStruct) EmptyWritablePtr() interface{} {
	return &WritableInfo{}
}

// UpdateWritableFromRaw converts configuration received from the registry to a service-specific WritableInfo struct
// which is then used to overwrite the service's existing configuration's WritableInfo struct.
func (c *ConfigurationStruct) UpdateWritableFromRaw(rawWritable interface{}) bool {
	writable, ok := rawWritable.(*WritableInfo)
	if ok {
		c.Writable = *writable
	}
	return ok
}

// GetBootstrap returns the configuration elements required by the bootstrap.  Currently, a copy of the configuration
// data is returned.  This is intended to be temporary -- since ConfigurationStruct drives the configuration.toml's
// structure -- until we can make backwards-breaking configuration.toml changes (which would consolidate these fields
// into an bootstrapConfig.BootstrapConfiguration struct contained within ConfigurationStruct).
func (c *ConfigurationStruct) GetBootstrap() bootstrapConfig.BootstrapConfiguration {
	// temporary until we can make backwards-breaking configuration.toml change
	return bootstrapConfig.BootstrapConfiguration{
		Clients:     c.Clients,
		Service:     c.Service,
		RpcServer:   c.RpcServer,
		Registry:    c.Registry,
		SecretStore: c.SecretStore,
	}
}

// GetLogLevel returns the current ConfigurationStruct's log level.
func (c *ConfigurationStruct) GetLogLevel() string {
	return c.Writable.LogLevel
}

func (c *ConfigurationStruct) GetLogPath() string {
	return c.Writable.LogPath
}

// GetRegistryInfo returns the RegistryInfo from the ConfigurationStruct.
func (c *ConfigurationStruct) GetRegistryInfo() bootstrapConfig.RegistryInfo {
	return c.Registry
}

// GetDatabaseInfo returns a database information map.
func (c *ConfigurationStruct) GetDatabaseInfo() map[string]bootstrapConfig.Database {
	cfg := c.Databases["Metadata"]
	return cfg
}

// GetDataDatabaseInfo returns a database information map for events & readings.
func (c *ConfigurationStruct) GetDataDatabaseInfo() map[string]bootstrapConfig.Database {
	cfg := c.Databases["Data"]
	return cfg
}

// GetDataDatabaseInfo returns a database information map for events & readings.
func (c *ConfigurationStruct) GetRedisInfo() map[string]bootstrapConfig.Database {
	cfg := c.Databases["Redis"]
	return cfg
}

// GetInsecureSecrets returns the service's InsecureSecrets.
func (c *ConfigurationStruct) GetInsecureSecrets() bootstrapConfig.InsecureSecrets {
	return c.Writable.InsecureSecrets
}

// 判断是否为物模型
func (c *ConfigurationStruct) IsThingModel() bool {
	return c.Writable.IsNewModel
}

func (c *ConfigurationStruct) GetPersistData() bool {
	return c.Writable.PersistData.Load()
}

func (c *ConfigurationStruct) GetPersisPeriod() int32 {
	return c.Writable.PersistPeriod.Load()
}
