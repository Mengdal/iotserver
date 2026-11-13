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
	"strings"

	bootstrapConfig "iotserver/internal/pkg/config"
)

// Struct used to parse the JSON configuration file
type ConfigurationStruct struct {
	Writable     WritableInfo
	MessageQueue MessageQueueInfo
	Clients      map[string]bootstrapConfig.ClientInfo
	// Databases 按用途归属：Metadata（元数据）/ Data（时序）。
	Databases   map[string]bootstrapConfig.Database
	Registry    bootstrapConfig.RegistryInfo
	Service     bootstrapConfig.ServiceInfo
	RpcServer   bootstrapConfig.RPCServiceInfo
	SecretStore bootstrapConfig.SecretStoreInfo
	WebServer   bootstrapConfig.ServiceInfo
	// WebUI 前端静态资源目录。本地部署时指向 /opt/iotserver/ui。
	WebUI        WebUIInfo
	DockerManage DockerManage
	Pprof        PprofInfo
	Tenant       TenantInfo
	// Deployment 部署形态：server 支持 Docker 驱动；embedded 以本机进程运行插件。
	Deployment DeploymentInfo
	// Plugins 原生插件安装目录与实例数据目录。
	Plugins PluginsInfo
	// Integrations 外部系统接入配置，按被接入方归属。
	Integrations IntegrationsInfo
	// ComponentLogs 系统组件日志源；不配置时使用内置默认路径。
	ComponentLogs ComponentLogsInfo
}

// ComponentLogsInfo 系统组件日志文件配置。
type ComponentLogsInfo struct {
	Sources []ComponentLogSource
}

// ComponentLogSource 一个系统组件的日志文件来源。
// Path 为空表示显式禁用该组件的日志入口。
type ComponentLogSource struct {
	Id   string
	Name string
	Path string
}

// WebUIInfo 前端静态资源目录。
type WebUIInfo struct {
	// BuildPath 前端构建产物目录，默认 ./cmd/iotserver-ui/build
	BuildPath string
}

// WebBuildPath 返回前端静态资源目录。
func (c ConfigurationStruct) WebBuildPath() string {
	if path := strings.TrimSpace(c.WebUI.BuildPath); path != "" {
		return path
	}
	return "./cmd/iotserver-ui/build"
}

// PluginsInfo 原生插件运行时的目录配置。
type PluginsInfo struct {
	// Root 插件包解压后的安装根目录，默认 data/plugins
	Root string
	// DataDir 插件实例的配置、日志、PID 文件目录，默认 data/plugin-data
	DataDir string
}

// PluginRoot 返回插件安装根目录。
func (c ConfigurationStruct) PluginRoot() string {
	if root := strings.TrimSpace(c.Plugins.Root); root != "" {
		return root
	}
	return "data/plugins"
}

// PluginDataDir 返回插件实例数据目录。
func (c ConfigurationStruct) PluginDataDir() string {
	if dir := strings.TrimSpace(c.Plugins.DataDir); dir != "" {
		return dir
	}
	return "data/plugin-data"
}

// IntegrationsInfo 外部系统接入配置。
type IntegrationsInfo struct {
	// EdgeIngest 参考系统（/edge 协议）接入所用的独立 broker 配置
	EdgeIngest EdgeIngestInfo
}

// EdgeIngestInfo 参考系统（iotddc / things，/edge 协议）接入的 broker 配置。
//
// 为什么必须与 MessageQueue 分开：MessageQueue 同时承担平台的**内部消息总线**
// （PublishTopicPrefix = eventbus/in，规则引擎/事件链路都走它），而参考系统的上行
// 数据在**另一个 broker** 上。直接改 MessageQueue.Host 会把这套内部总线也搬到对方去。
//
// Broker 留空时回落到 MessageQueue，保证既有部署行为不变。
type EdgeIngestInfo struct {
	// Broker 参考系统 broker 地址，形如 tcp://host:1885
	Broker string
	// ControlBroker 参考系统的控制通道 broker（restart / 标签树 / 属性写）。
	// 实测 things 的 1885 只桥接了上行与属性写，restart 与标签树在 1883；
	// 留空则回落到 Broker。
	ControlBroker string
	// Username / Password 接入该 broker 的凭据（对方允许匿名访问时可留空）
	Username string
	Password string
	// ResyncIntervalSec 订阅刷新间隔（秒）。网关在界面上增删后，据此自动重订阅，
	// 无需重启服务。<=0 时用默认值。
	ResyncIntervalSec int
}

