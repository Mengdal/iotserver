# DuckDB 数据层迁移与遗留问题处置方案

> 背景：时序数据层从 DuckLake（DuckDB + Parquet 湖仓）切换为纯 DuckDB（单文件列存），
> 随后收敛掉多余后端并打通多租户读写链路。
>
> 本文记录已落地的改动、原有时序库问题的最终处置，以及仍然待办的事项。

## 一、已落地改动

### 1.1 新增 DuckDB 数据层（单文件列存）

| 文件 | 说明 |
| --- | --- |
| `internal/tools/datadb/duckdb/client.go` | 连接、三张表、写入、三类查询、计数、表结构管理 |
| `internal/tools/datadb/duckdb/batch.go` | 攒批落盘、保留策略、`PurgeBefore` / `PurgeTenantBefore` |
| `internal/tools/datadb/duckdb/aggregate.go` | SQL 分桶聚合 |
| `internal/tools/datadb/duckdb/helpers.go` | 值拆分、载荷序列化、时间窗、分页归一化 |
| `internal/tools/datadb/duckdb/client_test.go` | 15 个用例（含并发写入、租户捕获、分级保留、事件时间不串号） |

默认参数：`flushRows=5000`、`flushInterval=2s`、`retentionDays=30`、`maxOpenConns=8`、`maintenanceInterval=1h`。

### 1.2 后端收敛（删除已停用的三个时序后端）

| 动作 | 对象 |
| --- | --- |
| 整包删除 | `internal/tools/datadb/tstorage/`、`internal/tools/datadb/tdengine/` |
| 部分删除 | `internal/tools/datadb/leveldb/` 只保留 `driverstorage.go`（驱动 KV 存储仍在用），删除时序实现 |
| 删除 | `internal/tools/datadb/timeaware.go`（`TimeAwareInserter`，仅 tstorage/tdengine 使用） |
| 依赖清理 | `go.mod` 移除 `nakabonne/tstorage`、`taosdata/driver-go/v3`（`goleveldb` 仍被驱动存储使用） |
| 代码清理 | `persistence/thinkmodel.go` 从 1244 行降到约 120 行，删掉 21 个后端专属函数与全部 `GetDataDBType()` 分发 |

> 说明：`leveldb` 包名保留（内含驱动 KV 存储），未整体删除。

### 1.3 多租户读写闭环

| 项 | 改动 |
| --- | --- |
| 写路径 | 时序行新增 `tenant_id` 列；上行消息不带租户时，由**设备归属**推导后注入 ctx（`thinkmodel_lake.go` 的 `resolveTenant`），带本地 TTL 缓存避免每条消息查库 |
| 读路径 | 查询接口统一加 `context.Context`，持久化层不再用 `tenant.Platform(...)` 绕过，改用请求自身 ctx —— 仓储层的 GORM 租户插件据此自动追加 `tenant_id` 条件，跨租户访问设备直接查不到 |
| 计数路径 | `SearchDeviceMsgCount` 同样改用请求 ctx，租户作用域只统计本租户设备（原先硬编码平台作用域会跨租户） |
| 接口改动 | `PersistDeviceItf` 6 个方法全部加 ctx；`DeviceInvokeThingService` 加 ctx（仅 1 个调用方） |
| 调用点 | 13 处调用方更新，控制器统一传 `c.Request.Context()`（viz 链路传其派生的超时 ctx） |

**租户作用域行为对照**（由 `pkg/middleware` 的 `TenantContext` 决定）：

| 请求身份 | 插件行为 |
| --- | --- |
| 普通用户（token 带 tenantId） | 过滤 `tenant_id = ?` |
| 平台管理员 + `x-tenant-id` 头 | 过滤为该租户 |
| 平台管理员（无头） | `Platform=true`，不过滤（跨租户运营） |
| 无租户上下文的后台调用 | 受 `[Tenant] Enforce` 策略控制：当前 `false`（Warn）= **不过滤仅告警**，`true` = fail-closed |

> 注意：`Enforce=false` 只影响「完全没有租户上下文」的调用；带租户的请求在任何策略下都会真实过滤。

### 1.4 验证结果

- `go build ./...` 通过；改动文件 `gofmt` 干净。
- `go test ./internal/...` 全量通过。
- `datadb/duckdb` 单包 5.2s（原 ducklake 为 110s 级，差异来自不再建湖与加载扩展）。

## 二、原问题最终处置

### A. 随选型变更 / 后端下线而消失

分区设计与 `N` 调参、`CREATE TABLE ... PARTITIONED BY` 语法坑、`month(ts_ms)` 哑雷、
1 事务 1 文件（实测 576k 文件/天）、稳态 12,400 文件、三段式压实、2s→60s 持久性取舍、
catalog 快照膨胀、ducklake 扩展离线预置与 `linux_amd64` 缺失、catalog 格式文档误导。

