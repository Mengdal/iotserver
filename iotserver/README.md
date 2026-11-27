# iotserver 后端服务

iotserver 的 Go 后端，负责设备 / 网关接入、物模型、告警、规则引擎、可视化看板、多租户与系统管理。项目整体介绍、功能清单和页面截图见[仓库根目录 README](../README.md)。

## 本地开发

前置：Go 1.24+、Node 18+。

```bash
# 构建前端并同步到 cmd/iotserver-ui/build
make ui

# 直接启动（启动前会自动检查前端产物是否最新）
make start

# 运行全部后端测试
make test
```

默认配置见 `cmd/iotserver-core/res/configuration.toml`，常用项：

| 配置项 | 说明 |
| --- | --- |
| `Databases.Data.Type` | 数据层：`duckdb`（默认）或 `iotp` |
| `Databases.Data.DataSource` | DuckDB 数据库文件路径（`iotp` 模式下不生效） |
| `Databases.Data.BaseURL` / `Token` | iotp 地址与认证；`Type=iotp` 时 BaseURL 必填 |
| `Databases.Data.Name` / `Timeout` | iotp 库名（缺省 `iotserver`）/ 请求超时（毫秒） |
| `Service.CallbackUrl` | eKuiper 规则动作回调基地址 |
| `Service.CallbackToken` | 回调共享密钥；非空时回调必须带 `x-callback-token` |
| `Pprof.Enabled` | 调试端点 `/debug/pprof`，默认关闭；同时控制 Swagger UI（开启后 `/api/v1/swagger/index.html` 可访问） |

> 规则编辑器的「调试规则」调用 eKuiper 试运行接口 `/ruletest`，SSE 端口使用该接口返回的 `port`（默认 `10081`）。
> Docker Compose 已把 `127.0.0.1:10081` 映射到 eKuiper；本地直接运行 eKuiper 时需保证该端口可访问。
> 调试 SSE 会显式清除 HTTP 服务的写超时，长连接不会在 45s/120s 被截断；客户端断开后自动删除试运行规则。
> eKuiper 试运行只把结果写到 SSE，调试面板会在展示输出的同时，按当前规则的应用配置真实推送
> （支持 HTTP 推送和 MQTT；Kafka/InfluxDB 仅在正式运行时执行）。
> 试运行输出使用 `sendSingle` 语义逐条返回，数据模板按单条记录渲染，与正式 sink 的发送方式一致。

### 规则引擎

高级能力中的规则引擎使用「规则 ID + 原始 SQL + 一个或多个数据资源动作 + 规则选项」模型：

- 规则 ID 可由用户指定（字母、数字、下划线、中划线，最多 64 位），留空时由服务端生成；
- 一条规则可绑定多个数据资源，启动时按最新资源配置重建 eKuiper actions；
- SQL 支持 `mqtt_stream` 上的过滤、计算和窗口聚合；`JOIN`/`UNION`、注释、多语句和多数据源会被拒绝；
- 租户规则保存时由后端注入 `tenantId` 条件，前端编辑器显示原始 SQL，不显示注入后的条件；
- 规则选项（并发、缓存、QoS、检查点、事件时间、重试次数等）透传到 eKuiper；
- 「调试规则」使用 eKuiper 试运行接口和 SSE 将规则输出实时返回；同时按当前应用配置真实推送 HTTP/MQTT；
- 规则列表支持按 ID/名称/标签搜索、批量启停和 JSON 导入导出。

### 时序数据后端：DuckDB / iotp

默认写入本地 DuckDB 单文件。iotp 也是 `[Databases.Data]` 的一种类型：把 `Type` 改为
`iotp`，并在同一段配置 `BaseURL` / `Token` / `Name`（库名）/ `Timeout`（毫秒），即可把时序数据
写入外部 iotp；重启生效。只填 `BaseURL`（Type 仍为 duckdb）时会启用 iotp 元数据同步与实时值读取，
但不写入数据。切换不迁移既有 DuckDB 数据（文件原样保留）。所用令牌必须同时具备元数据与
`/write` 写权限，否则服务启动正常但数据写入会被 iotp 以 403 拒绝。

iotp 是「一个产品一张宽表」，适配层负责宽窄转换：有产品的设备写 `<产品ID>`，
无产品设备（网关子设备）写兜底表 `default`；字段按物模型/首包惰性创建，设备写入前自动注册。
iotp 模式下的能力边界：