// PprofInfo 调试端点配置。
// 默认关闭：/debug/pprof 会暴露 goroutine 栈、堆内存（可能含令牌/密码）与可消耗 CPU 的 profile 接口。
// 开启后仍需 x-token 鉴权并具备 system:debug:pprof 权限（超级管理员直通）。
type PprofInfo struct {
	Enabled bool
}

// TenantInfo 多租户隔离配置
type TenantInfo struct {
	// Enforce 无租户上下文的调用是否 fail-closed。
	//   false（默认，迁移期）：不打租户条件，仅打告警日志（日志级别可控制）—— 未改造完的调用点仍可工作；
	//   true（上线/测试）：租户表查询在缺少租户上下文时返回空集，写入直接报错。
	// 建议在业务接口全部完成 WithCtx 改造、隔离测试跑通后再置 true。
	Enforce bool
}

// 部署形态。
const (
	DeploymentModeServer   = "server"
	DeploymentModeEmbedded = "embedded"
)

// DeploymentInfo 部署形态与插件运行方式。
type DeploymentInfo struct {
	// Mode server：保留 Docker 驱动；embedded：插件以本机进程运行，隐藏镜像仓库。
	// 为空时按 server 处理，兼容既有部署。
	Mode string
}

// DeploymentMode 返回规范化后的部署形态。
func (c ConfigurationStruct) DeploymentMode() string {
	mode := strings.ToLower(strings.TrimSpace(c.Deployment.Mode))
	if mode == DeploymentModeEmbedded {
		return DeploymentModeEmbedded
	}
	return DeploymentModeServer
}

type WritableInfo struct {
	LogLevel        string
	LogPath         string
	InsecureSecrets bootstrapConfig.InsecureSecrets
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

// URL constructs a URL from the protocol, host and port and returns that as a string.
func (m MessageQueueInfo) URL() string {
	return fmt.Sprintf("%s://%s:%v", m.Protocol, m.Host, m.Port)
}

// BrokerURL 参考系统接入实际使用的 broker 地址：未单独配置时回落到 MessageQueue。
func (e EdgeIngestInfo) BrokerURL(m MessageQueueInfo) string {
	if b := strings.TrimSpace(e.Broker); b != "" {
		return b
	}
	return m.URL()
}

// ControlBrokerURL 参考系统控制通道实际使用的 broker 地址：未单独配置时回落到 Broker。
func (e EdgeIngestInfo) ControlBrokerURL(m MessageQueueInfo) string {
	if b := strings.TrimSpace(e.ControlBroker); b != "" {
		return b
	}
	return e.BrokerURL(m)
}

// Credentials 参考系统接入使用的账号密码：单独配置优先，否则用 MessageQueue 里的可选凭据。
func (e EdgeIngestInfo) Credentials(m MessageQueueInfo) (username, password string) {
	username, password = strings.TrimSpace(e.Username), e.Password
	if username == "" && password == "" {
		return m.Optional["Username"], m.Optional["Password"]
	}
	return username, password
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

// GetDatabaseInfo 返回元数据库配置。
func (c *ConfigurationStruct) GetDatabaseInfo() bootstrapConfig.Database {
	return c.Databases["Metadata"]
}

// GetDataDatabaseInfo 返回时序数据库配置。
func (c *ConfigurationStruct) GetDataDatabaseInfo() bootstrapConfig.Database {
	return c.Databases["Data"]
}

// IotpConfig 返回 iotp 连接参数；BaseURL 为空表示未接入 iotp。
//
// iotp 作为时序数据库的一种，连接参数统一放在 [Databases.Data]：
// BaseURL / Token / Name（库名）/ Timeout（毫秒）。Type='iotp' 时数据写入 iotp；
// 未选择 iotp 但配置了 BaseURL 时，仅启用元数据同步与实时值读取。
func (c *ConfigurationStruct) IotpConfig() (bootstrapConfig.Database, bool) {
	db := c.GetDataDatabaseInfo()
	return db, strings.TrimSpace(db.BaseURL) != ""
}

// GetInsecureSecrets returns the service's InsecureSecrets.
func (c *ConfigurationStruct) GetInsecureSecrets() bootstrapConfig.InsecureSecrets {
	return c.Writable.InsecureSecrets
}
