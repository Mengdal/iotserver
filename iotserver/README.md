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
| `Databases.Data.Primary.Type` | 数据层：`ducklake`（默认）/ `duckdb` / `leveldb` / `tstorage` / `tdengine` |
| `Databases.Data.Primary.DataSource` / `Dsn` | DuckLake 的 catalog（SQLite）与 Parquet 数据目录 |
| `Service.CallbackUrl` | eKuiper 规则动作回调基地址 |
| `Service.CallbackToken` | 回调共享密钥；非空时回调必须带 `x-callback-token` |
| `Pprof.Enabled` | 调试端点 `/debug/pprof`，默认关闭 |

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
tools/                  # lakectl、权限生成等运维工具
```

## 文档

- [网关接入规范](docs/gateway-integration.md)
- [设备接入设计](docs/device-access-design.md)
- [多租户隔离审计](docs/multi-tenant-audit.md)
- [时序数据层迁移说明](docs/duckdb-migration-plan.md)
- [审计日志方案](docs/audit-log-plan.md)

## 开源协议

[Apache License 2.0](../LICENSE)
