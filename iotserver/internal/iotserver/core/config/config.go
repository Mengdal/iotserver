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

var (
	DefaultEnv = "pro"
)

// Struct used to parse the JSON configuration file
type ConfigurationStruct struct {
	Writable     WritableInfo
	MessageQueue MessageQueueInfo
	Clients      map[string]bootstrapConfig.ClientInfo
	Databases    map[string]map[string]bootstrapConfig.Database
	Registry     bootstrapConfig.RegistryInfo
	Service      bootstrapConfig.ServiceInfo
	RpcServer    bootstrapConfig.RPCServiceInfo
	SecretStore  bootstrapConfig.SecretStoreInfo
	WebServer    bootstrapConfig.ServiceInfo
	DockerManage DockerManage
	Pprof        PprofInfo
	Tenant       TenantInfo
	// EdgeIngest 参考系统（/edge 协议）接入所用的独立 broker 配置
	EdgeIngest EdgeIngestInfo
	// Iotp iotp 时序库的元数据同步配置（产品/物模型 → 库表字段，设备 → 设备实例）
	Iotp IotpInfo
	// CollectEngine 采集引擎（IOTP Gateway API）配置：通道/设备/点位下发
	CollectEngine CollectEngineInfo
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

// IotpInfo iotp 时序库的接入配置。
//
// 既用于「产品/物模型 → 库表字段」「设备 → 设备实例」的元数据同步（iotpsync），
// 也在 [Databases.Data.Primary].Type = "iotp" 时作为时序数据后端。
// Enabled=false 时全部跳过，平台行为与未接入 iotp 完全一致。
type IotpInfo struct {
	Enabled bool
	// BaseURL iotp HTTP 地址，形如 http://host:8080
	BaseURL string
	// Token 访问令牌（Authorization: Bearer）
	Token string
	// Database 平台数据所在库；留空用 iotserver
	Database string
	// TimeoutSec 单次 HTTP 请求超时（秒）；<=0 用 10 秒
	TimeoutSec int
}

// CollectEngineInfo 采集引擎（IOTP Gateway API）配置。
//
// 采集配置的「下发」把平台里的通道 / 设备 / 点位同步到采集引擎；
// Enabled=false 或 BaseURL 为空时下发直接报错，保存到平台不受影响。
type CollectEngineInfo struct {
	Enabled bool
	// BaseURL 采集引擎地址，形如 http://host:8081
	BaseURL string
	// TimeoutSec 单次 HTTP 请求超时（秒）；<=0 用 10 秒
	TimeoutSec int
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

// GetInsecureSecrets returns the service's InsecureSecrets.
func (c *ConfigurationStruct) GetInsecureSecrets() bootstrapConfig.InsecureSecrets {
	return c.Writable.InsecureSecrets
}
