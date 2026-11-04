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
| `Integrations.Iotp.Enabled` / `BaseURL` / `Token` | iotp 地址与认证；`Type=iotp` 时必须启用 |
| `Integrations.Iotp.Database` | iotp 库名，缺省 `iotserver` |
| `Service.CallbackUrl` | eKuiper 规则动作回调基地址 |
| `Service.CallbackToken` | 回调共享密钥；非空时回调必须带 `x-callback-token` |
| `Pprof.Enabled` | 调试端点 `/debug/pprof`，默认关闭 |

### 时序数据后端：DuckDB / iotp

默认写入本地 DuckDB 单文件。把 `[Databases.Data].Type` 改为 `iotp`，并配置
`[Integrations.Iotp]`（`Enabled = true`、`BaseURL`、`Token`、`Database`），即可把时序数据写入外部
iotp 时序库；重启生效。切换不迁移既有 DuckDB 数据（文件原样保留）。所用令牌必须同时
具备元数据与 `/write` 写权限，否则服务启动正常但数据写入会被 iotp 以 403 拒绝。

iotp 是「一个产品一张宽表」，适配层负责宽窄转换：有产品的设备写 `p_<产品ID>`，
无产品设备（网关子设备）写兜底表 `p_default`；字段按物模型/首包惰性创建，设备写入前自动注册。
iotp 模式下的能力边界：

- 不支持按设备/产品删除历史，也不执行租户保留期清理（由 iotp 侧自行管理）；
- 不支持在线导出/备份（接口返回「当前数据层不支持在线导出」）；
- 消息数按宽表行统计，与 DuckDB 按测点行统计的数值会有差异；
- 历史聚合受 iotp 查询白名单限制（不支持分桶表达式），适配层取原始行后在本地分桶，
  单次最多取 10 万行，超密数据的大时间窗结果可能不完整（日志会告警）；
- 物模型字段类型变更在 iotp 已有数据时会被拒绝，适配层记录日志并保留旧类型。

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