**后端下线后一并消失的（此前是「待下线才消失」，现已确认删除）**：
TDengine nil 解引用 panic、TDengine 消息总数恒 0（`return 0, nil`）、TDengine 计数 SQL 表名与时间单位错、
TDengine 占位符被引号包住、tstorage 最新值越界 panic、tstorage 丢类型、
leveldb 时间区间丢边界点、leveldb 计数函数恒返回 1。

### B. 实现过程中修掉的缺陷（均有测试锁定）

| 缺陷 | 处置 |
| --- | --- |
| 计数方法未先落盘 → 数字少计 | 计数前统一 `flushPending`。**ducklake 存在同样缺陷**（三个计数方法均无 flush 调用） |
| `DropTable` / `DelDatabaseField` 未先落盘 → 缓冲数据在删除后被写回，删除无效 | 先落盘再删。**ducklake 存在同样缺陷** |
| `insertEvents` 时间戳串号（同批第二个无时间事件继承前一个的时间） | 每条事件独立取时间 |
| 高并发连接数无上限 | `SetMaxOpenConns(8)` |
| 删除操作不回收空间 | `DELETE` 后 `CHECKPOINT` |
| 并发写事务 | 增加 `writeMu` 串行化写入。**实测 32 写者 × 16000 行零冲突零失败**，该锁属防御性设计，不是在修复已复现的丢数据问题 |

## 三、待办与处置进度

### 3.1 已全部解决

| 事项 | 处置 |
| --- | --- |
| **平台级计数 N+1** | 新增可选能力接口 `interfaces.MsgCounter`（沿用 `TimeAwareInserter` 的能力探测模式）：duckdb 走一条 SQL 的 `CountAllMsgByTimeRange`，未实现的形态（ducklake 无 `tenant_id`）退回按设备汇总——设备列表本身已被租户插件过滤，语义仍正确。`duckdb.Client` 有编译期断言保证实现该能力，测试用「未注入 dbClient 会 panic」反证走的是快路径 |
| **聚合查询重复计算** | `duckdb/aggregate.go` 改为只跑一次 `GROUP BY`（桶数已被 `aggregate.MaxBuckets` 限制在 2 万以内），复用 `aggregate.Paginate` 在内存分页；新增按桶翻页不重不漏的测试 |
| **flush 失败回灌无上限无退避** | 缓冲加上限 `defaultMaxBufferRows=200000`（超出返回 `errBufferFull`，调用方明确感知而不是静默涨到 OOM）；失败后按 2 的幂退避（上限 5 分钟），定时落盘在退避窗口内跳过（`flushIdle`），而查询前/关闭前的落盘不受退避限制（`flushPending` 强制）。测试用「删表制造持续失败 → 退避生效且缓冲保留 → 恢复后强制落盘、数据不丢」覆盖 |
| **`DropStable` 空实现** | 复核后确认**不是漏实现**：删除产品前应用层已强制校验无子设备，而时序数据按设备写入、子设备删除时已清理。真正的问题是反过来的顺序——`DeleteDeviceById` 先删元数据、再用 `_ =` 丢弃时序清理错误，一旦失败这些行就再也无法归属（只能等保留期），属静默泄漏。已改为**先清时序、后删元数据**并传播错误 |
| **设备维度裁剪未验证** | 已用 10 万行实测，结论见 3.3：保持不建索引 |
| **分级保留未生效** | 已实现：`tenant` 表新增 `retention_days` 列（AutoMigrate 自动加列），`>0` 覆盖全局、`=0` 用全局配置、`<0` 永久保留；应用层每小时任务（`application/retention.go`）遍历租户、按各自保留期调用 `PurgeTenantBefore`。全局配置**每次运行重新读取**，改配置不再需要重启；数据层不再自带定时清理（单一 owner，避免重复 DELETE）。`RunRetention` 依赖 `DataPurger` 可选能力，ducklake 未实现该能力因而安全跳过、沿用自带维护。测试覆盖天数换算（含覆盖/默认/永久/兜底）与单租户失败不中断 |
| **`IsAll` 静默截断** | 按决策改为**聚合路径不截断**：聚合结果直接返回全部桶（上限由 `MaxBuckets` 兜底），原始数据路径仍保留 `maxPageSize`。新增「桶数超过 `maxPageSize` 时 IsAll 全量返回」的测试 |
| **备份流程缺失** | 已实现在线导出：`duckdb.Client.ExportDatabase` 先 `CHECKPOINT` 再 `EXPORT DATABASE (FORMAT PARQUET)`，导出一致快照（schema.sql + 各表 Parquet）；新增 `POST /api/v1/system/backup`（平台管理员 + `system:config:edit`，支持 `dir`/`keep` 参数自动清理旧备份）。测试验证导出目录内容完整、导出后可继续写入、且快照能被 `IMPORT DATABASE` 读回（恢复可行性） |