- 不支持按设备/产品删除历史，也不执行租户保留期清理（由 iotp 侧自行管理）；
- 不支持在线导出/备份（接口返回「当前数据层不支持在线导出」）；
- 消息数按宽表行统计，与 DuckDB 按测点行统计的数值会有差异；
- 历史聚合受 iotp 查询白名单限制（不支持分桶表达式），适配层取原始行后在本地分桶，
  单次最多取 10 万行，超密数据的大时间窗结果可能不完整（日志会告警）；
- 物模型字段类型变更在 iotp 已有数据时会被拒绝，适配层记录日志并保留旧类型。

消息总线上的 `PROPERTY_REPORT` 统一为扁平结构：`data.<code> = value`，并带顶层
`reportTime`（毫秒，用于告警历史回跳）。驱动/网关的 `{value,time}` 报文中转时会压平，
iotp 采集器直接转发 ingest `fields`，两种来源对规则作者保持一致。

软网关（iotpGW 等）设备的场景联动「执行动作」通过 iotp WS 写控制下发：
平台先调用 `POST /api/v1/devices/route` 找到网关所在节点，再连接节点返回的 `ws_url`
发送 `{"type":"write","id":"<device_id>.<field>","val":"..."}`。要求设备在 iotp 有
`device_gateway` 归属、对应采集点位为 `W`/`RW`，且节点 `[realtime]` 已启用；同一网关
不允许同时连多个节点。直连驱动设备仍走驱动 RPC，edge/things 网关仍走 EdgeCtl，行为不变。

## Docker 部署

```bash
make ui
make docker PUSH=0          # 同时构建核心镜像和带 Kafka/InfluxDB sink 插件的 eKuiper 镜像
cd manifest/docker
docker compose up -d

# 推送镜像到自己的仓库：
# make docker IMAGE=<registry>/iotserver-core:v1.0 EKUIPER_IMAGE=<registry>/iotserver-ekuiper:2.4.2
```

说明：

- eKuiper 镜像名为 `iotserver-ekuiper:2.4.2`，由 `manifest/docker/ekuiper/Dockerfile` 在构建阶段从 eKuiper v2.4.2 源码编译 Kafka / InfluxDB sink 插件（官方 2.4.2 预编译插件下载返回 403）。
- 首次构建需要访问 GitHub 拉取 eKuiper 源码并下载 Go 依赖；离线部署需提前构建并推送镜像。
- `docker-compose.yml` 已开启 `KUIPER__BASIC__ENABLEPRIVATENET=true`，允许 HTTP 数据资源推送到内网地址；如只允许公网可改回 `false`。
- Kafka / InfluxDB 仅安装 sink 插件，实际推送仍需要可访问的 Kafka broker / InfluxDB 服务。
- 系统组件日志路径按部署形态自动选择：server（Docker）读容器内 `/var/bin/iotserver` 下由 compose 共享的 Mosquitto / eKuiper 日志目录；embedded 读本机 `/var/log/iotserver`、`/opt/iotserver`。默认路径只有文件存在时才显示日志入口；也可用 `[ComponentLogs]` 显式覆盖。
- iotpGW 不随 Docker 编排部署；如果引擎在宿主机运行并写到 `/var/log/iotserver/iotpgw.log`，compose 已把该目录只读挂载给 core，server 模式会自动出现日志入口。
- Mosquitto 日志由 `logrotate` 服务按 50MB 轮转、保留 5 份；轮转后会自动重启 `mqtt-broker` 以重新打开日志文件。首次 `docker compose up -d` 会构建该小镜像。

## 目录

```text
cmd/                    # 服务入口、配置与前端静态产物
internal/
├── iotserver/core/     # 接入、应用层、控制器与 e2e 测试
├── models/             # 数据模型
├── pkg/                # 种子数据、权限、国际化等公共能力
└── tools/              # 告警通知、协议解析等工具
manifest/               # Docker Compose、初始化 SQL
docs/                   # 网关接入、多租户、数据层等设计文档
tools/                  # 数据迁移、演示数据与权限生成等运维工具
```

## 文档

- [网关接入规范](docs/gateway-integration.md)
- [设备接入设计](docs/device-access-design.md)
- [多租户隔离审计](docs/multi-tenant-audit.md)
- [时序数据层迁移说明](docs/duckdb-migration-plan.md)
- [数据资源推送（HTTP/MQTT/Kafka/InfluxDB）](docs/data-resource-sinks.md)
- [审计日志方案](docs/audit-log-plan.md)

## 开源协议

[Apache License 2.0](../LICENSE)
