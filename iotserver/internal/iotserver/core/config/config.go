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
	Shaper              ShaperInfo
	Pprof               PprofInfo
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

// ShaperInfo 可视化看板（Shaper）嵌入配置
type ShaperInfo struct {
	// Enabled 打开后前端「数据看板」页面才会去嵌入
	Enabled bool
	// BaseUrl Shaper 实例地址，必须浏览器可直连（用于加载 /embed/shaper.js）
	BaseUrl string
	// ApiKey Shaper Admin → API Keys 创建的密钥（需要 jwt 权限），只留在服务端
	ApiKey string
	// DashboardId 默认嵌入的看板 ID
	DashboardId string
	// HomeDashboardId 首页卡片用的看板 ID（可与 DashboardId 不同）
	HomeDashboardId string
	// BrowserBaseUrl 返回给浏览器的地址（容器部署时服务端用 http://shaper:5454、
	// 浏览器要用 http://127.0.0.1:5454，两者不同）；为空则与 BaseUrl 相同
	BrowserBaseUrl string
	// BoardsFile 看板管理用的登记文件（默认 .shaper-boards.json）：
	// Shaper 的接口不返回看板 SQL 原文，所以 SQL 副本与「首页/默认」标记存在这里
	BoardsFile string
	// LocalEnvFile 本地未入库的 KEY=VALUE 文件（默认 .env.shaper），
	// 用于在没人带环境变量启动时也能拿到 ApiKey/DashboardId
	LocalEnvFile string
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