### 3.2 新发现（本轮衍生）

| 事项 | 说明 |
| --- | --- |
| `community` 构建标签已损坏 | `go build -tags community ./...` 报 `undefined: application.InitSchedule`：`schedule.go` 带 `!community` 标签，而 `init.go` 无条件调用它，且没有 community 版替代实现。属**既有问题**（与本次改动无关），但会让 community 构建完全无法编译 |
| 网关未绑定设备的「record not found」日志 | 网关上报是两步流程（先按 `gateway_id + device_key` 查、未绑定时再按 `device_sn` 兜底），第一步查不到属正常分支；但 `deviceByGatewayAndKey` 用 `First` 查询，GORM 默认 logger 会把 `ErrRecordNotFound` 打成 error 级日志，于是未绑定网关的设备**每上报一次刷一条看起来像报错的日志**。已改为 `Find + Limit(1)`（查空不报错，返回的领域错误不变），sqlite 与 mysql 两份同步修改，并有回归测试锁定语义。注意：当前没有把设备绑定到工业网关的公开接口（`BatchBindDevice` 绑的是驱动实例），所以经 SN 兜底写入的设备必然走这条分支 |
| 网关上报的设备永远显示「离线」 | 设备创建时初始状态写死为 `DeviceStatusOffline`（`deviceapp.AddDevice`），而**全平台只有云设备通知路径**（`deviceapp/cloudability.go`）会调用 `DeviceOnlineById`；网关上行只更新了网关状态（`handleData` 里的 `GatewayOnline`），完全没更新设备，于是经网关上报的设备无论有多少数据都显示离线。已在 `gatewayingest.ingestRecord` 落库成功后补上「置设备在线」，并加 30 秒节流（同一设备最多每 30 秒写一次元数据库，避免高频上报写爆）。新增测试锁定状态与节流行为。
| 在线状态只有「置在线」没有「置离线」 | 上一条只补了「置在线」，状态会永久停在在线。已补上**超时扫描**：新增配置项 `data.offlineTimeout`（分钟，默认 5，页面可改、每次扫描重新读取），每分钟一次把「最后活跃时间早于 now-超时」的在线设备与网关置为离线；设备与网关**共用同一个超时值**，避免两套语义。设备依据 `last_online_time`，网关依据 `last_online_at`（数据上报与状态心跳两条路都会刷新它）。`device.status` 增加索引，避免每分钟全表扫描。测试覆盖：只动「在线且超时」的行（新鲜/已离线/从未上线/非在线状态都不动）、重复扫描幂等、cutoff 非法直接返回，以及「上报→在线→超时→离线」完整闭环 |
| 备份端点需要重新构建并重启服务才生效 | 当前运行中的 core 是改动前的二进制；`POST /api/v1/system/backup` 与按租户保留策略都要等下次发布后才可用 |

### 3.3 设备维度裁剪实测（10 万行 / 30 天）

| 场景 | 无索引 | 已建索引 |
| --- | --- | --- |
| 单设备 + 窄窗（1 小时） | 504 µs | 464 µs |
| 单设备 + 宽窗（30 天） | 2.87 ms | 2.95 ms |
| 换一台设备、同一窄窗 | 502 µs | 525 µs |
| 写入吞吐 | 100000 行 / 3.07s（≈3.3 万行/秒） | 建索引后 ≈3.1 万行/秒（无明显下降） |

结论：

- **时间裁剪有效**：窄窗比宽窗快 5.7 倍，与时间跨度成正比 —— 按时间递增追加让 `ts_ms` 的 zone map 保持狭窄。
- **设备维度确实没有裁剪**：换设备查询耗时几乎不变，说明扫描量由时间范围决定，设备过滤发生在扫描之后（row group 的 `device_id` min/max 横跨全部设备，符合预期）。
- **索引没有价值**：收益 8%，在噪声范围内；建索引也不明显拖慢写入。因此**保持默认不建索引**，`Optimize()` 继续作为运维可选项。
- 若将来单设备长区间查询成为瓶颈，正确手段是**按 `(device_id, ts_ms)` 物理聚簇**（百万行量级实测 534 µs → 384 µs），而不是加索引。

## 四、架构层面（换选型不解决）

- **单写者 / 无 HA**：DuckDB 与 DuckLake 一致，同一个 `.db` 文件同一时刻只允许一个进程打开，
  core 无法水平扩展，需要单实例 + 主备切换或定期快照来兜。
- **2 秒丢数据窗口**（已确认接受）：写入先入内存缓冲，默认 2 秒 / 5000 行落盘，
  进程被强杀会丢失窗口内数据；`CloseSession` 会落盘，正常退出不丢。
  如需取消该窗口，需引入前置 WAL 兜底。
