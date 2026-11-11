# IoTP 用户使用手册

> **适用版本：** v0.9.4（2026 年 11 月）  
> **适用对象：** IoTP 部署人员、网关集成工程师、应用开发人员及系统运维人员  
> **文档范围：** 单机与集群部署、数据模型、写入、查询、实时控制、配置、API 和运维。

IoTP 是以单个二进制程序交付的时序数据采集与查询服务，支持独立运行，也可通过 `control`（控制面）与 `node`（数据节点）组成集群。本文以实际操作为主线，保留完整的接口细节与运维说明。旧版本行为差异请查看项目仓库中的 `CHANGELOG.md`。

## 阅读指南

| 你的目标 | 建议阅读 | 主要内容 |
| --- | --- | --- |
| 第一次运行 IoTP | [快速上手](#快速上手) → [安装与启动](#2-安装与启动) | 启动、建模、写入、验证 |
| 理解设备与字段的关系 | [概念与数据模型](#1-概念与数据模型) | `db`、`table`、`device_id`、标签、属性 |
| 开发数据接口 | [元数据管理 API](#4-元数据管理-api) → [数据写入](#5-数据写入) → [查询](#6-查询) | HTTP、LP、JSON、MQTT、SQL |
| 接入实时画面和控制 | [状态与统计](#7-状态与统计) | 实时值、告警、聚合、WebSocket 写控制 |
| 部署到生产系统 | [配置参考](#3-配置参考) → [运维](#9-运维) | 认证、WAL、快照、合并、备份、集群 |
| 处理异常 | [已知限制与常见问题](#10-已知限制与常见问题) | 常见错误、边界条件和排查入口 |

**阅读约定**

- `代码样式` 表示配置键、请求路径、命令、字段名或返回值。
- 命令示例以 Linux Shell 和默认服务地址 `http://127.0.0.1:8080` 为基础；集群和认证部署须调整地址及凭据。
- **注意** 表示影响正常使用的前提；**警告** 表示可能导致数据丢失、权限暴露或服务中断的操作。
- 文中的“默认”与“缺省”含义相同，均指未显式配置时的取值。

## 目录

1. [快速上手](#快速上手)
2. [概念与数据模型](#1-概念与数据模型)
3. [安装与启动](#2-安装与启动)
4. [配置参考](#3-配置参考)
5. [元数据管理 API](#4-元数据管理-api)
6. [数据写入](#5-数据写入)
7. [查询](#6-查询)
8. [状态与统计](#7-状态与统计)
9. [前端页面](#8-前端页面)
10. [运维](#9-运维)
11. [已知限制与常见问题](#10-已知限制与常见问题)

---

## 快速上手

本节通过一个温度传感器示例，完成 **启动服务 → 建库建表 → 注册设备 → 写入数据 → 查询验证**。下面以默认单机配置为例，默认关闭 `[auth]`；若已启用认证，请为对应请求添加授权凭据（参见 [认证配置](#auth) 和 [用户管理](#46-用户管理主体与规则原访问控制)）。

### 操作前检查

- 已取得 IoTP 可执行文件，或具备 Rust 构建环境。
- 已将 `[data].dir` 修改为可写目录。
- 默认 HTTP 端口 `8080` 未被其他进程占用。
- 本例中的 `device_id=1` 仅作演示；**实际写入时应使用注册接口返回的 `id`**。

### 第一步：启动服务

```bash
# 1. 准备配置并启动（修改 [data].dir 为可写目录）
cp configs/iotp.toml.example my.toml
./target/release/iotp --config my.toml
# 健康检查：curl http://127.0.0.1:8080/health   → "ok"
```

### 第二步：创建库、表和设备，并写入数据

```bash
B=http://127.0.0.1:8080

# 2. 建库 → 建表 → 建设备（strict-only：不先声明，写入会被拒）
curl -X POST $B/api/v1/meta/databases -d '{"name":"demo"}'
curl -X POST $B/api/v1/meta/tables -d '{
  "db":"demo","table":"sensor",
  "fields":[{"name":"temp","type":"F64"},
            {"name":"site","type":"String","is_tag":true}]}'
curl -X POST $B/api/v1/meta/devices -d '{
  "db":"demo","table":"sensor","name":"s1","tags":{"site":"east"}}'
# → {"id": 1, ...}；1 就是写入行里的 device_id

# 3. 写入一行（HTTP 行协议）
curl -X POST "$B/write?db=demo" -d 'sensor,device_id=1 temp=23.5'

# 4. 立即查询（mode=all：内存缓冲 ∪ 已落盘）
curl -X POST $B/api/v1/query -d '{
  "sql":"SELECT * FROM demo.sensor ORDER BY time DESC LIMIT 10","mode":"all"}'

# 5. 看这台设备各字段的最新值
curl $B/api/v1/realtime/1
```

 > **查询模式注意：** `mode=history` 只查已落盘 parquet：新表要等第一次快照（缺省 `[flush] snapshot_interval = 10m`，可调短）之后才查得到；在那之前请用 `mode=all` 或实时值接口。
- **图形界面：** 控制台位于 `GET /`（与 API 同端口），默认打开「数据查询」页，可直接执行 SQL；页面结构见第 8 章。
- **继续阅读：** 写入方式与错误语义见第 5 章；SQL 与查询护栏见第 6 章；生产配置（认证、合并、保留期）见第 3、9 章。

---

## 1. 概念与数据模型

### 1.1 层级：db → table → 字段 → 设备实例

```text
数据库（db）
└── 数据表（table，即设备类型）
    ├── 测量字段（I64 / F64 / Bool / String）
    ├── 标签字段（String）
    └── 设备实例（device_id）
        ├── name / attrs
        └── tags（标签字段的取值）
```


- **db**：库，一组表的容器（如 `metrics`）。
- **table**：表 = "类"，声明字段集合（如 `cpu`）。
- **字段**：两种——**测量字段**（I64/F64/Bool/String，数据行的值列）与**标签字段**（恒 String，用于标识/分组，值是设备的静态属性）。
- **设备实例**：属于某一张表，有 `device_id`（INT64，devices 表 rowid，自动分配）、可选 `name`（同表唯一）、可选 `attrs`（JSON 对象，自由扩展）、以及一组标签字段值（可选，缺 = NULL）。
- **数据行**只携带 `device_id` 一个标签——其余标签组合在设备表中与 id 建立映射，查询时自动附加。

### 1.2 strict-only：一切先声明，再写入

表、字段、设备实例**只能经管理 API 创建**；写入路径严格按已定义的表/设备/字段校验，无任何"首写自动建表/字段/设备"。未知表、未知设备、未知字段名 → 该行拒收。

### 1.3 无损转换（逐值判定）

推送值的类型与模式类型不一致时，逐值判定能否**无损转换**到模式类型：

- 能 → 接受并转换存储（如 U64 ≤ i64::MAX → I64）；
- 不能 → **拒收该行**（如 F64 0.5 → I64），其余行照常写入（部分接受）。

### 1.4 模式演化

以下操作对已经写入的数据有不同限制。实施模式变更前，建议先确认表中是否存在内存缓冲或 Parquet 历史数据。


- 新增字段：随时允许（既有数据该列为 NULL）；
- 修改字段类型：**已有数据后拒绝**（409）；
- 改名：**标签字段随时**（tag_field_id 引用不变）；**测量字段仅限表里还没有数据时**（判据同改类型：buffer ∪ parquet，有数据 → 409——旧物理列会立刻不可见、合并重写时被丢弃）；测量字段改名后**旧名不落墓碑**，可以复用；⚠️ 集群模式下判据只看**执行改名的那个节点**的本机数据——控制面没有该表分片时无法确认其他节点是否已有数据，跨节点改名前请逐节点确认该表为空；
- 删除测量字段：允许（墓碑机制：名字不可复用，查询投影丢弃，物理列由合并重写时删除——合并由服务按 `[merge] schedule` 自己跑，见 §9.9）；
- 删除标签字段：剩余组合重复则 409。

---

## 2. 安装与启动

```bash
# 构建（目标平台 amd64 / ARM64）
cargo build --release            # 产物 target/release/iotp，单二进制 ~25MB

# 启动（配置文件路径 --config，缺省 configs/iotp.toml）
./target/release/iotp --config /etc/iotp/iotp.toml
```

启动流程：加载 SQLite 元数据 → 回放 WAL → 注册 parquet 表 → 起后台快照调度 → 起后台合并调度 → 监听 HTTP →（可选）启动内置 MQTT broker。**合并由服务自己跑**（0.6.0 起：进程内专用 `iotp-merge` 池 + `[merge] schedule` 墙钟 cron，见 §9.9）。健康检查：

```bash
curl http://127.0.0.1:8080/health      # 200 → "ok"
```

**日志**：默认只输出 ERROR 级（**多组有界例外**，见 §9.3），建议运维启动时设 `RUST_LOG=info`（快照周期、关闭序列等才有输出）。
日志缺省写到 `{data.dir}/logs/`（按天轮转、保留 7 天）：

```bash
RUST_LOG=info ./target/release/iotp --config /etc/iotp/iotp.toml
```

---

## 3. 配置参考

完整配置示例见 `configs/iotp.toml.example`。本章按功能模块组织配置项；**配置文件变更需要重启服务才能生效**。

| 配置段 | 作用 | 重点关注 |
| --- | --- | --- |
| `[server]` / `[node]` / `[cluster]` | HTTP 服务、节点身份及集群角色 | 监听地址、节点 ID、共享密钥 |
| `[data]` / `[wal]` / `[flush]` | 数据目录、写前日志、快照 | 持久化路径、同步间隔、内存上限 |
| `[query]` / `[ingest]` | 查询资源与采集线程 | 并发限制、超时、线程池 |
| `[auth]` / `[mqtt]` | 认证、授权与消息接入 | 生产认证、主题 ACL、TLS |
| `[merge]` | Parquet 合并与保留期清理 | 定时调度、保留期、删除风险 |
| `[alerts]` / `[rollup]` | 告警与窗口聚合 | 引擎开关、规则在元数据库中维护 |
| `[realtime]` | WebSocket 实时订阅及写控制 | 独立端口、权限、超时 |

> **生产部署提醒：** `[auth] enabled` 默认是 `false`，这意味着相关端点默认不启用身份校验。对外部署前应显式启用认证并规划 TLS、ACL 与网络访问控制。


### `[server]`

| 键 | 缺省 | 说明 |
|---|---|---|
| `host` | `0.0.0.0` | HTTP 监听地址 |
| `port` | `8080` | HTTP 端口（同端口服务前端页面） |
| `max_body_bytes` | `10485760` | `/write`、元数据 API 与 `/api/v1/query` 的请求体上限 |
| `log_dir` | `{data.dir}/logs` | 日志目录；显式空串 `""` = 只写 stdout（容器场景） |
| `log_retention_days` | `7` | 日志按天轮转、保留天数；`0` = 不限制 |
| `tls_cert` / `tls_key` | — | 服务端证书与私钥（**必须成对**）。配置后该端口为 HTTPS |
| `tls_ca` | — | 客户端证书的 CA。配置后**接受**由它签发的客户端证书，**证书 CN 即访问主体名**；不强制（未带证书的请求由应用层判 401，或改用 token）。**必须与 `tls_cert`/`tls_key` 同时配置**，单独配 `tls_ca` 会拒绝启动 |

### `[node]`

| 键 | 缺省 | 说明 |
|---|---|---|
| `id` | `n01` | 节点标识，同时是 parquet 文件名前缀与**集群节点身份**（归属集、复制、扇出都按它认节点）。`_` 分段不得为 `iotp3w` 或 `iotp3p<数字>`（那是 v3 文件名的标记段，会与覆盖解析冲突，启动即拒） |

### `[cluster]`（可选，整段缺省 = 独立模式）

集群拓扑（2026-10-05 起）。**配了 `role` 却缺必填项 = 拒绝启动**（拓扑声明错误比带病运行好）；运维流程见 §9.10–§9.13。

| 键 | 缺省 | 说明 |
|---|---|---|
| `role` | — | `"control"`（控制面，元数据唯一写者）或 `"node"`（分片节点）。不写 / 整段缺省 = 独立模式 |
| `secret` | — | 部署级共享密钥，**所有节点一致、至少 16 字符**（建议 32+）；内部端点 `/internal/*` 的鉴权凭据 |
| `control` | — | **仅 `role = "node"` 必填**：控制面地址，必须 `http://` 开头（内部拉取为 v1 明文） |
| `addr` | — | **声明 `role` 时必填**：本节点对其他节点可达的地址，必须 `http://` 开头；改地址要同步改这里并重启 |

- `role = "control"` 与 `[server]` 任一 TLS 项（`tls_cert`/`tls_key`/`tls_ca`）同时出现 ⇒ **拒绝启动**（内部调用只有明文，TLS 会让节点永远连不上）。

### `[data]`

| 键 | 缺省 | 说明 |
|---|---|---|
| `dir` | `/var/lib/iotp` | **唯一数据根目录**。派生布局不可单独配置：`{dir}/wal/`（WAL）、`{dir}/meta/`（last_snapshot.json 等）、`{dir}/data/{db}/{table}/`（快照 parquet）、`{dir}/spill/`（查询溢写）、`{dir}/logs/`（日志，受 `[server].log_dir` 控制）、`{dir}/iotp.db`（元数据 SQLite） |

### `[wal]`

| 键 | 缺省 | 说明 |
|---|---|---|
| `flush_interval_secs` | `1` | WAL 文件 fsync 周期（group commit：写请求只入 pending，每周期合成一个文件统一 fsync；进程崩溃最多丢失本周期内已确认的写入） |
| `max_write_buffer_ops` | `100000` | 写缓冲 op 数上限（写路径原子性：满则整请求 503，不产生半批） |

### `[flush]`

| 键 | 缺省 | 说明 |
|---|---|---|
| `snapshot_interval` | `10m` | 快照周期（buffer → parquet 全量冲刷；扁平缓冲后不再决定分桶粒度） |
| `memory_limit` | `512MB` | buffer 内存上限（所有表之和）。触发**减压式冲刷**：按行数降序逐表落盘，总量回落到限下即停（与定时全量快照不同） |
| `worker_threads` | `0` | **快照**专用线程池（`iotp-snapshot`）的 worker 数；`0` = 自动 = 1。快照的 parquet 编码与写盘是同步 IO（全仓无 `spawn_blocking`），一次可能占住 worker 数秒——单独成池后不再让 HTTP/采集在这期间少了 worker。1 个足够（快照被 snapshot_lock 串行化） |

**写法**：`snapshot_interval` 只认小写 `s`/`m`/`d`（**没有 `h`**），`memory_limit` 只认整数 `MB`/`GB`；**读不懂时静默回落缺省**（600s / 512MB）——写错不会报错，只会在启动后表现为"周期/上限不对"。`[query]` 的 `query_memory_limit` / `rss_high_water` 走同一个解析器（同样只认整数 `MB`/`GB`，畸形串回落 512MB）。

### `[query]`

| 键 | 缺省 | 说明 |
|---|---|---|
| `query_timeout_secs` | `30` | 单查询超时（含在查询池内排队的时间 + prepare+plan+collect） |
| `max_rows` | `10000` | 单查询返回行上限（结果行数**达到** `max_rows` 即标 `truncated: true`；恰好等于也标） |
| `max_concurrent_queries` | `4` | 并发查询上限（满则 503 OVERLOADED） |
| `query_memory_limit` | `512MB` | 单查询内存上限：排序/哈希聚合超过即溢写磁盘（`{data.dir}/spill`），不占进程内存 |
| `rss_high_water` | `2048MB` | 进程 RSS 高水位：达到时新查询返回 503（采集不受影响），回落后自动恢复 |
| `worker_threads` | `0` | 专用查询线程池的 worker 数；`0` = 自动 `max(1, 核数-1)`（给采集留一核）。查询在独立线程池上规划/执行，不占用采集与 HTTP 的线程（详见"查询与性能"一节） |

### `[auth]`

| 键 | 缺省 | 说明 |
|---|---|---|
| `enabled` | `false` | 认证与授权总开关。**缺省关闭 = 所有端点开放**（与历史行为一致）；生产必须显式设为 `true`，关闭时启动会打一行 WARN |

开启后有三种主体（`kind`），凭据互不通用：

| kind | 凭据 | 入口 | 建立方式 |
|---|---|---|---|
| `cert` | 客户端证书（**CN 即主体名**） | HTTP + MQTT | 签发证书（§9.5）+ 建 `cert` 主体 |
| `token` | Bearer token（256 位随机） | HTTP | `iotp token new`（§9.4） |
| `mqtt` | CONNECT 的 username/password | MQTT | `iotp mqtt-user new`（§9.4） |

**为什么 `mqtt` 要与 `token` 分开**：两者存在同一张表、同一个 secret 列里，
若不按 kind 分流，MQTT 口令就会被 HTTP 的 `verify_token` 当成 Bearer token 接受 ——
一个弱 MQTT 口令会变成公网 HTTP 端口上的有效凭据。

主体与规则存 `{data.dir}/iotp.db` 的 `auth_*` 表，由 CLI/管理 API 维护（§4.6）。
维护方式：CLI（`iotp token` / `iotp mqtt-user`，§9.4）、管理 API（`/api/v1/meta/auth/*`，§4.6），
或**控制台的「用户管理」页**（原「访问控制」，2026-10-03 更名；新建/启用禁用/轮换/删除与规则增删都在那里，2026-09-19 起）。
管理 API 与控制台的改动立即生效，CLI 等外部改动 ≤5 秒生效。

端点要求：`/health`、`/api/v1/version` 与控制台静态页免认证；`/api/v1/status` 需认证；
`/api/v1/meta/*` 需 `admin`；`/api/v1/alerts*` 与 `/api/v1/rollup*` 需 `query`；
`POST /write` 需 `write`（**按库判定**：`write db:HCYtest` 即可写该库任意表；
表级判定尚未实现）；`POST /api/v1/query` 与
`/api/v1/realtime*` 需 `query`（且**逐表**校验：查询引用的每张表、实时值所属设备
的表都要有权限）。凭据缺失/无效 → **401**（`UNAUTHORIZED`），已认证但无权 →
**403**（`FORBIDDEN`，消息里点名资源）。

**MQTT 侧**：CONNECT 按 `kind = mqtt`（或证书 CN）认证，**主体未登记即拒**；
发布/订阅按 `topic:` 规则逐条判定（§3 `[mqtt]` 的主题级 ACL）。**投递时也会再判一次**，
所以改规则/删主体对**已建立的订阅同样立即生效**，不需要设备重连。详见设计 spec
`docs/specs/2026-09-13-iotp-auth-acl-design.md`。

### `[ingest]`

| 键 | 缺省 | 说明 |
|---|---|---|
| `worker_threads` | `0` | **采集**专用线程池（`iotp-ingest`）的 worker 数；`0` = 自动 = **2**。跑在这条池上的是 MQTT forwarder/consumer 与 WAL 周期 fsync。2 是下限而非随意值：consumer 的 LP 解析是同步 CPU（无 await 点），单个 worker 时解析期间 forwarder 拿不到 CPU，就答不上 rumqttd 的 pause/ready（2026-09-08 断流 3h 的机制）。调大只在多核且 MQTT 吞吐很高时有意义 |

### `[merge]`（合并 —— **服务自己跑**）

**0.6.0 起合并在服务进程内**：服务有一条专用 `iotp-merge` 线程池，按 `schedule`（**墙钟 cron**）
自己触发一轮（逐表自愈 → 分批流式合并 → 原子落盘 → 保留期清理），跑在**服务的 cgroup 里**、
日志进服务日志、**不出现在另一个进程里**（内存账见 §9.9）。`iotp merge-once` 仍然存在，但只是
**手工补跑**用的 CLI 子命令（与服务自动那一轮**会撞车**，见 §9.9）；旧的 `[compaction]` 段与
`iotp compact-once` 已删除（老配置里残留 `[compaction]` 不会报错，只是被忽略——请清理掉）。

| 键 | 缺省 | 说明 |
|---|---|---|
| `enabled` | `true` | 进程内合并调度器的总开关；`false` = 服务完全不跑合并（要手工跑就自己停服务后执行 `merge-once`） |
| `schedule` | `0 */6 * * *` | 触发节拍，**墙钟** cron（5 段：分 时 日 月 周；日/月/周**只支持 `*`**）。**绝不是"启动后 N 小时"**——旧 compaction 把同一个串当成 `sleep(6h)`，生产上"每 6 小时一次 OOM"的节拍正是那么来的 |
| `seal_size` | `32MB` | 合并产物的目标大小；同时是**单批输入上限**与单轮合并的内存/CPU 上界 |
| `retention` | `30d` | 保留期：**区间与 mtime 都早于** `now - retention` 的文件会被删除（见下） |

**配置不合法只禁用合并、不拒绝启动**：`schedule` 写错（比如 `0 3 1 * *` 这种带日/月/周过滤的
写法）、`retention`/`seal_size` 写法不合法时，服务照常启动（采集优先），日志里一行 ERROR 说明
原因，**合并整条停掉**——改好配置重启即可恢复。`iotp merge-once` 则是**拒绝运行那一轮**并
以 exit 2 结束（见 §9.9）。

**写法**：大小用 `MB`/`GB`（大小写不敏感）；时长**只有小写 `s`/`m`/`d`**——**没有小时单位**，
`720h` 会被拒（`[merge] enabled = true` 时，字符串层校验在服务启动与 `iotp merge-once` 两处都会跑，
报错指名道姓；`enabled = false` 时服务启动完全不校验这一段）。被接受的值域：
`seal_size` ∈ [1MB, 4GB]、`retention` ∈ [1 小时, 100 年]，**越界即拒绝运行本轮**（不兜底）。
为什么这么严：合并会**删数据**，而取数函数对畸形串是**静默兜底**的——一次手滑的
`retention = "30D"`（大写 `D` 不被识别）如果放过去就等于 10 分钟保留期，一轮就能把库清空。
所以字符串层（可读报错）与数值层（不管谁调用都拦）两道守卫都必须硬拒。

**合并语义**（设计权威：`docs/specs/2026-09-16-iotp-merge-process-design.md`）：

- 只认可解析的快照/合并 parquet：v3 标记名 `{node}_iotp3w_{min}_{max}_{seq}.parquet`（快照）与
  `{node}_iotp3p{max_input_seq}_{min}_{max}_{hash}.parquet`（合并产物，0.8.6 起），同时兼容 v2
  无标记名 `{node}_{min}_{max}_{seq}.parquet`；目录里不可解析的老格式文件被跳过；
- 候选 = 目录里**小于 `seal_size`** 的文件，按 `(min, max, name)`（≈ 时间）顺序累积成批；产物
  `{node}_iotp3p{max_input_seq}_{min}_{max}_{hash}.parquet`（区间 = 输入区间并集，hash = 输入集的确定性指纹）。
  这是**大小启发式**（不需要任何"已合并"标记）：**产物经常小于 `seal_size`**——合并输出按
  time/device_id 排序，压得比快照小，实测 ≈ 0.7×（生产表影子实测 45 个产物里 38 个落回阈值
  以下）——因而**会重新入选、被整份重写**；无害且幂等（同一输入集得到同一产物名），但
  **首轮之后通常还有第二轮才收敛**；
- **流式**：内存与表有多大**无关**（旧实现要整批载入 + 不可溢写的 `distinct()`，正是每 6 小时
  一次 OOM 的成因，见 §9.8）；整行去重、按 time 排序、按当前权威 schema 投影（缺列补 NULL、
  已删字段列丢弃）；
- **输入先下线、产物后进场**：任一时点至多一份可见，因此查询不会把同一批行读两遍（旧的
  "先 rename 产物、再删输入"会）。崩在中间由下一轮的启动自愈收敛（journal 为权威：产物已就位
  就删输入，没就位就把输入改回原名——不丢行，宁可重复）；
  ⚠️ **反面也要知道**（这是本设计**唯一**查询可见的劣化）：在"输入改名"与"产物 rename"之间，
  两份**都**看不到 —— 这个窗口内**开始**的查询可能**少看到该批一次**，而且**不报错、因此也
  不会触发重试**。窗口长度 = 一次**目录 fsync**，**不是常数**：空闲盘上毫秒级，忙盘上可达
  数十~数百毫秒（未实测）。需要严格一致时避开合并轮次；
- **一轮只碰已落盘的快照/合并 parquet**（v3 标记名与兼容的 v2 名）：不碰 buffer / WAL /
  `last_snapshot.json`，所以服务照常采集与查询（这是设计属性而不是碰巧）；
- **查询撞上正在被删的文件会自动重试一次**：查询在**计划期** pin 文件列表、**执行期**按名字
  打开，合并恰在这两点之间删掉某个输入文件时，该查询会命中"文件不存在"类错误——引擎此时
  **重新规划并再执行一次**（只一次；`table not found` 之类的计划/SQL 错误**不重试**），重试与
  首次尝试共用同一份超时预算与同一个查询并发许可。运维不需要为"合并与查询撞车"做任何事；
- ⚠️ **同一时刻只许跑一轮**：引擎故意不设锁（两个合并者同时动同一张表得不到正确结果）。
  **服务自己就是一个合并者**（按 `schedule` 自动跑），所以**手工 `iotp merge-once` 之前必须先
  停服务**（`sudo systemctl stop iotp` → 跑 → `sudo systemctl start iotp`），不要用外部 timer
  再叠一个合并者。理由与做法见 `deploy/README.md`；
- **失败必须看得见**：服务自己跑的那一轮**不打印**任何东西到终端，账目进服务日志（轮次摘要是
  `info!`，逐批失败原因是 `warn!`；默认过滤器已经保证合并模块的这些行一定落盘，见 §9.3）。
  `iotp merge-once` 另外把账目打一份到 stdout，**一个批次都没成功时以非零退出**（运维脚本据此
  判失败）。逐批原因在同一天的日志里。

**保留期行为**（会真删文件，单向、不可逆、没有 journal）：

- 删除判据是**两条同时成立**：① 文件名区间完全早于 `now - retention`；② 文件 **mtime** 也早于
  `now - retention`；
- mtime 这道门是为**冻结设备时钟**加的：这类设备推上来的数据带着很旧的区间，只看事件时间会把
  **刚收到**的数据当场删掉。加上 mtime 后，文件按**"收到时间"**计龄过期（`now > max(事件时间,
  mtime) + retention`），磁盘仍是有界的；
- 推论：**事件时间早已过期、但一直在写的表不会被自动清掉**（这是刻意的取舍——宁可留着，也不能
  删掉刚收到的数据），需要运维处置；
- 系统时钟读不出来时**整轮跳过清扫**（绝不按一个错误的"现在"删数据），账目行里会注明
  "（清扫跳过：读不出时钟）"；
- ⚠️ 改小 `retention` 是**立即生效的删除**，先想清楚再改。

### `[mqtt]`（可选，缺省关闭）

| 键 | 缺省 | 说明 |
|---|---|---|
| `enabled` | `false` | 内置 MQTT broker 开关 |
| `max_payload_size` | `10485760` | PUBLISH 载荷上限（**固定 10MB，不随 `[server].max_body_bytes` 联动**；**超限客户端会被 broker 断开连接，iotp 侧无计数**；0 为非法值） |

**监听端口起不来 → 启动失败**（2026-09-12 起）：启动时先预检每个监听端口是否可绑，起来后再确认端口已在监听；任一失败进程直接以非零码退出，不会"HTTP 正常但 broker 不在"。此前 rumqttd 把监听线程的 bind 失败吞成一行日志，进程会健康地跑在没有 broker 的状态下，**网关连不上而无人察觉**——宁可起不来。

监听器（至少一个）：

```toml
[mqtt.listeners.tcp]        # MQTT 3.1.1，缺省端口 1883
port = 1883
# tls_cert = "/path/server.crt"   # 配了 cert+key 该监听即 TLS
# tls_key  = "/path/server.key"
# tls_ca   = "/path/ca.crt"       # 可选：配了则**接受**客户端证书（CN 即身份）

[mqtt.listeners.websocket]  # 缺省端口 9001
port = 9001
```

**传输层三档**（2026-09-14 起，此前只有"明文"与"mTLS-only"两个极端）：

| 档 | 配置 | 客户端证书 | 适用 |
|---|---|---|---|
| A 明文 | 只写 `port` | — | 内网 / 过渡期。口令在途可被嗅探 |
| B 服务端 TLS | `tls_cert` + `tls_key` | **可选**（带了就以 CN 为身份） | 只需一张服务端证书，设备仍用口令 —— 加密但不必给每台设备签证书 |
| C mTLS | 再加 `tls_ca` | 可选，**未带则回退口令** | 与 HTTP 侧共用 CA；设备侧最强身份 |

> ⚠️ 2026-09-13～14 之间曾要求 `tls_cert` 必须配 `tls_ca`（漏配即启动失败）。那是
> rumqttd `verify-client-cert` 编译期 feature 的行为，已在 fork 里补出档 B/C 的分支，
> **成对要求随之取消**；`tls_ca` 单独出现（没有服务端证书）仍然是启动错误。

TLS 文件必须**成对且可读**：`tls_cert`/`tls_key` 成对出现，`tls_cert`/`tls_key`/`tls_ca`
三份文件都必须能打开，否则启动失败。以上监听端口、TLS 与 `max_payload_size` 的校验
**只在 `[mqtt] enabled = true` 时执行**；关闭时这些配置被静默忽略，不会阻止启动。

**认证**：**没有 `[mqtt.auth]` 段了**（2026-09-14 移除）。口令与 HTTP token 存在同一张
SQLite 主体表里，按 `kind` 分流：

```bash
# 建 MQTT 口令主体（username = 主体名）。--password 缺省则生成高熵口令并只打印一次
iotp mqtt-user new --name gw-01 --password '<口令>' \
    --allow 'mqtt_pub:topic:devices/+/data' --note '01 号网关'

iotp token list          # 三种 kind（cert/token/mqtt）一起列
```

- 认证开关就是 `[auth] enabled`：**关闭时 MQTT 与 HTTP 一样不认证**（此前 `[mqtt.auth]`
  独立于 `[auth]`，行为已变）。
- 配置里仍写着 `[mqtt.auth]` → **启动失败**并打印迁移命令。不是"忽略 + 告警"：
  那等于"配了认证却没生效"。
- **MQTT 口令不能当 HTTP Bearer token 用**，反之亦然（`kind` 在类型层面隔离）——
  否则一个弱 MQTT 口令会变成公网 HTTP 端口上的凭据。
- 主体**未登记即拒**：CONNECT 阶段就断开，而不是"连上了但什么都做不了"。

**主题级 ACL**（2026-09-14 起）：按**主题**授权，与库/表概念无关 —— 非采集用途的主题
（控制、状态、OTA…）同样能授权。

```bash
iotp mqtt-user new --name gw-01 --allow 'mqtt_pub:topic:devices/+/data' \
                                      --allow 'mqtt_sub:topic:devices/1/#'
```

| 动作 | 资源 | 说明 |
|---|---|---|
| `mqtt_pub` | `topic:<通配模式>` | 发布；`+` 单层、`#` 多层，与 `[mqtt.ingest]` 同一套通配语义 |
| `mqtt_sub` | `topic:<通配模式>` | 订阅；**规则必须覆盖过滤器**（见下），`$share/<组>/<主题>` 按**真实主题**判权 |

**默认拒绝**，且动作之间不互相蕴含（有 `mqtt_pub` 不等于能订阅）。

**订阅按"覆盖"判，不是按"匹配"判**：订阅请求的是一个**过滤器**（可能自带 `+`/`#`），
只有当过滤器能收到的**每一个**主题都被某条规则覆盖时才放行。所以**规则的宽窄必须 ≥ 过滤器的宽窄**：

| 规则 | 过滤器 | 结果 |
|---|---|---|
| `topic:devices/+` | `devices/1`、`devices/+` | ✅ 覆盖 |
| `topic:devices/+` | `devices/#` | ❌ **`#` 比 `+` 宽** |
| `topic:#` | 任意 | ✅ |
| `topic:devices/1/#` | `devices/1/data` | ✅ |

这条反直觉但方向是对的：**订阅只能收窄，不能放宽**。所以"只给了 `devices/+` 却能订阅 `#`
读全部主题"这种事不会发生。发布不受影响 —— 发布的是真实主题（没有通配符），按普通通配匹配判。

越权时：

- **发布** → 消息被**丢弃**（不进 commitlog、不落库、订阅者也收不到），**连接保留**。
  QoS 1 仍会收到 PUBACK（协议流程走完，载荷丢掉）—— 不断连是为了避免"主题配错 =
  重连风暴"（2026-09-08 断流 3h 的形态）。
- **订阅** → 该 filter 收到 **SUBACK `0x80`（Failure）**，同一次 SUBSCRIBE 里合法的
  filter 照常生效，连接保留。
- iotp 自己的采集链路（`iotp-consumer`，进程内 link）**豁免 ACL** —— 它是 broker 的
  一部分，不是网络客户端。
- 越权**只进内存计数**（不落 SQLite，防放大），见 `GET /api/v1/meta/mqtt` 的 `acl` 段：
  `denied_total` 永远准确；细分按 (主体, 主题, 动作) 聚合，**键数上限 256**、主题串截断
  到 96 字节，超出部分并入一个 `overflow` 桶（主题由对端任意构造，无界的表就是内存
  放大攻击）。重启清零，长期趋势交给监控采集本端点。

> **语义提醒**：发布到**命中 `[mqtt.ingest]` 规则**的主题，等于获得该主题所映射库的
> 写入权（消息会被落进那个库）。MQTT ACL 刻意不引入库/表概念，这条隐含授权是取舍。

采集规则（`[mqtt.ingest]`，主题通配符 → db + 格式）：

```toml
[mqtt.ingest]
"devices/+/data" = "metrics"                              # 简写：format 缺省 "lp"
"devices/+/json" = { db = "metrics", format = "json" }    # 显式 JSON 规则
```

- 通配符：`+` 单层、`#` 多层（`#` 必须是末段且为整段）；按配置文件顺序**首条命中**，同一条消息最多进一个 db；未命中任何规则 → 不采集（broker 仍正常转发给其他订阅者）。
- 主题可带**前导斜杠**（`"/hubin/devices/+/data"`）：首段为空是合法的 MQTT 形态，照常匹配；只拒绝**中间/结尾**的空段（`devices//data`）——那才是笔误，启动报错。
- `format` 仅接受 `"lp"` / `"json"`，非法值启动报错。
- ⚠️ 任何一条 ingest 规则能覆盖 `iotp/alerts/` 前缀（`#`、`iotp/#`、`iotp/alerts/+/+` …）
  时**告警引擎被禁用**（采集照常，日志里有一条 ERROR 指名道姓报出是哪条规则）——
  否则 iotp 会把自己的告警当采集数据收回来，见 §7.4。

### `[alerts]`（可选，缺省关闭）

进程内实时告警引擎（2026-09-27）：按 `tick` 巡检每台设备的**最新值**，状态**变化**时写
一行告警到 `out_db.out_table`（引擎在**第一次有规则的那次重载**里创建，通常是启动时
已有规则、或建了第一条规则之后的下一个 tick）并发布到 MQTT 主题。

⚠️ **规则不写在这个文件里**（2026-09-27 起）：它们住在 `{data.dir}/iotp.db` 里，在
控制台的「告警管理」页维护 —— 规则的形态、改动的生效时机与运维口径都在 §7.4。这一段里再写
`[[alerts.rules]]` 会被**明确拒绝**（日志里一条 ERROR + **只禁用告警**、服务照常起）：
那是旧形态，而 serde 会静默忽略未知字段，不拦就等于"配了却不生效"。（哨兵跟着
`enabled = true` 一起判：`enabled = false` 时这一段既不报错也不生效 —— 反正引擎没跑。）

| 键 | 缺省 | 说明 |
|---|---|---|
| `enabled` | `false` | 总开关。**缺省关** —— 它依赖 `[mqtt]` 投递与一张输出表，缺省开会改变升级后的行为 |
| `tick` | `"1s"` | 巡检节拍。判定是纯内存操作（微秒级），1s 毫无压力；单位只有小写 `s`/`m`/`h`/`d`，**读不懂就禁用告警**（不兜底、也不拒绝启动） |
| `out_db` | — | 告警表所在库，引擎自动创建。`enabled = true` 时必填，且不能含空白、`,`、`=`（会拼进行协议测量名） |
| `out_table` | — | 告警表名。约束同 `out_db`；违反时**只禁用告警**、服务照常启动 |

```toml
[alerts]
enabled = true
tick = "1s"
out_db = "ops"
out_table = "alerts"
```

- 规则引用的表/字段由**装配期**逐条对着权威 schema 校验（strict-only）：对不上的那条
  **只废掉它自己**（控制台「告警管理」页上能看到原因），其余规则照常跑 —— 一条手滑的规则
  不该停掉所有告警。
- `[mqtt]` 未启用时**照写告警表、不发布**（没有链路）。
- 规则的源表**不能是告警表自己**（那会把引擎刚写进去的告警当输入）。

### `[rollup]`（可选，缺省关闭）

进程内窗口聚合引擎（2026-10-01）：按固定节拍把源表**按时间窗口**聚合成结果行，写进每条规则
自己的输出表。窗口语义、缺口与补算的运维口径都在 §7.5 —— 这一段只说这三个标量。

⚠️ **规则不写在这个文件里**：与告警同款，规则住在 `{data.dir}/iotp.db` 的 `rollup_rules`
三张表里，在控制台「聚合管理」页或 `/api/v1/meta/rollup/rules`（§4.7）维护。`[rollup]` 只有标量，
没有、也不接受任何规则字段。

| 键 | 缺省 | 说明 |
|---|---|---|
| `enabled` | `false` | 总开关。**缺省关**（与 `[alerts]` 同款：缺省开会改变升级后的行为——新表、新设备、后台查询） |
| `tick_secs` | `60` | 调度器**唤醒节拍**（秒）：多久检查一次"有窗口到期吗"。**不是窗口宽度**（窗口宽度在规则里，60–3600 秒且必须是 60 的整数倍）。调小只把到期后的等待从 ≤tick 压缩一点（且被 grace 淹没），唤醒次数成倍涨 |
| `grace_secs` | `60` | 宽限期（**绝对秒**，0–86400）：窗口封口后再等这么久才算它，覆盖设备补发/乱序。**绝对秒而不是窗口的倍数**——迟到视野是设备群的属性（补发延迟几分钟），与窗口宽度无关 |

```toml
[rollup]
enabled = true
tick_secs = 60
grace_secs = 60
```

**总延迟 ≈ `grace_secs + tick_secs`**：窗口封口后 `grace_secs` 到期，再等最多一个 tick 被处理
（缺省配置下 1 分钟窗口约 2–3 分钟后出结果）。

非法标量（`tick_secs = 0`、`grace_secs` 越界）**只禁用 rollup**、服务照常启动（采集优先，
与 `[merge]`/`[alerts]` 同纪律，原因在启动日志的一条 ERROR 里）；`enabled = false` 时**不校验**
（关掉的引擎不该被一个没人看的旋钮拦住启动）。规则自己的毛病（源表/字段对不上、输出表建不出来）
是另一条路：**逐条拒绝**，见 §7.5。

### `[realtime]`（可选，缺省关闭）

网关**实时推送 + 写控制**的 WebSocket 服务（2026-11-02）。`enabled` **只控制 WS
listener/端口**（缺省 8085）与这个服务的关闭信号：关着时没有该端口，WS 客户端连不上，
其余行为与改动前一致。有两件事**不由这个开关控制**：订阅 hub 在所有模式下都会构造
（无 WS 连接时零订阅，写路径 observer 直接返回）；`iotp-command` 本地链路与 reply
消费者只由 `[mqtt] enabled` 决定（与告警链路同一个创建窗口），`[realtime]` 关着时它们
照旧存在。协议、同节点约束、审计与已知限制见 §7.6。

| 键 | 缺省 | 说明 |
|---|---|---|
| `enabled` | `false` | 只控制 WS listener/端口（及该服务的关闭信号）。**缺省关**；`[mqtt]` 的 `iotp-command` 链路不受它控制 |
| `host` | `0.0.0.0` | 监听地址 |
| `port` | `8085` | 监听端口。**部署位置**字段：与 `[server].port` 一样不参与配置指纹（各节点端口不同不算配置分叉） |
| `write_timeout_secs` | `5` | 等网关 `.../write/reply` 的超时（秒）；超时 → WS 返回 400 |
| `max_subscriptions_per_conn` | `1024` | 单连接订阅项上限（超出保留前 N 个并告警，不断开） |
| `max_pending_writes_per_conn` | `16` | 单连接在飞写控制上限（超出 400；reason=`too_many_pending` 只出现在审计/日志，响应体仍是通用 `Write Error`） |
| `max_message_bytes` | `65536` | 单条 WS 消息字节上限（超出 close 1009） |
| `max_control_per_sec` | `10` | 每连接写控制速率上限（超出直接 400、无审计；保护网关与审计） |
| `audit_outbox_max_rows` | `50000` | 审计 outbox 行数上限（**仅 `role = "node"`**） |
| `audit_outbox_max_bytes` | `67108864` | 审计 outbox 字节上限（**仅 `role = "node"`**，64MB） |
| `audit_outbox_warn_pct` | `80` | outbox 使用率告警水位（**仅 `role = "node"`**） |

```toml
[realtime]
enabled = true
# host = "0.0.0.0"
# port = 8085
# write_timeout_secs = 5
```

- `enabled = true` 时若 `host:port` **绑定失败**，进程拒绝启动（非零退出，与 HTTP/MQTT 同纪律）。
- 该端口**只有 WS 升级**（路径 `/ws` 或 `/ws/`），默认明文；生产放在 TLS 反向代理后。
  浏览器 `WebSocket` API 不能设 `Authorization` 头，认证开启时用 `?token=`（会进 URL/日志，
  必须配合 TLS 或内网限制）。
- `[auth] enabled = true` 时：升级校验凭据；每条 `read` / `write` 重新做资源级授权
  （`read` 需 `query`、`write` 需 `control`，资源都是 `table:<库>.<表>`），主体被禁用/删除后
  连接以 close 1008 断开。
- `audit_outbox_*` 只有集群节点（`role = "node"`）使用：控制审计先落本地 outbox、后台补送
  control；control / 独立模式直接写本地 `auth_audit`。
- `[mqtt]` 未启用时没有命令链路：WS 仍可连接、`read` 订阅照常，但 `type=write` 直接 400
  （响应体仍是通用 `Write Error`；服务端日志/审计 reason=`control_unavailable`），
  不下发、也不会有网关。

---

## 4. 元数据管理 API

本章用于创建和维护数据模型。IoTP 使用 **strict-only** 模式：写入前必须先通过管理 API 声明数据库、表、字段和设备。

**推荐顺序：** 创建库 → 创建表及字段 → 注册设备 → 开始写入。集群部署时，管理写请求应发送到控制面。

前缀 `/api/v1/meta`，请求/响应均为 JSON。错误体统一 `{"error": "...", "code": "..."}`：
400 `BAD_REQUEST`、401 `UNAUTHORIZED`、403 `FORBIDDEN`（见 §3 `[auth]`）、404 `NOT_FOUND`、
405 `METHOD_NOT_ALLOWED`、409 `CONFLICT`（含「空才能删」与节点角色下的 `NOT_CONTROL_NODE`）、
413 `PAYLOAD_TOO_LARGE`、500 `INTERNAL`。
`[cluster] role = "node"` 上的管理写一律 409 `NOT_CONTROL_NODE`，请到控制面执行。

### 4.1 库

```bash
# 创建
curl -X POST http://127.0.0.1:8080/api/v1/meta/databases -d '{"name":"metrics"}'
# 列表
curl http://127.0.0.1:8080/api/v1/meta/databases
# 删除（「空才能删」：库里还有表 → 409；先删空所有表，通过后清理该库数据目录——不可逆）
curl -X DELETE http://127.0.0.1:8080/api/v1/meta/databases/metrics
```

### 4.2 表

```bash
# 创建（fields: name + type(I64|F64|Bool|String) + is_tag + 可选 attrs）
# 字段名不得为保留名 time/device_id/table/name/gateway（400），也不得含 - 或空白
curl -X POST http://127.0.0.1:8080/api/v1/meta/tables -d '{
  "db":"metrics","table":"cpu",
  "fields":[
    {"name":"i","type":"I64"},
    {"name":"f","type":"F64"},
    {"name":"b","type":"Bool"},
    {"name":"s","type":"String"},
    {"name":"region","type":"String","is_tag":true}
  ]}'
# 列表（**必须带 ?db=**，否则 400）
curl 'http://127.0.0.1:8080/api/v1/meta/tables?db=metrics'
# 单表详情（响应形状见 §4.5）
curl http://127.0.0.1:8080/api/v1/meta/tables/metrics/cpu
# 删除（「空才能删」：表内还有字段、或仍有数据（buffer ∪ parquet）→ 409；通过后才移除数据目录）
curl -X DELETE http://127.0.0.1:8080/api/v1/meta/tables/metrics/cpu
```

### 4.3 字段

```bash
# 新增字段（既有数据/设备实例该列为 NULL；不会给既有设备补标签值）
curl -X PUT http://127.0.0.1:8080/api/v1/meta/tables/metrics/cpu/fields \
  -d '{"name":"u","type":"I64"}'
# 改类型（表已有数据 → 409）
curl -X PUT http://127.0.0.1:8080/api/v1/meta/tables/metrics/cpu/fields/u \
  -d '{"type":"F64"}'
# 改名（标签字段随时；测量字段仅限表里还没有数据时，否则 409；旧名不落墓碑）
curl -X PUT http://127.0.0.1:8080/api/v1/meta/tables/metrics/cpu/fields/u \
  -d '{"name":"u2"}'
# 删除字段：测量字段落墓碑（名字不可复用）；标签字段不落墓碑、名字可复用，
# 但删后同表设备的标签组合若重复 → 409
curl -X DELETE http://127.0.0.1:8080/api/v1/meta/tables/metrics/cpu/fields/u2
```

### 4.4 设备实例

```bash
# 注册（device_id 可省略、由服务端分配；显式指定须为正整数且 < 2^40，同表不重复；
# tags 只含标签字段；组合在同一表内唯一）
curl -X POST http://127.0.0.1:8080/api/v1/meta/devices -d '{
  "db":"metrics","table":"cpu","name":"机房1","tags":{"region":"east"}}'
# → {"id": 1, ...}   1 即写入时的 device_id

# 批量注册（devices 数组，每项自带 db/table，可跨库跨表）
curl -X POST http://127.0.0.1:8080/api/v1/meta/devices -d '{
  "devices":[
    {"db":"metrics","table":"cpu","device_id":2,"tags":{"region":"west"}},
    {"db":"other","table":"elec","device_id":5}
  ]}'
# → {"created":2,"results":[{"ok":{...}},{"ok":{...}}]}
# 部分失败逐条记 error（200 + created 计数）；全部失败 400 附逐条结果

# 按标签组合查 id（GET；匹配语义见 §4.5）
curl 'http://127.0.0.1:8080/api/v1/meta/devices/lookup?db=metrics&table=cpu&region=east'
# 更新（name/attrs 省略 = 保持原值；tags 是**整体替换**——省略 tags 等于清空该设备全部标签值）
curl -X PUT http://127.0.0.1:8080/api/v1/meta/devices/1 \
  -d '{"name":"机房1-新","tags":{"region":"west"}}'
# 删除
curl -X DELETE http://127.0.0.1:8080/api/v1/meta/devices/1
```

批量注册的两条形状约束：`devices` 键不能与顶层 `db/table/device_id/name/attrs/tags` 混用（400）；`devices: []` → 400。

### 4.5 查询（读）接口与响应形状

以上是写操作；下面是全部**读接口**的完整响应形状。通用语义：名称（db/表/字段/标签值）**大小写敏感**；错误体统一 `{"error": "...", "code": "..."}`。

**① 库列表** — `GET /databases`

```json
["metrics", "other"]
```

**② 表名列表** — `GET /tables?db=metrics`（`db` 参数必填，缺省 400）

```json
["cpu", "elec"]
```

**③ 单表详情** — `GET /tables/{db}/{table}`（不存在 → 404）

```json
{
  "db": "metrics", "table": "cpu",
  "fields": [
    {"name": "i", "type": "I64", "is_tag": false, "attrs": null},
    {"name": "f", "type": "F64", "is_tag": false, "attrs": null},
    {"name": "region", "type": "String", "is_tag": true, "attrs": null}
  ],
  "device_count": 3
}
```

`fields` 按声明顺序；`is_tag: true` = 标签字段（String）；`attrs` 为字段级元数据（v1 只存不解释，未设置时为 null）。

**④ 该表设备列表** — `GET /tables/{db}/{table}/devices`

```json
[{"id": 1, "db": "metrics", "table": "cpu", "name": "机房1",
  "attrs": null, "tags": {"region": "east"}, "tags_str": "region=east",
  "gateway": null}]
```

**⑤ 全部设备** — `GET /devices`（跨所有库所有表）

```json
[{"id": 1, "db": "metrics", "table": "cpu", "name": null,
  "attrs": null, "tags": {}, "tags_str": "", "gateway": null}]
```

**设备对象形状**（④⑤⑥⑦通用）：`id` = 写入时的 device_id；`name` 未设置为 null；`tags` = 标签字段名→值（未赋值的标签字段不出现）；`tags_str` = `k=v` 按 tag_field_id 顺序以 `&` 连接（组合唯一性判定用同一组合）；`gateway` = 设备当前归属（无章为 null；显示的是**原始章**，语义见 §9.11）。

**⑥ 单台设备** — `GET /devices/{id}`（不存在 → 404）。响应 = 设备对象。

**⑦ 按标签值查设备** — `GET /devices/lookup?db=&table=&k1=v1&k2=v2`

- `db`、`table` 必填（缺 → 400；表不存在 → 404）；
- 其余查询参数全部作为**标签过滤**：每个 `k=v` 都必须与该设备的标签值**完全相等**（`&` 关系）；
- 某设备没有该标签字段的值（NULL）→ 不匹配、被排除；未知标签名 → 同样无匹配；
- 响应 = 匹配的设备对象数组（可能为空数组）。

```bash
# 查 region=east 且 plant=3 的设备
curl 'http://127.0.0.1:8080/api/v1/meta/devices/lookup?db=metrics&table=cpu&region=east&plant=3'
# → [{"id": 5, "db": "metrics", "table": "cpu", "name": null, ...}]
```

**⑧ 经数据查询 API 查元数据**：每张数据表同 db 下注册有 `devices` 辅助表（列：`device_id`、`name`、`table`（= `"db.table"`）、每个标签字段一列、末列 `gateway`），可与数据表 JOIN（用法见第 6 节）；`GET /api/v1/meta/mqtt` 的响应形状见 §7.3。

### 4.6 用户管理（主体与规则；原「访问控制」）

> 仅在 `[auth] enabled = true` 时生效；关闭时这些接口仍可调用（管理 API 自身受 `admin` 能力保护，
> 但关掉认证就没有任何鉴权了）。控制台的「用户管理」页可直接增删改（2026-09-19 起），与这里、CLI（§9.4）等价。

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/api/v1/meta/auth/principals` | 列出主体及其规则 |
| `POST` | `/api/v1/meta/auth/principals` | 新建主体；token 主体的令牌**只在本响应返回一次** |
| `PUT` | `/api/v1/meta/auth/principals/{id}` | 启用/禁用（body 必须含 `enabled`） |
| `DELETE` | `/api/v1/meta/auth/principals/{id}` | 删除主体（**吊销主路径**，立即生效） |
| `POST` | `/api/v1/meta/auth/principals/{id}/token` | 轮换密钥：返回新令牌，旧令牌/口令立即失效 |
| `POST` | `/api/v1/meta/auth/principals/{id}/rules` | 新增规则（body 为 `{"action","resource"}`，单个）；规则列表从 `GET /principals` 的每个主体里取（该路径的 GET 返回 405） |
| `DELETE` | `/api/v1/meta/auth/rules/{id}` | 删除一条规则（id 取自规则列表） |
| `GET` | `/api/v1/meta/auth/audit?limit=N` | 最近 N 条审计（默认 50，**上限 500**）；不按路径分域，见下 |

**审计覆盖范围**（统一存 `iotp.db` 的 `auth_audit`，控制台**「变更记录」页**即此表 —— 2026-10-03 起独立成 tab）：
- 主体/规则增删改与令牌轮换（`principal_*` / `rule_*` / `token_rotated`，`actor` 为操作者主体名；CLI 操作为 `cli`）；
- 告警规则与聚合规则增删改（`alert_rule_*` / `rollup_rule_*`）；
- **数据模型变更（2026-10-03 起）**：库/表/字段/设备的增删改 —— `database_added|deleted`、`table_added|deleted`、`field_added|deleted|renamed|type_changed`、`device_added|updated|deleted`。
  粒度 = 一次人工动作一行：批量注册只记一行（detail 写"批量注册 N 台"）、设备标签修改记 `改前 → 改后`、
  删表/删字段在 detail 里留下被删时刻的字段形态；**零变更动作与失败的请求不记**（幂等删、被 409 拒绝的删除等）。
  认证关闭时 `actor` 为 `api`（此时人人可调，审计只反映"经由 API 改动"）。
- **集群拓扑与网关归属（2026-10-06 / 0.9.1）**：`cluster_node_added|removed`、`cluster_gateway_added|removed|placed`、`gateway_attached`、`device_gateway_set`。
- **列**（2026-10-03 起）：**手工** `iotp rollup-once` 补算时 provision 出的输出表与合成设备也各记一行（`actor=cli`，detail 标 `rollup-once`）；
  服务内引擎**自动** provision 输出表、自动建镜像/合成设备**不记**（系统动作，且每 tick 重建被删镜像设备会灌满审计表）。
- 认证失败与授权拒绝**只进内存计数、不落库**（防高频放大，spec §4.3），见 `GET /api/v1/meta/mqtt` 的 `acl` 段。

```bash
T='Bearer <ops-token>'; B=http://127.0.0.1:8080

# 建 token 主体（人/脚本）：令牌只在响应里出现一次，库里只存 sha256
curl -X POST $B/api/v1/meta/auth/principals -H "Authorization: $T" -d '{
  "name":"ops-wft","kind":"token","note":"运维",
  "rules":[{"action":"query","resource":"db:*"},{"action":"admin","resource":"*"}]}'
# → {"id":3,"name":"ops-wft","kind":"token","enabled":true,"token":"iotp_…"}

# 建 MQTT 口令主体：自带口令（过渡期）或省略让服务端生成（响应里的 token 即口令）
curl -X POST $B/api/v1/meta/auth/principals -H "Authorization: $T" -d '{
  "name":"gw-01","kind":"mqtt","password":"<口令>",
  "rules":[{"action":"mqtt_pub","resource":"topic:devices/+/data"}]}'

# 建证书主体：**没有 secret**，凭据就是证书本身（CN 必须等于 name）
curl -X POST $B/api/v1/meta/auth/principals -H "Authorization: $T" -d '{
  "name":"gw-230","kind":"cert","rules":[{"action":"write","resource":"db:HCYtest"}]}'

# 加一条规则 / 删一条规则（规则 id 从主体列表里取）
curl -X POST $B/api/v1/meta/auth/principals/3/rules -H "Authorization: $T" \
  -d '{"action":"mqtt_sub","resource":"topic:devices/#"}'
curl -X DELETE $B/api/v1/meta/auth/rules/7 -H "Authorization: $T"

# 禁用（不禁用也能靠删主体，但禁用保留了规则，便于恢复）
curl -X PUT $B/api/v1/meta/auth/principals/3 -H "Authorization: $T" -d '{"enabled":false}'
# 轮换密钥（旧凭据立即失效）
curl -X POST $B/api/v1/meta/auth/principals/3/token -H "Authorization: $T"
# 审计：谁在什么时候改了哪个主体的什么
curl "$B/api/v1/meta/auth/audit?limit=10" -H "Authorization: $T"
```

**校验与响应**：`kind` 省略时按 `token` 处理；显式给出时必须是 `token` / `cert` / `mqtt`；`action` 必须是
`write` / `query` / `admin` / `control` / `mqtt_pub` / `mqtt_sub`；`resource` 必须是 `*`、`db:<名>|*`、
`table:<库>.<表>|*` 或 `topic:<通配模式>`（通配语义见 §6 的 `[mqtt.ingest]`，两处共用同一份校验）。
**坏请求 400，且不落库**（先全量校验、再写入，不留半成品）。重名 409。
**响应永不返回 secret**（token 主体的明文只在建/轮换那一次；证书主体的 `secret_sha256` 恒为 null）。

**生效时间**：经本 API 的改动**立即生效**；经 CLI 等**外部进程**的改动 ≤5 秒
（服务盯着 `iotp.db` 与 `iotp.db-wal` 的 mtime/size）。已建立的 MQTT 连接**不需要重连**——
主题 ACL 在**每次投递时**判定，所以吊销/改规则对在线会话同样即时生效。

### 4.7 聚合（rollup）规则

窗口聚合引擎的规则（2026-10-01）。引擎的语义、缺口与补算在 §7.5；这里只有端点与请求体形状。
权限 **Admin**（与其余 `/api/v1/meta/*` 同档）。

```bash
# 列表（全部规则：含聚合与分组，各按插入序）
curl $B/api/v1/meta/rollup/rules -H "Authorization: $T"
# 新建 → 200，返回与请求同形状 + id
curl -X POST $B/api/v1/meta/rollup/rules -H "Authorization: $T" -d '{
  "name":"cpu_1m",
  "source_db":"metrics","source_table":"cpu",
  "out_db":"ops","out_table":"cpu_1m",
  "window_secs":60,
  "aggregates":[
    {"func":"avg","field":"Ua","alias":"ua_avg"},
    {"func":"max","field":"Ua","alias":"ua_max"}
  ]}'
# 改（全量替换：体与 POST 同形，GET 回来的对象可原样 PUT 回去）
curl -X PUT $B/api/v1/meta/rollup/rules/3 -H "Authorization: $T" -d '{...}'
# 删（只删规则；输出表与已经写出的结果行都留着）
curl -X DELETE $B/api/v1/meta/rollup/rules/3 -H "Authorization: $T"
```

响应形状：列表 = `{"rules":[<规则对象>…]}`；新建/修改 = 单条规则对象（含 `id`、`enabled`、`aggregates`、`group` 等）。

| 字段 | 说明 |
|---|---|
| `name` | 规则名。非空、**无空白、无 `/`**（名字进日志与状态端点；首尾空白在这里不 `trim`，直接 400） |
| `source_db` / `source_table` | 源表 |
| `out_db` / `out_table` | 输出库/表。**同一张输出表只能归一条规则**（引擎靠"一规则一表"从 `max(time)` 推导进度），撞了 409；重名也 409 |
| `window_secs` | 窗口宽度（秒），60–3600 且为 60 的整数倍 |
| `enabled` | 布尔，缺省 `true`。禁用 = 不参与聚合，也没法补算（`rollup-once` 只跑 enabled 的规则） |
| `group` | 标签字段名数组。**省略 / 空数组 = 按设备分组**（每台源设备每窗口一行，可回溯源设备）；非空 = 按这些标签字段的取值组合分组。分组字段必须是源表的**标签字段** |
| `aggregates` | 至少一条：`func`（算子名，白名单见 `GET /api/v1/rollup/state` 的 `operators`）、`field`（源表的**测量字段**）、`alias`（输出列名，本规则内唯一、非保留名、不能含 `-`、空白、`,` 或 `=` —— 它同时是回写行协议的字段名，含 `,`/`=` 会让整批结果行被解析器拒掉）、可选 `arg`（**字符串**）、可选 `field2` |

- `arg` 是字符串：`approx_percentile_cont` 的 p（`"0.95"`）、`string_agg` 的分隔符（`","`）、
  `nth_value` 的 n（`"2"`）—— 库里就是文本列，装配层拿同一串再判一次，中间不经过有损转换。
- `field2` 只有**双字段算子**（`corr` / `covar_*` / `regr_*`）才给：缺了 400，给错了算子也 400。

**这一层只挡请求形状**（字面量、白名单、别名与参数的形态）。字段在不在源表上、类型合不合算子的
输入签名、`group` 里是不是标签字段、源表与输出表会不会成环……都由**装配期**对着权威 schema
**逐条**校验（strict-only）。装配不了的、以及输出表建不出来（provision 失败）的那条**只废掉
它自己**（其余规则照常跑），**原因列在 `GET /api/v1/rollup/state` 的 `rejected` 里**
（也进日志与控制台「聚合管理」页）。规则改动 **≤1 个 tick 生效**，不需要重启；输出表缺的标签列
由引擎自动补（加宽只加标签列）。

### 4.8 节点、网关与设备归属（集群拓扑）

集群分片图的管理端点（2026-10-06 起；attach 为 0.9.1 新增）。权限 **Admin**；`role = "node"` 上的非 GET 请求统一 409 `NOT_CONTROL_NODE`。语义、上线序列与排障见 §9.11。

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/api/v1/meta/nodes` | 已声明节点：`[{"id","note","created_at"}]` |
| `POST` | `/api/v1/meta/nodes` | body `{"id":"n01","note"?}` → `{"ok":true,"id":"n01"}`；重复 409，名字含非法字符 400 |
| `DELETE` | `/api/v1/meta/nodes/{id}` | → `{"ok":true}`；仍被网关放置集引用 → 409 并点名网关 |
| `GET` | `/api/v1/meta/gateways` | `[{"name","note","created_at","nodes":[…],"device_count":N}]` |
| `POST` | `/api/v1/meta/gateways` | body `{"name":"gw-01","note"?}` → `{"ok":true,"name":"gw-01"}`；重复 409 |
| `DELETE` | `/api/v1/meta/gateways/{name}` | → `{"ok":true}`；设备章变为悬挂章/未归属 |
| `PUT` | `/api/v1/meta/gateways/{name}/nodes` | body `{"nodes":["n01"]}` → `{"ok":true}`；**整体替换**放置集，空数组 = 未放置；引用未声明节点 400、网关不存在 404 |
| `POST` | `/api/v1/meta/gateways/{client_id}/attach` | 声明 + 防抢盖章 + 只增放置（见下）；`client_id` 非法 400 |
| `POST` | `/api/v1/meta/devices/gateway` | body `{"device_ids":[5,6],"gateway":"gw-01"}`（`gateway: null` = 清除）→ `{"ok":true,"changed":N}`；逐条独立事务、**非原子**，`changed` = 处理条数（重复 id 记多次） |

**attach**：body `{"device_ids":[5,6,7]}`。节点由 control 聚合的 `online_clients` 自动发现：0 个 → 只声明 + 盖章（可预配置，已有放置集保持不动）；1 个 → 加入放置集（**只增，不删旧节点**）；>1 个 → **409 且不做任何修改**。已属于另一已声明网关的设备不抢，回 `conflicts[]`（仍 200）；指向未声明网关的悬挂章自愈进 `repaired[]`。设备不存在 → 404 且整体回滚；未知请求键 / 空 `device_ids` → 400。独立模式下 `nodes_found=[]`、`placement_added=[]`，只声明与盖章、跳过放置。响应字段与更多说明见 §9.11。

---

## 5. 数据写入

IoTP 支持 HTTP 行协议（LP）、HTTP JSON 信封和 MQTT 三种采集入口。三种入口都遵循已声明的数据模型及逐值无损转换规则；请结合设备协议和系统架构选择。


### 5.1 行协议（LP，HTTP）

`POST /write?db=<db>`，Content-Type 缺省/`text/plain`。`db` 可省略（缺省写入 `default` 库）；同名参数重复时取**第一个**。行格式（InfluxDB 行协议）：

```
测量名,tag键=tag值 字段=值[,字段=值...] [纳秒时间戳]
```

iotp 规则：

- **唯一合法 tag 是 `device_id`**（整数 = 设备表 id）；其他 tag 或完全无 tag → 该行拒收；
- 类型语法：`i=42i`（I64 后缀 `i`）、`f=0.5`（无后缀带小数 = F64）、`b=true`（Bool）、`s="x"`（String）；
- 时间戳：可选显式纳秒整数；缺省 = 服务端当前时间；
- 一行一个测量；多行用换行分隔；批量中坏行拒收、好行照常写入。

```bash
curl -X POST "http://127.0.0.1:8080/write?db=metrics" \
  -d 'cpu,device_id=1 i=42i,f=0.5,b=true,s="ok" 1759000000000000000
      mem,device_id=1 usage=0.8'
```

### 5.2 JSON 信封（HTTP）

`POST /write?db=<db>`，Content-Type `application/json`（或 `application/*+json`；大小写不敏感，`;` 参数自动剥离）。

```json
{"measurement": "cpu", "tags": {"device_id": 1},
 "fields": {"i": 42, "f": 0.5, "b": true, "s": "x"},
 "timestamp": 1759000000000000000}
```

- 顶层数组 = 批量，元素独立处理（坏元素拒收、好元素照常写入）；
- `tags` **必填且必须含 `device_id`**（正整数；只允许这一个键）；`fields` 必填非空；`timestamp` 可选纳秒整数（缺省 = 服务端 now；负数拒收）；
- **严格类型矩阵**：JSON 整数 → I64、浮点 → F64、`true/false` → Bool、字符串 → String；`null`/数组/嵌套对象/超 i64 范围整数/含控制字符（`\n` `\r` `\t` `\0`）的字符串 → 拒收该元素；字符串数字（`"42"`）**不解析**，按字符串写入（若模式类型是 I64 则由无损转换拒收该行）；
- 被翻译层拒收的元素计入响应顶层 `rejected_rows`，但**不**计入 per-table `tables.*.rejected`（两者差值为翻译层跳过数，属正常现象）。

### 5.3 MQTT 采集

启用 `[mqtt]` 后，命中 `[mqtt.ingest]` 规则主题的 PUBLISH 被采集；其余主题正常由 broker 转发、不采集。

- **`format = "lp"`**（简写缺省）：载荷与 HTTP LP 完全同构（自带 `device_id` tag），消息体零转换进写入管线；
- **`format = "json"`**：载荷为 JSON 信封（§5.2 同契约，数组可批量）；
- QoS 0/1 均支持；**QoS ack 独立于写入结果**（ack = 消息到达 broker；持久化以 WAL 为准）；
- **retained 消息不采集**（broker 正常保留转发）；
- 计数（`GET /api/v1/meta/mqtt` 每规则）：`accepted`/`rejected` 按**行**、`dropped` 按**消息**（消费队列满丢弃，不阻塞 broker）；翻译失败（全坏 JSON）按 1 条消息计 rejected；每规则另有 `reasons` 拒收原因分解（见 §7.3），排查"rejected 为什么涨"先看它；
- 消息推送示例（mosquitto 或任意客户端）：

```bash
mosquitto_pub -h 127.0.0.1 -p 1883 -u iotp -P changeme \
  -t "devices/1/data" \
  -m 'cpu,device_id=1 i=42i'

mosquitto_pub -h 127.0.0.1 -p 1883 -u iotp -P changeme \
  -t "devices/1/json" \
  -m '{"measurement":"cpu","tags":{"device_id":1},"fields":{"i":43}}'
```

### 5.4 响应与错误语义

`POST /write` 响应（200）：

```json
{"accepted_rows": 2, "rejected_rows": 1,
 "tables": {"metrics.cpu": {"accepted": 2, "rejected": 1, "reasons": {...}}}}
```

- 200：部分接受（好行写入，坏行拒收 + warn 日志）；可解析但全部被 schema 拒收时仍是 200 + `accepted_rows = 0`；
- 400：空 body / 所有行都无法解析（含 JSON 元素全部翻译失败）；
- 503：WAL 写入失败（整请求原子性：本请求无任何行落盘）；
- 413：body 超 `max_body_bytes`。

---

## 6. 查询

通过 SQL 查询历史数据时，应先明确查询模式、时间范围和返回行数上限。刚创建且尚未形成首个 Parquet 快照的表存在可见性限制，详见本章与第 10 章。


`POST /api/v1/query`，body `{"sql": "...", "mode": "all" | "history"}`，mode 缺省 `all`：

- **all** = 内存 buffer ∪ 已落盘 parquet（含最近未落盘数据；union 两侧为计划时刻一致快照，不重不漏）；
- **history** = 仅已落盘 parquet；
- 查询列名即字段名：`time`（纳秒时间戳，INT64）、`device_id`、测量字段、标签字段（自动附加设备标签值，按 `device_id` 映射）；
- 结果行数**达到** `max_rows` → `truncated: true`（恰好等于也标）；未知表在 history 模式会报 table not found（尚未落盘过）；
- 响应字段：`rows`（行数组）、`truncated`、`returned_rows`、`elapsed_ms`（**服务端查询耗时**，毫秒，不含网络；与查询超时/RSS 护栏同源可比）、`message`（截断/异常提示）；
- **标识符大小写敏感**：db/表/字段名在 SQL 里必须与创建时完全一致（`SELECT * FROM huTest.elec` 与 `hutest.elec` 是不同引用，后者报 table not found）。名称本身可含大写字母，但不会按 SQL 惯例折叠成小写匹配。
- **查询内存保护**：排序/聚合超 `query_memory_limit` 溢写磁盘；执行计划含交叉连接（`FROM t, (SELECT ...)`）且涉及数据量超过该上限时**执行前拒绝**（422，附数据量估算与改写建议——等值 JOIN、加时间过滤、两步查询）；执行中内存耗尽会返回带指引的中文提示。`EXPLAIN SELECT ...` 可查看执行计划（对 EXPLAIN 不附加行数限制）。
- **按非时间列排序必须带时间范围**（2026-09-14 起，见下）：按 `Ua` 这类测量列排序时，即使写了 `LIMIT`，引擎也必须看过**每一行**才能确定前 N 行 —— 也就是说 `LIMIT` 完全不减少扫描量。实测（生产 6.3GB 表）：`ORDER BY time LIMIT 100` 峰值 RSS **+2MB**，`ORDER BY usage LIMIT 100`（无时间范围）**+2.5GB 并被 OOM 击杀**，加 1 小时时间窗后 **+0MB**。**判据是有没有时间范围，不是有没有 LIMIT。** 数据量超过 `query_memory_limit` 时这类查询会被**执行前拒绝**（422），报错给出三条出路：① 加时间范围（`WHERE time > <最近时刻>`）② 改 `ORDER BY time` ③ 取最新 N 条用 `ORDER BY time DESC`。注意这类内存**不在查询内存池的记账范围内**（池上限 1536MB 时实测 RSS 2.87GB），所以 `rss_high_water` 这类只拦新查询的护栏挡不住它 —— 唯一可靠的做法是加时间范围。**例外**：排序位于**聚合子树**内时护栏放行（如 `GROUP BY device_id ORDER BY count(*)`）——聚合已把行数收敛到分组数；若分组数本身很大，仍请加时间范围。**窗口函数同理**（2026-09-15 起）：`row_number() OVER (ORDER BY <非时间列>)` 这类写法里的 `ORDER BY` 不在 SQL 末尾，但引擎同样要看过每个分区的每一行才能产出第一行，护栏一样会拦；`OVER (ORDER BY time)` 与不带 `ORDER BY` 的窗口（如 `count(*) OVER ()`）不受影响。
- **采集、快照、查询、合并四条负载各占一条专用线程池**：`iotp-ingest`（MQTT forwarder/consumer + WAL fsync，缺省 2）、`iotp-snapshot`（parquet 编码 + 写盘，缺省 1）、`iotp-query`（查询，缺省 核数-1）、`iotp-merge`（合并，固定 1——轮次串行是安全属性，见 §9.9）。关键背景：代码里不直接调 `spawn_blocking`，但 WAL 写盘走 `tokio::fs`（其内部会），落的是**该 runtime 的 blocking 池线程**；其余阻塞工作（快照的 parquet 写盘 `std::fs` + fsync、SQLite、MQTT 的 LP 解析）则是**直接占用 worker**。所以「跑在哪个池」基本等于「阻塞谁」。分开之后，快照写盘不再顶掉 HTTP 的 worker，MQTT forwarder 也不会因为一次 LP 解析或写盘而答不上 broker 的 pause/ready。线程名可在 `top -H` / `/proc/<pid>/task` 里直接看到。
- **查询与采集互不拖累（2026-09-11 起）**：查询的规划与执行跑在**专用线程池**上（`[query] worker_threads`，缺省 = 核数-1，给采集留一核），慢查询不再占用采集（MQTT/HTTP 写入、WAL 落盘、快照）所需要的线程——此前"查询越慢、写入越卡"的耦合已断开。同时：查询超时现在**包含排队时间**（池被占满时会更快返回超时，行为与之前一致只快不慢）；客户端断开或超时会**立即取消**这次查询并释放资源（旧行为是继续跑完）。**小文件合并不在这条池上**（0.6.0 起走专用 `iotp-merge` 池，§9.9）：旧的"合并周期内查询排队到超时"的代价随之下线。代价是合并现在与采集、查询**共享同一个 cgroup**：一轮的内存计入服务的内存上限，内存紧张时 `rss_high_water` 会照样 503 新查询（采集不受影响）。
- **查询性能（2026-09-10 起）**：`time`/`device_id`/测量字段谓词会下推进 parquet 扫描。**带时间范围**的查询享受两层裁剪：文件名区间裁剪（窗口外的文件零 I/O，连 footer 都不读）+ footer 统计裁剪（行组级）——"查某设备最新 N 条"请写成带时间窗的形式（如 `WHERE device_id=13 AND time > <最近1小时的纳秒时间戳> ORDER BY time DESC LIMIT 100`），大表上毫秒~亚秒级返回。**无时间条件**的宽表 `SELECT *` 仍需解码全部列，大表容易触发 `query_timeout_secs`（422 Query timed out）——加时间窗或收窄投影列是正解。**写法要求（2026-09-15 实测）**：时间窗要写成 `time > 字面量`（`> >= < <=` 皆可，字面量在左或右都行），并且**用 `AND` 组合**——`OR` 连接的多段窗口现在也能裁剪（并集界），但**任一段无界则整体不裁剪**（`time>100 OR time<50` 覆盖整条时间轴）；`IN`、`BETWEEN` 靠优化器改写，实测 `BETWEEN` 可用而 `IN` 不可用。判据是"谓词能否被识别成 time 与整数字面量的比较"，识别不出就退化成全量扫描。生产实测：`(time>a AND time<b) OR (time>c AND time<d)` 从不裁剪的 52ms 变为 1ms 级。**标签列谓词也会被改写下推**（2026-09-10）：标签列不在 parquet 文件里（查询期按 device_id 附加），引擎在计划期查 devices 表，把**正向等值/IN**标签谓词翻译成 `device_id IN (...)` 交给扫描——`WHERE ECategory='生产用电'`、`WHERE region IN ('east','west')` 与手写 `WHERE device_id IN (...)` 享有同样的**裁剪方式**（文件/行组统计 + 解码期行过滤），上层仍保留原标签谓词兜底，结果逐行不变。**收益取决于标签选择性**（2026-09-10 在生产表约 5.5M 行上实测）：选择性标签显著变快——19 台设备的标签查询 304→144ms（约 2.1x），80 台设备 310→259ms；但**近乎覆盖全表的标签是净亏**：`ECategory='生产用电'` 命中 272/287 台设备（约 99% 的行），解码期逐行判定耗 ~72ms 却只滤掉 0.9% 的行，该查询由 356-368ms 变为 412-419ms。同等条件下手写 `device_id IN (...)` 仍最快（约 198ms，省去查询期标签列附加），对性能敏感的场景请优先按 device_id 过滤。`!=`、`IS NULL`、`NOT IN`、`LIKE` 等形态仍在上层过滤（其补集包含"标签未赋值/设备已删除"的行，无法用 device_id 集合枚举）。改写按**计划时刻**的设备标签计算，与 `PUT /devices/{id}` 改标签并发时，该次查询可能看不到新值对应的行（下次查询自愈）。

```bash
# 单设备聚合（含标签列）
curl -X POST http://127.0.0.1:8080/api/v1/query -d '{
  "sql":"SELECT device_id, region, COUNT(*) n, AVG(f) af
         FROM metrics.cpu WHERE device_id = 1 AND time > 1759000000000000000
         GROUP BY device_id, region",
  "mode":"all"}'
```

辅助表 `devices`（含 `device_id`/`name`/`table`/标签列，可 JOIN）：

```sql
SELECT d.name, c.i FROM metrics.cpu c JOIN devices d ON d.device_id = c.device_id
```

---

## 7. 状态与统计

本章介绍运行状态、设备最新值、MQTT 指标，以及告警、窗口聚合和 WebSocket 实时控制。建议先通过状态端点确认服务工作正常，再排查单个功能模块。


- `GET /health` → `200 "ok"`；
- `GET /api/v1/version` → 二进制版本 + **当前版本**的变更条目：

```json
{"version": "0.9.4", "date": "2026-11-07",
 "items": ["…当前版本的变更条目（编译期内嵌自 CHANGELOG.md）…"]}
```

说明：版本源是 `Cargo.toml` 的 `version`（与 `iotp --version` 一致）；`items` 是仓库根 `CHANGELOG.md` 中**与当前版本匹配的那一段**（编译期内嵌，段头版本必须与 `Cargo.toml` 一致，有测试兜底），历史版本不出现在响应里（控制台也不需要，完整历史看仓库文件）。发版时 bump 版本号并在 CHANGELOG 顶部加一段即可。

### 7.1 运行状态

`GET /api/v1/status` — 进程与机器运行时状态（process 各值来自 1 秒采样器）：

```json
{"pid": 12345, "version": "0.9.4", "uptime_secs": 3600,
 "process": {"rss_kb": 715000, "cpu_percent": 3.2, "threads": 9},
 "system": {"mem_total_kb": 3670016, "mem_available_kb": 2450000,
            "loadavg": [0.1, 0.2, 0.3]},
 "collection": {
   "buffer":  {"bytes": 262144000, "rows": 81234, "tables": 14},
   "wal":     {"pending_ops": 3, "files": 5, "dir_bytes": 12345},
   "parquet": {"files": 210, "dir_bytes": 890000000}},
 "mqtt": {"enabled": true, "clients_connected": 3, "messages_total": 152030,
          "accepted": 150000, "rejected": 2000, "dropped": 30}},
 "query_guard": {"active": true, "high_water_kb": 2097152,
                 "rss_kb": 715000, "queries_rejected": 0}}
```

说明：`process.cpu_percent` 为最近 1 秒瞬时值；`system.mem_available_kb` 是 OOM 预警的关键数字（可用内存）；**非 Linux 平台** `process` 各值与 `system` 为 `null`；MQTT 未启用 → `{"enabled": false}`。`query_guard` 为 RSS 高水位查询护栏（§3 `[query]`）：`rss_kb` 达到 `high_water_kb` 时新查询返回 503（`queries_rejected` 计数），采集路径不受影响。响应里还有两个引擎计数段（内存计数、重启清零；引擎未启用时是 `{"active": false}`）：`alerts`（§7.4）与 `rollup`——后者是 `active` + 十个计数（`ticks` / `windows_computed` / `empty_windows` / `blocks_deferred` / `rows_attempted` / `rows_written` / `rows_rejected` / `query_failed_blocks` / `truncated_blocks` / `reload_failed`），逐规则的运行状态（水位/积压/上次评估）在 `GET /api/v1/rollup/state`（§7.5）。响应顶层还带 WS 控制面与审计出口计数（2026-11-02）：`ws_connections` / `ws_subscriptions` 是**当前值**，`ws_pushes` / `ws_push_dropped` / `control_writes` / `control_timeouts` / `control_rate_limited` / `control_response_dropped` 与 `audit_forwarded` / `audit_forward_failures` / `audit_outbox_overflow` 是累计计数，`audit_outbox_pending` / `audit_outbox_oldest_age_secs` 是节点 outbox 当前快照 —— 口径与集群聚合规则见 §9.13。此外：`query_early_stop` 是「最新 N 条」前缀早停的常开计数器（`applied` / `fell_back`，§6）；`cluster` 段是集群状态（`active` / `role` / `lag` / `apply_skipped` / 分片图等，独立模式为 `active: false`）；`collection.wal` 还带启动期只读的回放去重统计 `replay_batches_skipped` / `replay_rows_skipped` / `replay_covered_tables`（§9.1、§9.8）。

### 7.2 设备实时值

`GET /api/v1/realtime/{device_id}` — 指定设备实例所有测量字段的最新值：

```json
{"id": 5, "db": "metrics", "table": "cpu", "name": "d1",
 "tags": {"region": "east"}, "tags_str": "region=east", "gateway": "gw-01",
 "fields": [{"name": "cpu", "type": "F64", "value": 0.9, "time": 1759000000000000000}, ...],
 "updated_at": 1759000000000000000}
```

说明：值为**最新一次上报**（内存缓存，WAL 重启回放自动重建；快照不清理），字段按声明顺序，`time` / `updated_at` 为**纳秒**（各字段自身最近上报时间）；设备从未上报 → `fields: []`、`updated_at: null`；设备不存在 → 404。内存占用 = 每（设备 × 字段）一条目，可忽略。

`GET /api/v1/realtime?device_ids=5,6,7` — **多设备批量**（看板/列表页一次拉多台）：

```json
{"devices": [{"id": 5, "db": ..., "fields": [...], "updated_at": ...}, ...],
 "errors":  [{"id": 99999, "error": "device id 99999"}]}
```

说明：`device_ids` 逗号分隔，也接受重复参数（`?device_ids=5&device_ids=6`）；id 去重、按**首次出现顺序**返回；单次上限 **1000** 个。不存在的 id 进 `errors`（HTTP 仍 200），其余设备照常返回——个别设备被删不会废掉整次请求；请求本身格式错误（缺参数、空项、非正整数、超上限）→ 400。每个 device 对象与单设备端点结构完全一致（多表设备混查时各自带自己的 `db`/`table` 与字段集）。

### 7.3 MQTT 统计

`GET /api/v1/meta/mqtt`（MQTT 启用时；权限 **Admin**，与其余 `/api/v1/meta/*` 同档 —— 与 `alerts/state`、`rollup/state` 的 `Query` 档不同）：

```json
{"enabled": true,
 "listeners": [{"protocol": "tcp", "port": 1883, "tls": false}],
 "clients_connected": 3,
 "online_clients": [{"client_id": "gw-01", "principal": "gw-01", "clean": true,
                    "subscriptions": ["iotp/hubin/#"]}],
 "messages_total": 152030, "bytes_total": 12345678,
 "ingest": [{"topic": "devices/+/data", "db": "metrics", "format": "lp",
             "accepted": 150000, "rejected": 2000, "dropped": 30,
             "reasons": {"type_mismatch": 1900, "unknown_table": 0,
                         "unknown_field": 0, "unknown_device": 0,
                         "unknown_tag": 100, "dropped_field": 0,
                         "not_owner": 0,
                         "translate_error": 0, "json_skipped": 0,
                         "write_error": 0}}],
 "acl": {"enabled": true, "denied_total": 3,
         "denials": [{"principal": "gw-01", "topic": "devices/+/write",
                      "subscribe": true, "count": 3, "overflow": false}]}}
```

说明：`clients_connected` 经 rumqttd meters 链路（安静 broker 上可能滞后）；`online_clients` 为**在线客户端明细**：router 每个 metrics tick（约 1 秒）全量重建，故**陈旧度 ≤ 1 秒**；进程内链路（`iotp-consumer` 采集、`iotp-alerts` 投递、`iotp-command` 控制命令）不计入；未出示身份时 `principal` 为 `null`；行按 `client_id` 排序。它与 `clients_connected` **可能对不上** —— 后者只在有发布活动后才刷新，**以明细为准**；`messages_total`/`bytes_total` 为**采集侧**计数（仅规则命中的发布，非 broker 全量）；`reasons` 为 `rejected` 的原因分解（十键恒定输出，含零值）：前七键与写路径拒收原因一致（跨表求和；`not_owner` = 集群归属拒收，见 §9.11），`translate_error` = JSON 信封翻译失败（整条消息）、`json_skipped` = 翻译层跳过的批量元素（按行）、`write_error` = WAL 失败/无有效行；MQTT 未启用 → `{"enabled": false}`。`acl` 是主题 ACL 越权计数（内存态、重启清零）：`denied_total` 永远准确；`denials` 按 (主体, 主题, 动作) 聚合、**键数上限 256**、主题串截断到 96 字节，超出并入 `overflow` 桶；ACL 未启用（`[auth] enabled = false`）时是 `{"enabled": false}`。

### 7.4 告警引擎

`[alerts]`（§3）打开后，进程内按 `tick` 巡检每台设备的最新值，**状态变化**时产生一条告警：
**边沿触发**，稳态不重复报 —— 所以"表里没有新行"等于"状态没变"，不是引擎停了。

**规则的维护**（2026-09-27 起规则住在 SQLite 里，**不在配置文件里**）：`{data.dir}/iotp.db`
的 `alert_rules` / `alert_rule_conditions` / `alert_rule_tag_filters` 三张表，**在控制台的
「告警管理」页维护**（新建 / 编辑 / 删除；`sqlite3` 直接改库同样有效）。每条规则**恰好给**
`conditions`（阈值，多条件为 **AND**）或 `stale_after`（失联）之一，另有 `severity`
（`warn` / `critical`）与 `for_ticks`（连续 N 个 tick 为真才报，缺省 1）。

等价端点：`GET/POST /api/v1/meta/alerts/rules`、`PUT/DELETE /api/v1/meta/alerts/rules/{id}`
（权限 Admin）。规则对象字段：`id,name,table,severity,enabled,for_ticks,conditions,tag_filter,stale_after`；
`tag_filter` 是 `{字段:[取值,…]}`（同一字段多值为 OR、不同字段之间为 AND）。

控制台表单（2026-10-02 起）：**源库/源表是两个联动下拉**（候选 = 元数据里已有的库与表），
条件的**字段**、标签筛选的**字段**也各是下拉 —— 分别只列源表的**测量字段** / **标签字段**
（候选就是这一步装配校验会用的那份元数据）；**阈值与标签取值仍是手写**（那是数据不是
schema）。规则引用了一个已经被删掉/被 drop 的表或字段时，下拉里会**多出一个标着「已不在
表中」的选项**把原值留着 —— 表单不擅自改写规则，能不能存由服务端按同一条判据说（与窗口
秒数、算子两处同一个纪律）。

- **改动立即生效（≤1 个 tick），不需要重启**：引擎每个 tick 比对一次规则集的版本号，变了
  就重新装配。用 `sqlite3` 改库也生效 —— 最多 5 秒后重新读一次（与访问控制那条路同款）。
  删掉一条规则，它的判定状态同时清零（**再建同名的规则会重新播报一次**，见下面那条）。
- **改一条只影响那一条**：判定状态按**规则名**索引，编辑一条不会动到别的规则的状态
  （同名整体替换、状态延续）。
- **一条规则改错了不会连累别人**：装配时逐条对着权威 schema 校验（表/字段存在、类型
  相容、`conditions` 与 `stale_after` 恰给一个、`op`/`severity` 认识，以及**源表的标签
  能照抄进告警表** —— 标签与告警表的固定列同名、类型不是 String、或那个名字在告警表里
  被 drop 过，都只废掉**引用它的那一条**），对不上的那条
  **只跳过它自己**，**原因就列在「告警管理」页的规则表格下方**（也进日志与
  `GET /api/v1/alerts/state` 的 `rejected` 段）。它不会告警，直到改对为止 —— 页面上
  "规则在列表里、却永远不告警"是**有解释**的，别当成引擎坏了。
- **条件的顺序 = 插入顺序**（编辑规则 = 删旧条件 + 插新条件，顺序就是表单里的顺序）；
  **多条件规则报告的是第一个条件的观测值** —— 它进告警表的 `value` 列与 MQTT 载荷。
  判定与顺序无关（AND 可交换），**报告**与顺序有关：`Ua>250 AND Ia>100` 触发时 `value`
  记的是 `Ua`。想让哪个字段的值出现在 `value` 里，就把它写在第一个条件。
- `op` 支持 `>` `>=` `<` `<=` `==` `!=`；`value` 的类型要与字段类型相容
  （`I64`/`F64`/`Bool`/`String`）。**`I64` 字段的阈值必须是整数值**：写 `250.5` 会被
  **拒绝**（不是截断、也不是放行）。
- **数值一律按 f64 比较**：`I64` 字段的观测值与阈值都会先转成 f64，所以**超过 2^53 的
  整数会丢精度**（`==`/`!=`、以及贴着阈值的 `>`/`<` 都可能判错）。IoT 量级上够不着；
  真要判那个量级的相等，别用 `==`，改用带余量的区间。
- **失联按"到达时间"判，不按事件时间**：设备时钟冻住（一直在写、但 `time` 是旧值）不会
  被误报失联。
- **规则名**：不得含 `/`、`+`、`#`，**不得含不可见字符**（零宽空格、BOM、软连字符这些：
  它们既不是空白也不是通配符，但会让 `ua_high` 与 `ua_high` 看起来一模一样、主题却是
  另一个），**首尾也不许有空白**（内部的空白合法，如 `ua high`）
  —— 它会原样进 MQTT 主题 `iotp/alerts/<规则名>/<源设备 id>`：`+`/`#` 是主题通配符、
  `/` 分层，而首尾空白会让主题里多出一个空段 —— 订阅方照界面上看到的规则名去订，
  就会一条也收不到（建/改规则时这些形态会被**拒**，不是替你改写）。**改名 = 主题改名**
  （订阅方要跟着改）；名字同时是判定状态的键，所以改名 = 删旧建新：那条规则的状态从头
  开始，**可能重播一次**。
- 规则引用的表/字段必须在**装配时**对得上（strict-only：逐条对着权威 schema 校验）。
  对不上的那条只被跳过 —— 所以"先建规则、再建表"不会报错，只是表建好之前它不告警
  （页面上那条 rejected 会一直挂着原因）。
- **标签筛选（2026-09-28，可选）**：一条规则可以只盯**一部分设备** —— 表单里**一行一个
  取值**（字段下拉选 `ECategory`、取值写 `生产用电`，再来一行 `ECategory` 写 `公共用电`），
  **同一字段写多行是「或」、不同字段之间是「且」**；不写 = 盯源表的**全部设备**（老规则一字
  不变）。取值的字符集不受限：**含逗号与 `=` 的取值照写**（一行就是一整串，没有分隔符这回事）；
  **含换行**的取值输入框显示不了 —— 只要不改那一格，保存时**原样写回**（改别的字段不会把它
  悄悄改掉），这类取值建议用 `/api/v1/meta/alerts/rules` 建/改。字段必须是源表的**标签字段**（写错名字
  或写了测量字段，只废掉这一条规则，原因列在规则表格下方）；每 tick 从元数据重算，所以
  给设备**改标签 ≤1 个 tick 生效**（被筛出去的设备，它的判定状态同时清零，再筛回来会
  重新播报一次）。**筛不到任何设备是合法的**：规则照常在跑、只是永远不告警 —— 这时
  「告警管理」页那一行的状态列会写**「0 台设备匹配」**（`GET /api/v1/alerts/state` 的
  `devices_matched`，口径是**上一次跑完的 tick**，`null` = 引擎还没为它巡检过），
  别把这种沉默当成引擎坏了。

**启动期 vs 运行中**：上面这套"改一条只影响那一条、≤1 tick 生效"说的是**运行中**的改动。
**接线期**（服务启动时）的失败是另一回事：告警表 `provision` 失败或调度器装配失败时，
**整个引擎**在这一个进程生命周期里都不启用（`/api/v1/status` 与状态端点都回
`active: false`，原因在启动日志里的一条 ERROR）—— 修好之后要**重启**才恢复告警。
**运行中**的重载失败（同样是 `provision` 告警表那一步失败，比如告警表的列被改坏了）
**不会让巡检停摆**：引擎**保留上一份规则集继续判定**，`/api/v1/status` 的
`reload_failed` 计数、`/api/v1/alerts/state` 的 `reload_error` 给出原因，下一个 tick
自动重试 —— 此时「告警管理」页会在顶部明说"页面上这一版是上一次成功装配的规则集"。

**告警表**（`out_db.out_table`）：引擎在**第一次有规则的那次重载**里自己创建（启动时
已有一条规则 ⇒ 就是接线期；否则是建出第一条规则之后的下一个 tick）；已存在则校验既有列的形状，并把
**缺的源表标签列补上**（加宽只加标签列：元数据层本来就允许随时新增标签字段），schema 固定：

| 列 | 类型 | 说明 |
|---|---|---|
| `time` | 时间戳 | 状态转换的时刻（服务器时钟） |
| `device_id` | 保留列 | **镜像设备**的号 —— 引擎为每个源设备在告警表里建一台镜像设备（名字 `src:<源表>#<源设备 id>`，标签照抄源设备） |
| `src_device_id` | I64 | **源设备**的号 —— **查"哪台设备在告警"用这一列** |
| `rule` | String | 规则名（测量字段：同一台设备的同一字段可以被多条规则盯上） |
| `severity` | String | `warn` / `critical` |
| `state` | String | `firing` / `cleared` / `stale`（`stale` 只由失联规则产出） |
| `value` | F64 | 阈值规则：触发的观测值；失联规则：**距上次到达的秒数**（失联的"多久"就记在这一列）；`cleared` 事件：**NULL**（恢复没有观测值可言） |
| `message` | String | 一句话（含规则名、设备、数值/时长） |
| 标签列 | String | `src_table`（镜像来自哪张源表，`db.table`）+ **源表的标签字段**（照抄，所以 `WHERE region='east'` 照样能用） |

```sql
-- 当前谁在告警：每个（规则, 源设备）取最后一行（表只追加，不能 UPDATE）
SELECT rule, src_device_id, state, value, time FROM ops.alerts
 WHERE time > CAST(now() AS BIGINT) - 3600000000000 ORDER BY time DESC LIMIT 100;
```

> 时间界要写成 `CAST(now() AS BIGINT) - <纳秒>`（或直接给一个纳秒字面量）。写成
> `now() - 3600000000000` 在查询引擎（DataFusion 40）上**规划期就报 422**
> （`Cannot coerce arithmetic expression Timestamp(Nanosecond, …) - Int64`）——
> `now()` 是 `Timestamp`，减整数不合法。上面这条是照抄可用的形式。

> ⚠️ **两个设备号是两回事**：`src_device_id` 是**源设备**的号（你写入时用的那个），
> `device_id` 是告警表里那台**镜像设备**的号。**查询源设备一律用 `src_device_id`** ——
> 镜像 id 只存在于告警表内，拿它去源表查没有意义。写操作不用管镜像：引擎自己建。

> ⚠️ **缺的标签列引擎会自己补**（2026-09-27 起）：标签列由引擎照抄源表 + 自己的
> `src_table` 组成，规则一变（盯上了另一张源表、或源表新增了一个标签字段）就有新列要加
> —— `provision` 每次重载都会把缺的**标签**列 `add_field` 补上（元数据层本来就允许随时
> 新增标签字段，无回填、既有实例为 NULL），所以**不用删表重建**，也不用重启。
> **测量列不在此列**：告警表的测量列是固定的那六个。既有列被人工改成别的类型 / 别的
> `is_tag` 形状时，引擎照样报错（接线期 ⇒ **禁用告警 + 要重启**；运行中 ⇒ 那一次巡检
> 失败、规则集不换）—— 写进去的行不会被任何查询认出来，而改既有列会动到已落盘的行。
> 处置是把那张表改回本引擎的形状（或者确认它确实是本引擎的表）。

**「最近告警」（控制台里看那张表，2026-09-28）**：`GET /api/v1/alerts/recent`
（权限 `Query`，与状态端点同档）是上面那张表的**只读**视图 —— 已经发生过的**状态转换**
一行一次边沿，**最新在前**（`ORDER BY time DESC`），缺省 100 条、上限 1000。

- 只要两个查询参数：`limit`（正整数；**超上限封顶**到 1000，而 `0`/非数字一律 400）与
  `rule`（**必须是当前生效的规则名** —— 拼错的、注入串、被拒或被禁用的规则名一律 400）。
  SQL 由**服务端**拼：浏览器只给这两个标量，规则名在服务端**既过白名单、又把单引号
  翻倍**（两道闸，见 `alerts_state_api.rs` 的模块头）；
- 走 `mode=all`：还压在内存缓冲里、没落盘的行照样看得见（"最近"漏掉最后几分钟就是错的），
  代价是与普通查询共用同一套护栏（并发槽位、超时、RSS 高水位 → 503）；
- **"还没有告警过"不是错误**：告警表还没建出来（一条规则都没生效过），或建出来了却
  一行都没有（刚 provision）时，回 `{"active": true, "table": "ops.alerts", "rows": []}`
  —— 空数组，不是 422。引擎没起来时是 `{"active": false, "table": null, "rows": []}`
  （与状态端点的 `active: false` 同一个口径），页面据此说"引擎未启用"；
- 控制台上：这张表在规则表格**下方**，随「告警管理」页每 3 秒刷新，且**独立降级**（它挂了
  只在那一块显示一行字，规则列表照常画）。点某条规则那一行的**状态**格 = **只看这条
  规则**的最近告警（表上方出现「筛选：<规则名> ✕」；点 ✕、或再点一次那条规则的状态格
  都能清除）。只有**当前生效**的规则才给点 —— 被拒 / 已禁用的规则名端点会 400。

**订阅告警**：发布在 `iotp/alerts/<规则名>/<源设备 id>`，载荷是 JSON
（`rule` / `severity` / `state` / `from` / `device_id`（**源**）/ `device_name` / `tags` /
`table` / `value` / `age_secs` / `time`）。

```bash
# 网络订阅方要一条 mqtt_sub 规则（[auth] enabled 时），否则订不上
iotp mqtt-user new --name dashboard --allow 'mqtt_sub:topic:iotp/alerts/#'
# 本地验证
mosquitto_sub -h <host> -t 'iotp/alerts/#' -v
```

- 投递是 **QoS0、不 retain、至多一次**：没连上就收不到 —— **表才是留痕**。看板连上后先查
  一次上面那条 SQL 补当前状态，再转订阅（这是无 retain 的 MQTT 里的标准做法）。
- 载荷的 `time` 与表里那一行的 `time` 是**两次取时刻**（同一时钟、微秒级之隔），不是同一个
  数；要对齐以表里的为准。
- 引擎自己的发布**豁免 ACL**（进程内链路）；外部订阅方照常按 `mqtt_sub` 判权。
- **客户端不能往告警主题发**（伪造告警）：`[auth]` 开着时，**发布**到 `iotp/alerts`
  命名空间一律被拒（不用查规则 —— 哪怕主体的发布规则是 `mqtt_pub:topic:#`），拒绝进
  越权计数；**订阅**照常放行（上面那条 `dashboard` 规则就是这么用的）。伪造的消息在
  告警表里没有对应行 —— 表是唯一的事实来源，对不上账的告警就是假的。
  ⚠️ 与其余所有主题一样：`[auth] enabled = false` 时**整条 ACL 都不存在**，这条守卫
  同样缺席。
- **规则改名 = 主题改名**：订阅方要跟着改（或订 `iotp/alerts/#`）；改名还会让那条规则的
  判定状态从头开始（见上面「规则的维护」）。
- 任何 `[mqtt.ingest]` 规则能覆盖 `iotp/alerts/` 前缀 ⇒ **只禁用告警引擎**、服务照常启动，
  日志里有一条 ERROR 指名道姓报出是哪条规则并给出处置（改规则或关 `[alerts]`）。判据是
  **宁多报**的：`+/+/+` 这种实际收不到告警的规则也会命中。

> ⚠️ **重启后可能重新播报一次**。判定状态（每条规则的当前判定 + 去抖计数）只在内存里，
> 重启**不**对着告警表恢复；而判定只吃**采集缓冲里的最新值**，缓冲靠 WAL 重放重建 ——
> **已经快照过的设备重启后没有最新值**（值随快照落了盘），要**重新上报**才会被判定看到。
> 于是：一台重启后照常上报、条件仍为真的设备，会**再产生一条 `firing` 行与一次 MQTT
> 发布**，哪怕它一直在告警、状态从未变过（不是"每次重启必报"，故说**可能**）。
> 订阅方要按"重复的 `firing`"设计（幂等处理，或按 `time` 去重）；**"今天报了几次"这类
> 计数不能拿 `firing` 行数当次数** —— 重启会虚增（要数状态转换，按 `(rule, src_device_id)`
> 折叠相邻同状态的行）。

**状态与计数**：`GET /api/v1/status` 的 `alerts` 段（内存态，重启清零）。

```json
"alerts": {"active": true, "ticks": 8123, "transitions": 4, "rows_written": 4,
           "write_rejected": 0, "published": 4, "publish_failed": 0, "tick_errors": 0,
           "reload_failed": 0}
```

- `active: false` = 引擎没在跑：`[alerts] enabled = false`、接线期的装配 / provision 失败
  （例如告警表被人工改成了别的形状）、或回环守卫命中 —— 原因都在启动日志里（一条 ERROR）。
  ⚠️ **规则自己的毛病不在这一档**：源表/字段对不上的规则只被跳过（进 `rejected`），
  其余规则照常跑、`active` 仍是 `true`；
- `ticks` 与 `tick_errors` **互斥**（跑完的 tick 数 / 失败的 tick 数）；
- `reload_failed` 是**规则重载里的 `provision` 失败**次数（告警表的形状被改坏这类表级
  问题）。它**不算** `tick_errors`：那种失败不停摆（保留上一份规则集继续巡检），原因在
  `GET /api/v1/alerts/state` 的 `reload_error` 里 —— 与「告警管理」页顶部那句是同一件事；
- `transitions` 是判定层产出的状态转换数，`rows_written` 是真落进告警表的行数，差额进
  `write_rejected`（**不变量：`transitions == rows_written + write_rejected`** —— 一次巡检
   panic 掉时没记上账的那几条也会补进 `write_rejected`，所以这个差额永远有解释）；
- 一次巡检的**状态转换**只在 `transitions > 0` 时记一行 info 日志（稳态不刷屏）。

### 7.5 聚合（rollup）引擎

`[rollup]`（§3）打开后，进程内按 `tick_secs` 的节拍把每条规则的源表**按时间窗口**聚合成结果行，
写进规则自己的输出表（一条规则一张表）。规则住 SQLite，在控制台「聚合管理」页或
`/api/v1/meta/rollup/rules`（§4.7）维护，改动 **≤1 个 tick 生效**；一条规则装配不了（源表/字段
对不上）或输出表建不出来**只废掉它自己**，原因在 `GET /api/v1/rollup/state` 的 `rejected` 里 ——
页面上"规则在列表里、却永远不聚合"是**有解释**的，别当成引擎坏了。
控制台表单（2026-10-02 起）：**源库/源表是联动下拉**（必须是既有的表），**聚合的字段也是
下拉**（候选 = 源表测量字段里该算子接受的类型，`corr`/`regr_*` 那类要的第二字段同款）；
**输出库/表仍是输入框 + 候选提示** —— 输出表可以还不存在（引擎自动建），做成封闭下拉会把
"建新表"这条路堵死。引用了已不在的表/字段时，下拉里会多一个标着「不在候选中」的显式选项
把原值留着，表单不擅自改写规则（同告警表单那条纪律）。

**窗口与封口**：窗口按 **epoch 对齐**（起点 = `time - time % 窗口宽度`，即 1970-01-01 起的整数倍），
结果行的 `time` 是**窗口起点**，不是数据到达时刻。只有**已经封口**的窗口才会被写进输出表 ——
当前正在走的那个窗口在输出表里**查不到**，这是设计，不是延迟。

**延迟**：窗口封口后，引擎再等**宽限期**（`grace_secs`，覆盖设备补发/乱序）才算它，最坏再等一个
tick 落表：**总延迟 ≈ `grace_secs + tick_secs`**（缺省 60 + 60 ⇒ 1 分钟窗口约 **2–3 分钟**后出结果；
窗口越宽，这个绝对延迟不变、相对越短）。

**缺口（如实写，这是本引擎最大的坑）**：**封口之后才到达的行永久漏掉** —— 设备补发晚于
`grace_secs`、以及**冻结时钟**设备推上来的旧时间戳，都落在已经算过的窗口里。引擎**不重扫已封口
的窗口**，所以这种漏失**在引擎侧测不到**（"少了几行"与"本来就没有数据"在输出表里长得一样，
没有计数、没有告警），唯一的补救是人工 `rollup-once` 补算（见下）。窗口的内容在它被算的那一刻
**定格**。

**输出表**：每条规则的输出表（`out_db.out_table`）由引擎自己 provision（建库/建表/建列，**是元
数据写入**、幂等；范围里一行都没写出来也会先把表建出来）。schema = 固定测量列 `src_device_id`
（I64）+ 每个聚合的 `alias` 列 + **照抄源表的全部标签字段**（下游按任意源标签过滤照常可用）。
结果行经与写入同一条路径落盘（WAL → 缓冲 → 快照），查询方式与普通表完全一样。

| 分组方式 | 输出表的设备 | 行上的标签 |
|---|---|---|
| 按设备（`group` 省略 / `[]`） | **镜像设备** `src:<源表>#<源设备 id>`；`src_device_id` 列 = 源设备号 | 建镜像那一刻源设备的标签（见下） |
| 按标签（`group` 非空） | **合成设备** `grp:<字段=值&…>`（字段名排序、取值里的 `~`/`&`/`=` 转义、NULL 取值占位 `~n`）；`src_device_id` 不填 | 就是分组取值本身 |

- **两个设备号是两回事**（与告警表同款，§7.4 的那条提醒原样适用）：`src_device_id` 是**源设备**的
  号，查"哪台设备"用它；`device_id` 是输出表里那台镜像/合成设备的号，只在输出表内有意义。
- ⚠️ **镜像设备的标签是"建镜像那一刻"源设备标签的快照**：源设备之后改标签**不跟随**（要改就
  直接改输出表里那台镜像设备）。输出表的**标签列**则会在每次 provision 时**加宽补齐**（源表新增
  标签字段会自动出现在输出表上，无回填）。
- **输出表就是普通表**：快照、合并（`[merge]`）与保留期清扫都照常。保留期用的是**同一把全局尺子**
  （`[merge]` 的 `retention`）：删文件要**文件名的区间与 mtime 都**早于 `now - retention`（§3 的
  "按收到时间计龄"），而输出文件是窗口封口后 `grace + tick` 才写的（mtime ≈ 窗口末 + grace + tick），
  所以聚合结果与原始数据**同寿**、差额就是这个 `grace + tick` 的量级 —— 原始数据过期时，聚合结果
  也差不多该过期了；要留更久只能调 `[merge] retention`（全局，对所有表生效）。
- 某窗口**一行都没有**时不写行（空窗口是常态，不是故障），但进度照常推进；**不做 MQTT 投递**
  （与告警不同：结果只在表里）。

**失败、积压与截断**：查询失败 / 回写整批失败 ⇒ **该块不推进**、计数、下个 tick 重试
（同一规则的连续失败按 tick×2 退避、上限 10×tick）；结果被 `[query] max_rows` 截断、或查询撞上
`Resources exhausted`（内存池被聚合状态顶满）时，**整块作废、块减半重试一次**（截断的结果绝不写
半截表）—— 同一个 tick 内只减半一次，仍失败才落进"不推进"那档。块内窗口数按
"结果行数 = 窗口数 × 分组数上界（= 源表设备数）"自适应，下限一个窗口：
⚠️ **源表设备数一接近 `[query] max_rows`，块就被压到下限（一个窗口）；若单个窗口的结果行数仍超过
`max_rows`（按设备分组时，设备数 ≥ `max_rows` 的表在设备报数齐的窗口上必然如此），块已无处可减，
该规则会一直报截断、永远追不上**（warn 日志 + 状态端点的 `failures`/`last_error` + `/api/v1/status`
的 `truncated_blocks`）—— 处置是调大 `[query] max_rows`，或改成按标签分组（行数从设备数降到组数）。
追赶很长的停机缺口时，**每 tick 每规则最多算 4 块**，剩下的下个 tick 继续（进度不丢；还差多少看
`lag_secs`）。

**水位**：进度**不落状态文件** —— 调度器在进程内**第一次推进某条规则时**从输出表的 `max(time)`
推导一次（"最后写过的窗口的下一个窗口"），之后在内存里单调推进、不再回看输出表。所以进度是
**跟着输出表走**的：输出表被人工移走/删掉之后重启，水位就从"现在的"边界重新起算（**不会自动回补历史**，
要补就用 `rollup-once`；删表时已经写出的结果行也一起没了）。

`GET /api/v1/rollup/state`（权限 `Query`）逐规则给出：

```json
{"active": true,
 "rules": [{"name": "cpu_1m", "window_secs": 60,
            "watermark_ns": 1759245600000000000, "lag_secs": 0,
            "group": [], "aggregates": 2, "failures": 0,
            "last_error": null, "last_run_at_ns": 1759245660000000000}],
 "rejected": [{"name": "bad", "reason": "…窗口必须是 60–3600 秒且为 60 的整数倍，得到 59"}],
 "reload_error": null, "operators": ["avg", "max", "…"]}
```

- `watermark_ns` = 下一个该算的窗口起点；`lag_secs` = 积压秒数（水位落后边界多少，追赶时 > 0）；
- 三个时间字段**可以是 `null`**：`watermark_ns`/`lag_secs` `null` = 水位还没引导过（水位在规则进入
  活动集后的**下一个 tick** 才引导，`≤ 1 个 tick`，缺省 `tick_secs = 60` 时最长约 60 秒），
  `last_run_at_ns` `null` = 从未被评估过 —— **`null` 不是 0**："从未跑过"与"运行正常"
  （新规则的 `lag_secs` 也是 0）靠 `last_run_at_ns` 分得开；
- `failures` = **连续**失败数（成功一块就清零）、`last_error` = 上次失败的原因：不记的话"没窗口"与
  "窗口算不出来"在运维眼里一模一样。失败**不刷新** `last_run_at_ns`（它说的是"上次正常完成"）；
- `rejected` = 装配被拒 + 输出表 provision 被拒的规则名与原因；`reload_error` 非空时上面这份规则集
  是**上一版**（重载失败不换集）；`operators` 是算子白名单的静态元数据（控制台表单靠它渲染）。

`GET /api/v1/status` 的 `rollup` 段：`active` + 十个计数（`ticks` / `windows_computed` /
`empty_windows` / `blocks_deferred` / `rows_attempted` / `rows_written` / `rows_rejected` /
`query_failed_blocks` / `truncated_blocks` / `reload_failed`）。两条不变量：
`rows_attempted == rows_written + rows_rejected`；`windows_computed == empty_windows +
windows_with_rows`（`windows_with_rows` 没单列，就是前者减后者）。`active: false` = 引擎没起来
（`[rollup] enabled = false`，或接线期失败——原因在启动日志的一条 ERROR）；**规则自己的毛病不在
这一档**（只进 `rejected`，`active` 仍是 `true`）。全零而 `active: true` = 调度在跑、只是还没有
到期的窗口。

**补算（`iotp rollup-once`）**：引擎**不做自动重算**（引擎没有删行语义），缺口只能人工补：

```bash
# ⚠️ 先停服务（与 merge-once 同纪律）：服务内的调度器与手工轮次会算同一批窗口 ⇒ 重复行
sudo systemctl stop iotp
sudo -u <运行用户> iotp --config /etc/iotp/iotp.toml rollup-once \
  --from 2026-09-01T00:00:00+08:00 --to 2026-10-01T00:00:00+08:00 --rule cpu_1m
sudo systemctl start iotp
```

- `--from`/`--to` 接受 **RFC3339**（`2026-09-01T00:00:00+08:00`）或 **Unix 纪元纳秒整数**；区间
  **左闭右开** `[from, to)`；`--to` 必须晚于 `--from`。`--rule` 缺省 = 全部 enabled 规则。
- 对每条规则按窗口批量：查询源表（含内存缓冲在内）→ 写回输出表；**不碰内存水位**（水位的唯一
  写者是服务内的调度器）。被装配拒绝 / provision 拒绝的规则会被跳过并打印原因。
- 会 **provision 输出表**（与引擎同一条路，幂等；即使一行都没写出，表也已经建出来了）；命令
  **只看库里的规则**（`[rollup] enabled` 是不是 `true` 不影响这条命令）。退出码：**一条规则都没
  写出任何行 ⇒ 2**（"全部空跑"要能被脚本判成失败，与 `merge-once` 同款），部分规则空跑但至少
  一条写了行 ⇒ 0。
- ⚠️ **`--from`/`--to` 请对齐到窗口边界**（窗口起点 = epoch 对齐的整数倍，即对齐到"整窗口"）：CLI
  **不做对齐**，区间边界落在窗口中间时，那个窗口会被两侧**各写一行部分聚合**（块按区间切、
  `GROUP BY w_start` 只看得见本块内的行）—— 补出来的窗口**少算**。
- ⚠️ **补算前清理旧行是运维动作，引擎不做**：窗口按 epoch 对齐、**不去重**，同一范围跑两次 =
  **两遍行**。引擎没有删行语义（表只追加），但 `DELETE /api/v1/meta/tables/{db}/{table}` 有
  「空才能删」前置条件（表内无字段、buffer ∪ parquet 无数据），有聚合结果时必然 409，**没有
  force 参数**。当前可行做法：停服务后人工移走该表的 parquet 目录（或等 `[merge] retention`
  清掉数据文件），再删光字段、删表，然后重建并补算；代价是水位随表一起丢，引擎之后从**当前**
  边界重新起算，不会自动回补历史。
- 中途失败（查询出错 / 块被截断 / 回写被拒）⇒ 直接失败退出，本次已算出的行**也一并不落盘**
  （"要么整轮、要么没有"，重跑一次即可）。

### 7.6 实时推送与写控制（WebSocket，2026-11-02 起）

`[realtime] enabled = true`（§3）时，服务在 `ws://<host>:<port>/ws/` 提供网关控制协议：
**只有 JSON 文本帧**（不是 MQTT-over-WebSocket）。`[auth] enabled = false` 时不鉴权、
审计 actor 记为 `anonymous`（部署必须限制端口访问）；开启认证时见下面的「身份」一节。

#### 订阅与推送

客户端连接后发送：

```json
{"type":"read","ids":["13.E","13.Ua"]}
```

- `id` 按**第一个** `.` 切分：前段是正整数 `device_id`，后段整体是 measure 字段名
  （区分大小写，允许再含 `.`）；tag 字段、已删（tombstone）字段、非法/无权的 id
  只跳过并记日志，**不会断开连接**。
- 每条 `read` **整体替换**该连接的订阅集；空 `ids` = 清空。超过
  `max_subscriptions_per_conn` 时保留前 N 个并告警。
- **无 ack、订阅后不补发初始快照**：需要初始值的客户端先查 §7.2 的 realtime API。
- 推送触发是**每条被接受的行**（不是"值变化"）：同一 `process_lp` 内的多行按到达序、
  按连接合并为一帧，单帧最多 256 条，超出拆帧。告警/rollup 的回写同样会触发推送。
  WAL 启动回放**不**触发（不会在重启后重推旧值）。

推送帧：

```json
{"code":200,"data":[
  {"id":"13.E","status":"Good","val":"1","timestamp":1781157715},
  {"id":"13.Ua","status":"Good","val":"2","timestamp":1781157715}
]}
```

- `status` 恒为 `"Good"`（协议没有定义 `Error` 来源）；`timestamp` = 行事件时间（纳秒）
  `/ 1_000_000_000`，取整为 Unix 秒；`val` 为字符串：I64/U64 十进制、F64 `to_string()`、
  Bool `true`/`false`、String 原样（JSON 转义由 serde 处理）。
- 乱序事件按行推送，客户端按 `timestamp` 自行忽略旧值；服务端不按 latest 过滤。
- 慢客户端：出站有界队列满时**整帧丢弃**并计入 `ws_push_dropped`，不阻塞写路径、不断开连接。

#### 写控制

```json
{"type":"write","id":"13.E","val":"2"}
```

服务端顺序固定：每连接速率/在飞上限（读循环，超限 400、**无审计**）→ 资源授权（`control`）→
解析 id → 设备/表/字段校验 → `device_gateway` 映射 → 归属检查（`not_owner`）→ 值类型校验 →
网关在本节点且精确订阅 → 生成 seq + 写 `control_request` 审计（**写失败不下发**）→ 发布 MQTT →
等 reply → 写 `control_result` 审计 → WS 响应。

- 成功（网关 reply `code=200`）：`{"code":200,"data":"13.E","val":"2","message":""}`。
- 所有失败：`{"code":400,"data":"13.E","val":"2","message":"Write Error"}`；具体原因只进
  服务端日志与审计 detail。
- **200 只代表网关受理，不代表设备执行成功**；网关 reply 的非 200 / 未知 code 一律按失败处理。
- 权限动作 `control` 是**表级**资源（`table:<库>.<表>` 或 `db:<库>`）；每连接另有速率
  （`max_control_per_sec`，超限直接 400、无审计）与在飞上限（`max_pending_writes_per_conn`）。
- 客户端断开**不取消**已发布的命令：命令继续等 reply 并写结果审计，只是不再尝试发送 WS 响应。

#### MQTT 主题与网关契约

| 主题 | 方向 | 说明 |
|---|---|---|
| `devices/<gwId>/data` | 网关 → 服务 | 数据上传（现有采集，§5.3） |
| `devices/<gwId>/write` | 服务 → 网关 | 命令 JSON：`{"device_id":13,"fields":{"E":"2"},"gwId":"gw1","seq":"<32 hex>"}` |
| `devices/<gwId>/write/reply` | 网关 → 服务 | `{"seq":"<32 hex>","code":200}`；`code` 只表示受理。**只发布、不可订阅**（reply 由服务端本地链路消费） |

- 网关应**精确订阅** `devices/<gwId>/write`（不要用通配）；建议 QoS1 + `clean_session=false`
  以获得 broker 不重启、commitlog 窗口内的离线补投。**这不是可靠命令队列**：broker 重启、
  超出窗口、网关从未订阅都会丢；`write_timeout_secs` 超时后 WS 返回 400。
- **延迟执行风险**：网关用持久会话且离线时，命令可能被 broker 排队、在 WS 超时之后才执行。
  要求严格时效的网关应对控制主题使用 `clean_session=true`（或明确接受该语义）。
- QoS1 可能重复投递；`seq` 可用于网关侧去重，但协议没有把它定义为幂等键。
- 服务端发布走本地链路、固定 QoS0 + `retain=false`。同节点检查依据**本节点** broker 的
  在线客户端快照（每秒重建）：刚连上的网关最多 1 秒后可见 —— 控制客户端应重试。

#### 网关身份、共享 principal 与 client-id 绑定

- **gwId = 网关 MQTT client id**。`[auth] enabled = true` 时 broker 强制 client-id 绑定
  （2026-11-02 的 Patch 5，**无配置开关**）：
  - `devices/<gw>/data` 的发布者 client id 必须等于 `<gw>`；
  - 只有 client id 等于 `<gw>` 的连接才允许订阅 `devices/<gw>/write`；
  - `devices/<gw>/write/reply` 对该网关是**只发布**主题：网关只向它发布 reply，**任何网络
    客户端都不得订阅**回复命名空间（服务端 `iotp-command` 本地链路自己订阅、消费 reply；
    Patch 5 会拒绝覆盖该命名空间的网络订阅）；
  - 后果：**升级前必须核对每个网关的 client id 与其 data 主题里的 gwId 完全一致**。不一致的
    发布会被 ACL 拒绝（QoS1 仍回 PUBACK，表现为**静默丢数据**）—— 这是本版本接受的升级风险。
- 网关可以**共用一个 MQTT 主体**（共享 principal：口令/证书在整群共享，泄露影响面是整群），
  client id 是唯一的隔离键。共享 principal 下**自动盖章不可用**：所有可控制设备必须通过
  `device_gateway(device_id, gateway)` **显式映射**（§9.11 的 attach API 可一次完成
  声明/盖章/放置；也可用 `POST /api/v1/meta/devices/gateway` 逐台盖章），
  且不要把共享 principal 声明成 cluster gateway（否则设备会被盖到共享名下）。
- `[auth] enabled = true` 且使用 `mqtt_sub` 规则时，**覆盖控制命名空间的宽订阅一律拒绝**：
  `#`、`devices/#`、`devices/+/write`、`$share/.../devices/#` 等（订阅按"规则覆盖过滤器"
  判定，控制命名空间不允许被任何主体宽订阅；网关只能订阅自己的精确 `devices/<gw>/write`，
  **不能订阅 reply 主题**）。认证关闭时没有 ACL，控制只能靠网络隔离 —— 不推荐直接暴露。

#### 同节点要求（集群）

写控制**只在网关所在节点生效**，不做跨节点路由/重定向：

1. 归属集非空时要求本节点在 `device_owners(device_id)` 内（reason=`not_owner`），避免
   "控制成功但数据被归属校验拒绝"。
2. 在本节点 broker 的 `clients_snapshot()` 里找 `client_id == gwId` 的在线连接，并要求它
   **精确订阅**了 `devices/<gwId>/write`；否则 400（reason=`gateway_not_local` /
   `gateway_not_subscribed` / `gateway_subscription_not_scoped`），不发布 MQTT。
3. 客户端连错节点**没有服务端重定向**：用 control 聚合的 `/api/v1/meta/mqtt` 的
   `online_clients[].node`（或 `/api/v1/meta/gateways` 的 `nodes`）找到网关所在节点后重连。
   同一 client id 不要同时连多个节点（数据/归属会不一致，不支持）。

#### 审计与变更记录

- 事件：`control_request`（已通过校验、准备下发；**发布前**落库）、`control_result`
  （`ok` / `gateway_error` / `timeout` / `publish_error` / `server_shutdown`；detail 带
  `seq/outcome/code/latency_ms`）、`control_rejected`（`validation` / `no_gateway_mapping` /
  `gateway_not_local` / `gateway_not_subscribed` / `gateway_subscription_not_scoped` /
  `not_owner` / `too_many_pending` / `control_unavailable`）、`control_denied`（无 `control`
  规则；同一 actor+subject 60 秒内最多一行，detail 带 `suppressed=N`）。outbox 满导致的
  `audit_buffer_full` 只出现在**日志**里（WS 响应固定为 `Write Error`），不是审计事件 ——
  那时审计行本身写不进去。
- `control_request` 与 `control_result` 靠 `seq` 关联（`conn` 只记在 `control_request` 里）；已下发命令不因客户端断开而取消。
- **本地持久化是下发门槛**：`role=node` 先写本地 `audit_outbox`（达到上限 → 400，不下发），
  后台批量补送 control；control / 独立模式直接写本地 `auth_audit`。control 不可达不影响
  控制执行，但控制台审计会延迟。
- 节点关机只停 forwarder，**pending 行留在 outbox**，下次启动继续补送；节点磁盘丢失会丢
  未转发的审计。
- 控制台「变更记录」页会显示 `control_*` 行，可按「控制操作」过滤；detail 为 `key=value`
  串（长串截断展示，完整值在悬停提示里）。

#### 状态与已知限制

- `/api/v1/status` 的 WS / 审计字段：`ws_connections`、`ws_subscriptions` 是**当前值**
  （gauge）；`ws_pushes`、`ws_push_dropped`、`control_writes`、`control_timeouts`、
  `control_rate_limited`、`control_response_dropped` 是累计计数；节点另有
  `audit_outbox_pending`、`audit_outbox_oldest_age_secs`、`audit_forwarded`、
  `audit_forward_failures`、`audit_outbox_overflow`。集群聚合时逐节点求和，只有
  `audit_outbox_oldest_age_secs` 取各节点最大值（"最旧的那个节点有多旧"）。
- 已知限制：`status` 恒为 `Good`（没有 `Error` 来源）；订阅不补发初始快照；在线快照最多
  1 秒陈旧；命令无持久队列/服务端重试/幂等（超时后可能延迟执行、QoS1 可能重复）；审计
  最终一致；跨节点控制不支持；8085 默认明文，浏览器的 `?token=` 必须配合 TLS/反代或内网
  限制；client-id 绑定无开关（升级前核对，见上）。

**客户端列表**（2026-11-07 起）：`GET /api/v1/meta/realtime`（**Admin**，与 `/meta/mqtt`
同档；集群模式支持 `?node=`，无参数时聚合——顶层 `connections` 为可达节点求和，
`clients[]` 并集且每行带 `node`，另有 `nodes[]` / `errors[]`）。每行：`conn_id`、
`principal`（认证关闭时 `null`）、`peer`、`connected_at_ns`、`last_activity_ns`
（最近一次入站消息；均 Unix 纳秒）、`subscriptions[]`（`设备id.字段`，`read` 整体替换后的
当前集）。未启用时 `{"enabled":false}`。控制台 `#ws` 页的「客户端」表即此端点；非 Admin
令牌只降级这张表，WS 计数照常显示。

---

## 8. 前端页面

内置控制台与 HTTP API 通过同一服务端口提供访问，可用于日常配置、查询及运维操作。


`GET /` 控制台单页（嵌入服务，与 API 同端口；侧栏 **12 个 tab**，默认「数据查询」）。按侧栏顺序：

- **数据查询**（`#sql`，默认）：直接执行 SQL（mode 可切 all/history）；**历史查询下拉**——最近执行的语句（含执行失败的）留在浏览器 localStorage（最新在前、按 SQL 文本去重、上限 50 条），选中只回填文本与当时的 mode、不自动执行，「清空历史」按钮二次确认后清空；记录纯浏览器端，换机器/换浏览器不共享；
- **运行状态**（`#status`）：进程内存/CPU、机器可用内存与负载、采集 buffer/WAL/parquet 占用、MQTT 计数（每 3 秒自动刷新）；集群模式下追加**节点表**（角色/可达/运行时长/复制滞后/配置指纹/关键计数）、**网关与放置概览**、**对等错误**与**配置指纹分叉提示**（独立模式整块不渲染）；
- **网关管理**（`#gateways`，0.9.1 起）：子 tab「网关 / 节点 / 设备归属」。可新建/删除节点与网关；编辑网关的放置集（**整体替换**，可多选节点）。「设备归属」子 tab **按需查询**——打开页面只拉节点/网关/在线状态（约 5KB），点「查询设备」后才拉设备列表（默认 **20 行/页**，0.9.2 起），支持库/表/归属（全部/未归属/具体网关）/关键字筛选，勾选跨筛选和翻页保留；批量操作为「改盖到」「清除章」「接入」（接入调用 attach，**只增放置**，与放置弹窗的整体替换不同）。网关表的「在线节点」列来自 `/api/v1/meta/mqtt` 的 `online_clients`；独立模式该列显示「—」，接入按 0 个发现节点只声明+盖章、跳过放置。等价 API 见 §4.8，语义见 §9.11；
- **数据模型**（`#metadata`）：库/表/字段/设备的**浏览与增删改**（不是只读）——新建库/表、增删改字段、注册/编辑/删除设备；**删表/删库须先清空**（表内无字段且无数据 / 库内无表），否则 409。表详情的设备列表带**实时值列**（按 500 个 id 分片批量拉取，前 3 个字段 + 悬停看全部），设备详情页带实时值面板，均每 3 秒刷新；
- **用户管理**（`#auth`）：主体/规则/状态，并可**在浏览器里增删改**——新建（token / mqtt / cert 三选一，mqtt 口令留空则自动生成）、启用/禁用、轮换令牌（token 主体）、删除主体；规则可加（`+规则`）可删（规则标签上的 `×`）。**令牌/口令明文只在生成的那一次显示**，刷新即不可再得。等价入口：CLI（§9.4）与管理 API（§4.6）；
- **变更记录**（`#audit`）：最近 50 条审计（人工变更 + 网关写控制 `control_*`，后者单独分组为「控制操作」）；覆盖主体与规则、数据模型、告警与聚合规则、手工 `rollup-once`、集群拓扑/网关归属；只读、打开时拉一次。范围与纪律见 §4.6；
- **告警管理**（`#alerts`）：告警规则的增删改（条件 / severity / 启用 / `for_ticks`；源库/源表与条件的字段、标签筛选的字段都是**下拉**，阈值与取值手写）、每条规则**最近一次巡检**的判定状态灯、**被拒规则连同原因**的清单、**上一次重载失败的原因**（有的话），以及**「最近告警」表**（状态转换，最新在前，最多 100 条，点规则的状态格只看这条规则）；规则修改立即生效（≤1 个巡检），页面每 3 秒刷新。详见 §7.4；
- **聚合管理**（`#rollup`）：聚合规则的增删改（源表 / 输出库表 / 窗口宽度（分钟下拉）/ 分组方式 / 聚合项；源库/源表与聚合字段是**下拉**、输出库表是输入框 + 候选）、每条规则**最近一次 tick** 的积压与上次评估时刻、连续失败与上次错误、**被拒规则连同原因**的清单、**上一次重载失败的原因**（有的话）；规则修改 ≤1 个 tick 生效，页面每 3 秒刷新。详见 §7.5；
- **MQTT**（`#mqtt`）：Broker 监听、连接/消息/字节计数、集群逐节点表、**在线客户端表**（client_id / 身份 / clean / 订阅，集群下带节点列）、ingest 主题规则的 accepted/rejected/dropped 与拒收原因；每 3 秒刷新。数据形状见 §7.3；
- **WebSocket**（`#ws`，2026-11-07 起）：WS 控制面运行计数（每 3 秒刷新，数据来自 `/api/v1/status`）：实时推送（当前连接/订阅、累计推送帧/丢帧与丢帧率）、写控制（准入写/超时/限流/响应未送达）、审计出口（outbox 待补送/最老年龄/已补送/失败/溢出；独立模式注明无 outbox）；集群模式下追加逐节点表与对等错误。**「客户端」表**来自 `/api/v1/meta/realtime`（Admin）：conn / 主体 / 来源 / 连接时间 / 最近活动 / 订阅，集群下带节点列；非 Admin 只降级这张表。全 0 时按该端点的开关给出「未启用」或「已启用但暂无客户端」，不再猜。协议与端点见 §7.6；
- **API 文档**（`#docs`）：静态速查（随二进制内嵌）——页内目录、错误码表、数据面（写入/查询/实时值/WebSocket）、运维（运行状态/MQTT）、管理（元数据/用户与审计/节点与网关/告警/聚合）与版本/健康检查；完整语义见 §4–§7、§9；
- **版本变更**（`#version`）：当前版本号与**本版**变更条目（只显示当前版本，不列历史；更早版本见仓库 `CHANGELOG.md`）。

侧栏底部显示**访问状态**：未登录时点「输入令牌」粘贴令牌（存浏览器本地，随每个 API 请求发送）。

---

## 9. 运维

本章面向生产环境，涵盖数据生命周期、日志、凭据、证书、备份、合并任务及集群维护。涉及停服务、清理文件和保留期调整的操作，应先评估业务影响并确保有可用备份。


### 9.1 数据生命周期

写入 → WAL（fsync 周期 1s）→ 内存 buffer（每表按到达序平铺，不做时间分桶）→ 快照 parquet（`.parquet.tmp` 写 + fsync → 排空已捕获前缀 → 原子 rename，行在任一时刻只存在于一侧；文件名 `{node}_iotp3w_{min_time}_{max_time}_{last_seq}.parquet`）→ 合并小文件（**服务自己在 `iotp-merge` 池上按 `[merge] schedule` 跑**，§9.9；输出同契约 `{node}_iotp3p{max_input_seq}_{min}_{max}_{hash}.parquet`，整行去重、按 time 排序；输入先下线、产物后进场，任一时点至多一份可见）→ 保留期清理（同一轮按 `[merge] retention` 删过期文件）。**行只有进入 WAL 后才是持久的**；重启时从 WAL 回放，并按**每表 v3 标记文件名推导的覆盖水位**去重（0.8.6 起）：`meta/last_snapshot.json` 只作启用门槛，`.deleting` 文件计入覆盖、`.parquet.tmp` 不计；meta 不可读时**整体禁用覆盖**（宁重复不丢行）。启动期统计见 `/status` 的 `collection.wal.replay_batches_skipped` / `replay_rows_skipped` / `replay_covered_tables`（只读）；残余重复范围见 §9.8 与 `docs/specs/2026-10-31-iotp-replay-dedup-design.md` §10。

### 9.2 优雅关闭

Ctrl-C / SIGINT → HTTP 停止 → **WebSocket 控制服务停止（在飞写控制补写 `control_result outcome=server_shutdown`，连接关闭，≤5s）→ 命令回复消费者停止 → 审计 forwarder 停止（pending 留在节点 outbox，下次启动续传）** → MQTT 消费队列排空（≤5s）→ **告警引擎停止（等完在飞的那一次巡检）→ 聚合（rollup）调度器停止（同样等完在飞 tick）** → WAL flush → 最后快照 → **只 flush 的第二次 WAL 写（落盘快照留下的 open seq 组）** → 快照调度器停止 → **合并调度器停止（等完在飞的那一轮）** → **复制循环停止（等完在飞的一次拉取）** → SQLite 关闭。**勿用 SIGKILL 常规停机**（WAL 会兜底恢复，但请留出数秒）。告警与 rollup 都**排在 WAL flush 之前**：它们的输出也走写路径（WAL + 缓冲），先停它们，最后那批派生行才落得进最后一次快照（否则只能等下次启动回放）。0.6.0 起合并回到进程内，所以关闭时**会等一轮合并跑完**：等待本身没有超时（正常一轮是有界的），真卡死由服务管理器的停止超时兜底——`SIGKILL` 正好落在合并引擎的崩溃语义内（journal + `.deleting` + 启动自愈），不会丢行。审计 forwarder 的停止**不删 outbox 行**（节点上未补送的审计下次启动继续）；WS 关闭时在飞写控制会尽力补写 `control_result outcome=server_shutdown`（有界预算，超出由进程退出兜底）。

### 9.3 日志

日志**缺省写到 `{data.dir}/logs/`**，按天轮转（`iotp.log.2026-09-13`），默认保留最近 7 天。
写盘走 non-blocking 有界队列：队列满时丢日志，**不会拖慢采集/查询**。

| 键 | 缺省 | 说明 |
|---|---|---|
| `log_dir` | `{data.dir}/logs` | 日志目录；显式空串 `""` = 只写 stdout（容器场景） |
| `log_retention_days` | `7` | 按天轮转并保留的天数；`0` = 不限制 |

- **级别**由 `RUST_LOG` 决定；未设置时缺省过滤器 = 全局 `error` **加上多组有界例外**：
  合并/告警/rollup 引擎恒为 `info`，查询引擎、复制/replay、MQTT 拒收汇总、审计补送与命令链路至少 `warn`
  （`error,iotp::merge=info,iotp::server::merge_scheduler=info,iotp::server::query_engine=warn,iotp::alerts=info,iotp::server::alert_scheduler=info,iotp::rollup=info,iotp::server::rollup_scheduler=info,iotp::server::replication=warn,iotp::meta::replay=warn,iotp::mqtt::router=warn,iotp::server::audit_forwarder=warn,iotp::mqtt::command=warn`）——
  合并轮次的账目与**逐批失败原因必须落盘**，否则一次失败的轮次只会把运维指到 `{data.dir}/logs`
  而里面没有原因（§9.9）；查询侧「最新 N 条」前缀早停的**异常路径**同理（它的逐查询账目是
  `info` 级，要 `RUST_LOG=info`；常开的观测口是 `/api/v1/status` 的 `query_early_stop` 计数器）；
  **告警引擎与合并逐字同因**：回写被拒（schema 或字段名对不上）时的那条 warn 是"引擎没告警"
  与"告警写不进去"的唯一区别（§7.4），而调度器只在**有状态转换**的 tick 记一行，不是流水账。
  rollup、复制/replay 与 MQTT 拒收汇总同款：回写被拒/块失败/拉取失败/网关翻译失败都是"静默
  失败"的唯一信号；MQTT 每规则每 60s 最多一条 WARN 样本 + 一条汇总。审计补送失败（节点上
  control 不可达/拒收，§7.6）与跨网关 reply 告警（`iotp::mqtt::command`）也默认可见，
  只开到 `warn`（逐命令流水仍要 `RUST_LOG`）。
  其余模块**只打 ERROR**（默认很安静）；运维建议 `RUST_LOG=info`（会覆盖缺省，此时合并照旧可见）；
- **逐消息拒收不再刷日志**（2026-09-13 起）：MQTT 逐条拒收由 WARN 降为 DEBUG —— 现场网关
  异常时它可达约 120 条/分钟（≈17 万行/天），而同样的信息已由 `rejected` 计数与 reasons
  分解覆盖（`GET /api/v1/meta/mqtt`）。需要逐条细节时按模块开：
  `RUST_LOG=info,iotp::mqtt::router=debug`（2026-10-30 起缺省过滤器已把该 target 放到
  `warn`：每规则每 60s 最多一条样本 + 一条汇总；上面的 debug 串用于逐条细节）；
- **目录不可用**（建不出/不可写）时自动**回落 stdout** 并打一行 WARN，日志配置不会导致服务起不来；
- **启动期错误**（如配置解析失败）早于日志初始化，只能进 stderr —— 所以启动命令里的
  `2>&1` 重定向建议保留；
- 写文件的日志**不带终端转义序列**（写文件时关闭 ANSI）。

### 9.4 凭据（`iotp token` / `iotp mqtt-user`）

认证开启后，三种主体（`kind`）各走各的凭据，**同一张表、同一套 `--allow` 语法、同一套审计**：

| kind | 凭据 | 入口 | 建法 |
|---|---|---|---|
| `token` | Bearer token（服务端生成，256 位随机） | HTTP | `iotp token new` |
| `mqtt` | CONNECT 的 username/password | MQTT | `iotp mqtt-user new` |
| `cert` | 客户端证书（CN = 主体名） | HTTP + MQTT | 先 `iotp token new` 之外的路径建主体（管理 API `kind:"cert"`），证书用 `tools/pki/` 签 |

**没有任何"初始口令"**：能跑 CLI = 已经能登机器（root 级信任），这就是 bootstrap。

```bash
# 人/脚本：令牌只打印这一次，库里只存 sha256
iotp token new --name ops-wft --allow 'query:db:*' --allow 'admin:*' --note "运维"

# 设备（MQTT）：口令可自带，也可缺省生成
iotp mqtt-user new --name gw-01 --allow 'mqtt_pub:topic:devices/+/data'
iotp mqtt-user new --name gw-legacy --password '<口令>' --allow 'mqtt_pub:topic:devices/+/data'

iotp token list          # 三种 kind 一起列
```

- 规则写法 `ACTION:RESOURCE`：动作 `write|query|admin|control|mqtt_pub|mqtt_sub`，资源
  `*` / `db:<库>`（含该库的表）/ `db:*` / `table:<库>.<表>` / `topic:<MQTT 通配>`；
- **命令要么全成、要么不留痕**（2026-09-15 起）：规则先全量校验再建主体，其中任何一条
  不合法（动作不认识、资源串写错）则整条命令失败且**不建主体**；`--password ''` 被拒
  （空口令等于没有凭据，不给 `--password` 会自动生成一个高熵口令）；
- **`mqtt` 口令不能当 HTTP Bearer token 用，反之亦然**（kind 隔离）。这条是安全边界：
  共用凭据空间的话，一个弱 MQTT 口令就成了公网 HTTP 端口上的有效凭据；
- **服务在运行也可以建**：外部改动 ≤5 秒生效（服务盯着 `iotp.db` 与 `iotp.db-wal`）；
  经管理 API 的改动是立即生效；
- **轮换**：用新名字再建一个 → 各处切过去 → `token list` 确认无人使用旧名后删除
  （管理 API `DELETE /api/v1/meta/auth/principals/{id}`）；口令类主体也可直接
  `POST /api/v1/meta/auth/principals/{id}/token` 换密钥（旧口令立即失效）；
- **吊销**：删除主体即失效（删完 ≤5 秒生效），不需要重启。**不会被自动补回来** ——
  过渡用的共享主体也只能由 CLI/API 显式创建，绝不做"启动时缺则自动建"
  （那会让删除这条吊销路径在下次重启时失效）；
- ⚠️ 命令行里写 `--password` 会进 shell history 与 `ps`；过渡期用完请轮换。

### 9.5 证书签发（`tools/pki/`）

机器身份用客户端证书（HTTP 与 MQTT 共用一套，**证书 CN 即主体名**）。签发流程固化为脚本，
完整说明见 `tools/pki/README.md`：

```bash
tools/pki/init-ca.sh "iotp-ca"                 # 一次性；CA 私钥离线保管（别放应用服务器上）
tools/pki/issue-server.sh iotp-broker 203.0.113.10   # 服务端证书（SAN 必须是客户端连它用的地址）
tools/pki/new-client-csr.sh gw-230 /etc/iotp-certs   # 设备侧：私钥不出设备
tools/pki/sign-client.sh gw-230 gw-230.csr           # 签发机：签完把 .crt 交回设备
tools/pki/check-expiry.sh 30                         # 到期巡检（cron 里跑，退出码接告警）
```

要点：**证书到期 = 该设备静默断流**（握手失败），所以巡检必须做；**轮换要换新名字**
（一个 CN 就是同一个身份，重签不会让旧证书失效）；**吊销主路径是在 iotp 里删主体**
（立即生效，不依赖 CRL）。设备侧配置文件一行：

```bash
curl --cacert ca.crt --cert gw-230.crt --key gw-230.key -X POST --data-binary @batch.lp \
     "https://<host>:8080/write?db=HCYtest"   # 端口随配置；该写法要求 [server] 启用了 TLS
```

> ⚠️ **生产（cloud）当前不适用上面的 HTTPS 写法**：自 2026-10-07 起它是**集群控制面 + 明文
> HTTP**（§9.10：`role = "control"` 与 `[server]` TLS 互斥，启动即拒），对生产请用 `http://`，
> 客户端证书那条路径在生产当前不可用；独立模式或未声明集群角色的部署仍可照上面写。

### 9.6 备份

- 冷备：停服后整体拷贝 `data.dir`；
- 元数据：`{data.dir}/iotp.db`（SQLite，权威模式）；
- 数据文件：`{data.dir}/data/{db}/{table}/*.parquet`。

### 9.7 稳定性压测脚本

```bash
python3 tests/stability_test.py \
  --clients 3 --mqtt-clients 2 --duration 18000 --interval 180
```

本机起 iotp，多客户端（HTTP LP + MQTT QoS0，纯标准库）推送确定性数据（覆盖全部类型、每表多设备、掺坏行），周期性以**精确聚合比对**校验 mode=all / mode=history / parquet 文件 / MQTT 计数（accepted/rejected/dropped/messages_total 双侧界），任一失败退出码 1。参数：`--duration`（秒）、`--clients`（HTTP 客户端数）、`--mqtt-clients`（MQTT 客户端数，0=不启用）、`--interval`（快照间隔秒）、`--port`（固定 HTTP 端口）、`--keep-tmp`（保留现场）。

打已有实例（不启动也不清理 iotp、不建临时目录、不编译；目标进程不会被发信号）：

```bash
python3 tests/stability_test.py --base-url http://127.0.0.1:8080 --interval 600 \
  --clients 2 --duration 60
```

`--interval` 必须填**目标实例**的 `[flush] snapshot_interval`（生产是 600），否则 history 窗口的期望会算错。库/表/设备已存在时复用（按标签值查回设备 id，不会重复注册）。因为是往长期存活的实例上写，校验窗口加了「本次运行首行时间」下界，把此前运行留下的行排除在聚合之外；本地 parquet 文件抽查在此模式下跳过。`--mqtt-clients > 0` 时用 `--mqtt-port` 指定目标的 broker 端口（缺省 1883）。注意它**会向目标实例写入数据**（库 `stress`，设备 `region=r1..r4`，不自动清理）。

### 9.8 已知问题（详见设计 spec §15）

- **tracing 默认静默**：未设 `RUST_LOG` 时只输出 ERROR（**多组有界例外**见 §9.3：合并/告警/rollup、查询异常路径、复制/replay、MQTT 拒收汇总、审计补送与命令链路）——建议启动设 `RUST_LOG=info`；
- **快照 rename 后、WAL 截断前崩溃**：**2026-10-31 起**回放按 v3 标记文件名
  （`{node}_iotp3w_{min}_{max}_{seq}`）推导每表覆盖水位，已 rename 的行不再重放；
  合并产物带 `{node}_iotp3p{max_input_seq}_...`，删除输入后覆盖仍保留。残余
  （首次快照尚无 meta、升级前未标记文件、node 改名、保留期删除最高 seq 文件、
  人工篡改）见 `docs/specs/2026-10-31-iotp-replay-dedup-design.md` §10；
  建议停机用 SIGINT。启动期跳过量看 `/status` 的 `collection.wal.replay_*` 三个字段。
- **0.9.0 升级前必查（网关 client-id 绑定）**：`[auth] enabled = true` 时，网关 MQTT client id
  必须与 `devices/<gw>/data` 主题里的 gwId 完全一致，否则发布会被 ACL 拒绝（QoS1 仍回 PUBACK，
  **表现为静默丢数据**，无开关）；`devices/#`、`devices/+/write` 等覆盖控制命名空间的宽订阅会
  收到 SUBACK `0x80`；`[realtime]` 默认关闭且 8085 明文。详见 §7.6。
- MQTT 载荷超 `max_payload_size` 会**断开客户端连接**且 iotp 侧无计数（运维须按此设计告警）；
- **保留期删除不可逆**：合并（服务自动那一轮与手工 `merge-once` 都一样）按 `[merge] retention` 删过期文件，没有 journal、没有撤销；冻结设备时钟的表按"收到时间"计龄（§9.9）。

### 9.9 合并（服务自调度；`iotp merge-once` 只是手工工具）

**合并由服务自己跑**（0.6.0/2026-09-17 起）：`iotp` 进程内有一条专用 `iotp-merge` 线程池，
按 `[merge] schedule`（**墙钟 cron**，缺省 `0 */6 * * *` = 本地 0/6/12/18 点）自己触发一轮：
逐表自愈 → 分批流式合并 → 原子落盘 → 保留期清理。它跑在**服务的 cgroup 里**、日志进服务日志
（`{data.dir}/logs`）；需要停它就改 `[merge] enabled = false` 并重启。**没有任何外部 timer**。

```bash
# 手工补跑一轮（自愈 + 合并 + 保留期清理；配置里的 [merge] 段就是它的参数）
# ⚠️ 先停服务 —— 服务自己就是一个合并者，两轮并发不在支持范围内
sudo systemctl stop iotp
sudo -u <运行用户> iotp --config /etc/iotp/iotp.toml merge-once
sudo systemctl start iotp
# → 合并完成：合并 12 批（失败 0）、写出 3,240,111 行、去重 0 行、保留期删除 3 个文件
```

- ⚠️ **手工那一轮必须先停服务**：引擎故意不设锁，同一份 `data.dir` 上**两个合并者同时动同一张表
  得不到正确结果**（`.deleting` 与 journal 的判定都建立在"同一时刻只有一个合并者在动这张表"
  之上）。服务在按 `schedule` 自动跑，**所以手工一轮与服务自动一轮会撞车** —— **先停服务是
  唯一受支持的做法**（不要去算"两次触发之间的窗口"：一轮的时长不可预测，算错就是两轮重叠）。
  **不要再装一个 timer / cron 去驱动 `merge-once`**：那就是第二个合并者。运维细节（为什么、
  怎么做）见 `deploy/README.md`；
- **成败判据一律看「非 0」**：`0` = 这一轮没事（含"没有候选文件"与"`data` 目录还不存在"两种无事可做）；
  **`1` = 启动/配置层面的失败**（`--config` 路径错误、TOML 读取/解析失败等，发生在子命令分发前，
  错误形如 `Error: …`）；**`2` = 这一轮失败了**，**例如**：① 所有批次都失败（积压一个字节都没消化）；② **配置被拒**
  （`[merge] retention`/`seal_size` 写法不合法 × 时间单位只有小写 `s`/`m`/`d`，或值域越界，
  `schedule` 串不合法）—— 配置层的拒绝发生在**任何 parquet 文件被读改之前**（`iotp.db` 会被
  打开：那是元数据层的正常访问，取权威 schema 用的）；③ `iotp.db` 打不开 / `data` /
  `data/<db>` 等目录**列不出来**（权限、IO；注意"目录不存在"只在**顶层** `data` 被当作无事可做）。
  运维脚本据此判失败，逐批原因与错误详情在 `{data.dir}/logs` 的同一天日志里（配置错误还会直接
  打在 stderr）；
- **一轮只碰已落盘的快照/合并 parquet 文件**（v3 标记名与兼容的 v2 名；不碰 buffer / WAL / `last_snapshot.json`）——服务照常采集与查询，
  这是设计属性而不是碰巧；不能接受的是**两个合并者并行**（手工那轮撞上服务自动那轮），不是
  "服务在跑"；
- **服务自己跑的轮次不需要任何运维动作**：日志里有轮次起止与账目（info）、逐批失败原因（warn）；
  **默认日志过滤器已经保证合并模块这些行一定落盘**（见 §9.3），不需要给服务单元加 `RUST_LOG`；
- **首次部署用它消化历史积压（通常要跑两轮）**：历史积压（一堆小文件）要么等下一轮
  `schedule`，要么停服务手工跑一轮 —— 但第一轮的产物**大多仍小于 `seal_size`**（合并输出按
  time/device_id 排序、压得比输入小 ≈30%，实测 45 个里 38 个），会被下一轮**重新入选**。
  **首轮按"两轮的量"预留时间与 I/O**，第二轮之后才进入稳态。

**内存**：合并与服务**同进程、同 cgroup**，所以单轮的峰值内存**计入服务单元的内存上限**
（`MemoryMax` / `rss_high_water` 也照常生效——内存紧张时新一轮查询会被 503，采集不受影响）。
一轮的上界是**构造性**的（`O(重排窗口 + 一个行组 + 单次读入的 batch)`，与表多大无关），由
`[merge] seal_size` 定档：**调大 `seal_size` 时要把服务的 `MemoryMax` 一起复核**；内存紧张的机器
应当**调小 `seal_size`**。设计验收目标 ≤1GB（对照旧引擎生产峰值 ~2.0GB）。

- 想验证一轮合并的效果（改参数前后对比），**影子目录必须包含 `iotp.db`**：权威 schema 只来自
  元数据，只有 `data/` 而缺 `iotp.db` 时每张表都会被当成"目录在盘上、元数据里没有"的孤儿表跳过，
  一轮下来什么都没做（而且看起来"成功"）。影子不要与在跑的服务共用同一个 `iotp.db`，也不要与
  在跑的服务共用同一个 `data.dir`。

### 9.10 集群（控制面，2026-10-05 起）

- `[cluster]` **整段缺省 = 独立模式**（单机行为不变）。三种角色见 `configs/iotp.toml.example`。
- 控制节点：元数据唯一写者；分片节点上的管理写会收到 **409 `NOT_CONTROL_NODE`**（错误里带控制节点
  地址——照控制台/API 提示到控制节点执行）。读（GET）、`/write`、`/query`、`/realtime` 不受影响。
- 节点重启：**存量节点**不等待控制面（照常起、滞后在 `/api/v1/status` 的 `cluster.lag` 可见）；
  **全新节点**必须先完成元数据追赶，控制面不可达则**拒绝启动**（退出码非 0，stderr 含
  `cluster bootstrap 失败`）——宁可停着，也不拿空元数据对外服务。
- CLI 纪律：`iotp token new` / `iotp mqtt-user new` / `iotp rollup-once` **只在控制节点**运行
  （节点上退出码 2 + 原因 + 控制节点地址；`iotp token list` 任意节点可用）；
  `iotp merge-once` 任意节点可跑（本地数据命令，停服务后）。
- 升级顺序：**控制面先升，节点后升**（当前**协议号 4**；协议不匹配的拉取会被明确拒绝，错误里带双方版本号，节点数据面照常）。
- 内部端点 `/internal/*` 用 `[cluster] secret` 鉴权，建议只在内网暴露。除 `changes` / `query` / `realtime` 等既有路由外，0.9.0 起节点向控制面补送网关写控制审计：`POST /internal/audit?protocol_version=4`（**仅控制面**，`event_uid` 去重、保留原始时间戳）。
- **部署顺序**：集群化 = **控制面先以 `role = "control"` 启动**（首次启动会把"日志前时代"的
  存量元数据播种进变更日志，见 `docs/specs/2026-10-05-iotp-cluster-control-plane-design.md`），
  **节点再加入**。反过来先起节点，全新节点会因控制面不可达而拒绝启动。

**排障**

- `cluster.lag` 是**展示值**（`control_current_seq - last_applied_seq`，负数截 0），不是正确性判据；
  **首次成功拉取之前是 `null`**（未知 ≠ 0，别把 `null` 当"滞后 0"），**控制节点自己的 `lag`
  恒为 `null`**（它从不拉取）。逐节点"最后所见"在控制面 `/status` 的 `cluster.nodes_seen`。
- **缺父行容错（2026-10-07 起）**：日志里的行引用了本库不存在的父行时（典型：控制面的
  **历史孤儿行**——DDL 尚无 FK 声明年代的删除残留——被基线播种照实带上），**跳过该行**、
  其余照常应用、水位照常推进——新节点的 bootstrap 不会被一条历史垃圾挡住（此前是整批
  `FOREIGN KEY constraint failed` 永久卡死）。跳过数在 `/status` 的 **`cluster.apply_skipped`**
  （不是错误计数，>0 = 副本与权威库在这些行上有差异；明细按批走一条 warn 日志，最多列 8 条）。
- **手工回拨复制水位**（直接改 `{data.dir}/iotp.db` 的 `replication_state.last_applied_seq`）
  **之前，必须先清空目标库里的被复制表，或明确接受批边界限制**（设计 §5.3）：重放的幂等承诺
  只覆盖"从旧状态重放一段含创建与删除的净效果流"的常规形状；**同一批里先建父、后有删父**
  （如 `create_table → drop_table` 落在同一批）只在首应用有意义——二次应用时缺父行的那条
  **被跳过**（`cluster.apply_skipped` 上涨 + warn 点名，批照常提交）——所以回拨水位的**误用
  信号是 `apply_skipped` 上涨**，别把"重放跑过去了"当成"回拨没问题"。
- 拉取/应用失败 → 退避重试 1→2→4→…→30s，计数在 `/status` 的 `cluster.errors`
  （`pull_failed` / `apply_failed` / `upload_failed` / `rejected_admin_writes`）；
  原因逐条走 warn 日志（`iotp::server::replication` 默认常开）。
- **控制面头部落后本地水位**（`cluster.control_current_seq < last_applied_seq`，两种成因：
  控制面从备份恢复**回档**，或节点被改指到**另一个**控制面、seq 不同源）：节点再拉也追不上，
  表现为 `pull_failed` **持续上涨**（首次触发打一行 warn，之后静默防日志洪水）——需人工处置
  （恢复原控制面 / 修回配置；确实要回拨水位时先清空目标库，见上一条）。
- 上行的 `cluster.skipped_local` = 上行时被跳过的**非派生本地条目**数（节点没有管理写纪律：
  只有人误跑 CLI 才会非 0；正常恒为 0，不是错误计数）。

**部署注意（2026-10-05 终审补充）**

- **控制面不开 HTTP TLS**：`[cluster] role = "control"` 与 `[server]` 的
  `tls_cert` / `tls_key` / `tls_ca` 任一同时出现 ⇒ **拒绝启动**（内部拉取 v1 为明文，
  设计 §6.1，节点永远握不上手）；节点侧同理，`[cluster] control` 必须以 `http://` 开头
  （`https://` 在配置校验就被拒）、`secret` 至少 16 字符（建议 32+）。
- **节点首启前先让控制面把告警表 provision 出来**：告警表经元数据复制到达节点（节点
  **只验不建**，裁定 Y）。表还没到时该节点的告警引擎**暂不评测**（服务读写照常）；接线期
  provision 失败在**节点角色**下仍会启动调度器，表复制到达、`meta_generation` 前移后
  自动重试 provision 自愈（**修复前**那一档要等下次重启），控制面/独立模式仍是"失败即
  不启动告警引擎"。所以纪律依旧是**控制面先起、确认告警表在，再加节点**。
- **回滚口径：整支一起退**。只把**节点**二进制退回集群之前 = 旧二进制忽略 `[cluster]` 段，
  该节点元数据**静默冻结**（不再拉取、读照常 —— 不报错，别当成"还在同步"）；只把**控制面**
  退回 = 内部端点不存在，**全新节点拒绝启动**（存量节点能起但不再前进）。

### 9.11 集群接入分发（分片图与写入归属校验，2026-10-06 起）

**分片单位 = 注册主体**：一台网关一个 token（**主体名 = 分片图键**），网关用它注册设备、用
mqtt 主体连 MQTT。`auth_principals.name` 全局唯一 ⇒ token 与 mqtt 主体**不能同名**（建议
token `gw-01`、mqtt 主体 `gw-01-mqtt`）。网关配置两个地址：**注册（HTTP）→ 控制节点**
（管理写只收控制节点）；**MQTT → 属主节点**。

管理面（Admin；CLI 写臂只在控制节点，`list` 任意节点）。**放置是校验的开关**：已声明未放置
的网关 = 未归属 = **永不拒收**。

```bash
iotp cluster node add --id n01                             # 声明节点（= 该节点 [node].id）
iotp cluster gateway add --name gw-01                      # 声明网关（键 = 注册主体名；可先不放置）
iotp cluster gateway place --name gw-01 --node n01         # 设置归属集（可多 --node；不给 = 未放置）
iotp cluster device-gateway --device-id 5 --gateway gw-01  # 显式盖章（--clear = 清除为未归属）
iotp cluster node rm --id n01                              # 删除节点（仍被放置集引用 → 409 并点名网关）
iotp cluster gateway rm --name gw-01                       # 删除网关（设备章变悬挂/未归属）
iotp cluster node list                                     # 看分片图现状（list 任意节点可用）
iotp cluster gateway list                                  # 归属集 + 每网关的设备数
```

HTTP 同款：`GET/POST /api/v1/meta/nodes`、`DELETE /api/v1/meta/nodes/<id>`、
`GET/POST /api/v1/meta/gateways`、`DELETE /api/v1/meta/gateways/<name>`、
`PUT /api/v1/meta/gateways/<name>/nodes`、`POST /api/v1/meta/devices/gateway`
（请求/响应形状见 §4.8）。**控制节点启动时会自我声明**（`[node].id` 进注册表；失败
只打一条 error、不拒启动）——归属集要能引用控制节点自己。

控制台「网关管理」页的「接入」按钮直接调用下面的 attach API（「接入」是产品叫法，
API 名仍是 attach）。

网关 attach（声明 + 防抢盖章 + 只增放置，一次完成）：

```bash
T='<ops-token>'                     # 裸令牌（不带 "Bearer " 前缀）
curl -X POST -H "Authorization: Bearer $T" \
  http://127.0.0.1:8080/api/v1/meta/gateways/gw-01/attach \
  -d '{"device_ids":[5,6,7]}'
```

响应（上例设备 7 已属另一已声明网关 `gw-02`，`conflicts` 中回显；字段：`declared` =
本次是否新建声明；`stamped` / `repaired` / `unchanged` = 设备 id 列表；`nodes_found` /
`placement_added` = 发现 / 新增放置的节点）：

```json
{"ok": true, "client_id": "gw-01", "declared": true, "nodes_found": ["n01"],
 "placement_added": ["n01"], "stamped": [5, 6], "repaired": [],
 "unchanged": [], "conflicts": [{"device_id": 7, "gateway": "gw-02"}]}
```

- 节点由 control 聚合的 `online_clients` 自动发现：0 个 → 只声明 + 盖章（可预配置；
  已有放置集保持不动，不清空）；1 个 → 加入放置集（**只增，不删旧节点**）；
  >1 个 → 409，不做任何修改。
- `online_clients` 是每秒重建的尽力而为快照：网关在“发现 → 提交”之间迁移时可能少放
  新节点（旧节点不会被删除，下一次 attach 会补上）。control 自身 `[mqtt]` 关闭时本地
  没有 `online_clients`，聚合结果也会把这台网关显示为不在线。
- 已属于另一已声明网关的设备不抢，响应 `conflicts[]`（`{"device_id":…, "gateway":…}`）
  列出 —— 仍为 **HTTP 200**（部分成功，冲突设备保留原网关）；指向未声明网关的悬挂章
  会被自愈（`repaired[]`）。
- 设备必须已存在；任一不存在 → 404 且整体回滚。未知请求键 / 空 `device_ids` → 400。
- node 角色调用 attach → 409 `NOT_CONTROL_NODE`（管理写只收控制面）；`client_id` 非法 → 400；
  独立模式下 `nodes_found=[]`、`placement_added=[]`，只声明与盖章、跳过放置。

**上线序列（存量库）** —— **第 0 步（升级之前）：先查存量字段有没有叫 `gateway` 的**，
有就先处理掉。它坏的不止 `devices` 视图：告警/rollup 的 provision 也走同一份保留名清单
（照抄源表标签列 / 校验别名），规则源表上留着 `gateway` 标签时，**告警接线期 provision
失败（告警整体不启用）、rollup 逐条拒掉那条规则**（见本节末"升级前检查"）。

1. 升级（新表 `IF NOT EXISTS` 自动建；**本版协议号 4** —— 协议不匹配的拉取会被明确拒绝、
   错误里带双方版本号；升级顺序永远是**控制面先升 → 节点后升**）；
2. 控制面声明节点（`iotp cluster node add --id n01`；控制节点自己启动时已自动声明）；
3. 声明各网关（`cluster gateway add --name gw-01`，**可以先不放置**）；
4. 网关侧逐台换 token（此前的共享主体章不是"已声明网关"，会被自愈覆盖）；
5. 网关重连注册（60–90 秒量级）→ 存量设备的章自愈为各网关名（重注册即自愈，无需回填工具）；
6. 逐个放置（`cluster gateway place …`）—— 此时写检查才开始生效；
7. `GET /api/v1/status` 的 `cluster.devices_unassigned` 驱动到零（控制台/运维建的设备用
   `cluster device-gateway` 显式放置）。

**语义**：未归属设备（无章 / 章的网关未声明 / 已声明未放置）**永不拒收**；归别的节点的设备
写进来 → `not_owner` 拒收：`/write` 响应 `tables.*.reasons.not_owner`、MQTT 每规则
`reasons.not_owner`、`/status` 的 `cluster.rejected_not_owner`（累计，内存计数、重启清零；
集群视图下顶层是**全簇求和**，本节点那份见 `?node=`，§9.13）与
`rejected_not_owner_by_node`（按**期望属主**摊开：单属主 = 节点名，多属主 = 排序后逗号连接
——"某台网关该指向 n02"一眼可见）。**采集优先**：**只有 `not_owner` 降为 DEBUG**（指错的
网关按全速率复现，逐条 WARN 会成日志洪水），**其余拒收原因按 `(表, 原因)` 限流**（2026-10-30：每 60s 最多 3 条样本 + 1 条汇总）。`cluster` 段
另有 `node_id` / `node_declared`（本节点 id 是否在注册表；`false` = id 打错或忘了声明，
启动时一条 WARN）/ `gateways_declared` / `gateways_unplaced` / `devices_unassigned` /
`stamp_conflicts`（防抢矩阵触发次数，内存计数、重启清零）。

`devices` 视图的 `gateway` 列显示的是**原始章**：章的网关已被撤时它仍显示原网关名（悬挂章），
而归属判定按**未归属**（`cluster.devices_unassigned` 照计）——别拿视图列当归属判据。

**双属主与搬迁纪律**：归属集 0..N。搬迁的唯一顺序是 **先把归属集设成 `{旧, 新}`（两边都收）
→ 切网关连接 → 停**（工具出现前不收敛）——任何"先翻分片图、后切网关"或反过来的顺序都有
**单边拒收窗口**。**收敛（删旧属主）前必须先完成文件搬迁并校验**：本版**没有搬迁工具**
（Phase 2），半收敛态是稳定可用的——数据不缺（新行进新节点、旧行留旧节点），查询要等协调层
（下一版）两端都问；提前收敛 = 查询静默丢历史。

**共享主体**：多台网关共用一个主体 = 一个**分片组**（同一归属集、均衡粒度 = 组）。拆分：
先给新网关放置 `{旧, 新}`（双属主）→ `iotp cluster device-gateway --device-id … --gateway 新`
→ 切网关连接 → 收敛。**防抢矩阵会挡住"直接换主体重新注册"**：章属于另一个**已声明**网关时
重注册不改章，只计 `stamp_conflicts` —— 换归属必须走显式 API。

**auth 关闭时**：注册即盖章与防抢矩阵不可用（`actor` 恒为 `api`，不是已声明网关——注册
不会自愈设备的章）；**显式盖章（`POST /api/v1/meta/devices/gateway` / `iotp cluster
device-gateway`）与写检查照常生效**（端点/CLI 照常盖，写检查照常拒他节点行，
`rejected_not_owner` 照常计数，告警/rollup 的归属过滤照常适用）。本特性面向
`auth.enabled = true`。

**升级前检查（`gateway` 是保留字段名）**：`gateway` 进了保留名清单，存量库里叫 `gateway` 的
字段有**两处**后果：① `devices` 视图的末列也叫 `gateway`（设备章），而视图是 `SELECT *`
—— 重名列让视图坏掉；② **引擎 provision 也走同一份清单**（告警/rollup 把源表标签列照抄进
输出表、rollup 还要校验聚合别名）—— 规则源表上留着 `gateway` 标签时，**告警接线期
provision 失败（告警整体不启用）**、**rollup 逐条拒掉那条规则**。升级前用
`GET /api/v1/meta/tables/<db>/<table>` 看 `fields`（规则源表尤其要看），有 `gateway` 就
先处理掉：**标签字段**可以改名挪走（改名只校验**新名**，所以旧名是保留名也能改走）；
**测量字段**在表还没有数据时可以同样改名挪走（判据同类型变更），有数据就只能删（删除走墓碑，
该名字此后不可复用）。2026-10-06 起改名/删除在 API 上都可用、2026-10-29 起测量字段也能改名
（保留名清单只挡**新**名）——**升级后才发现也能照此补救**（改走/删掉后告警下次 provision、
rollup 下次装配即通过），但升级前先查最省事。

**回滚**：整支回退二进制即可 —— 旧二进制忽略分片表、不做校验（退化为"不校验"，**不会拒收**，
安全方向）；已写的章/放置留在库里，再升上来即恢复。只回退节点/控制面的后果见 §9.10 的
"回滚口径"。

**顺带两条**：① 集群模式下 `GET /api/v1/alerts/state` 的 `devices_matched` 只数**归属过滤后**
的设备 —— 一条规则的设备全搬走时会读成 0（规则惰性 = 正常，不是坏了）；② 告警/rollup 的
派生设备（镜像/合成）**永不盖章**，其回写天然通过写检查，归属按**源设备**判定。

### 9.12 集群查询协调（跨节点查询与 realtime，2026-10-06 起）

设计权威：`docs/specs/2026-10-06-iotp-cluster-query-coordinator-design.md`（下称"协调设计"）；
端点语义不变（`/api/v1/query`、`/api/v1/realtime`）—— 前端与脚本无感知地拿到集群视角。

**`[cluster] addr`（必配）**：它是"别的节点怎么找到我"。节点每轮拉取都把它自报给控制面
（幂等落账 → 复制），**其他节点 ≤2s 可见**；**控制节点不拉取（它是日志源头），所以启动时
自己落一条**（在节点自我声明处，同一个幂等存储）—— 它也是分片节点，别的节点要能打到它。
改了监听地址/端口必须同步改它并重启该节点（配置修改一律重启生效，控制节点改地址同样
重启生效）。写错/留占位的代价是**扇出打到死地址**：跨节点查询失败并点名
（`shard n02: connect …: Connection refused`）或
`node n02 has no address; set [cluster] addr on n02` —— 绝不静默漏行（当前没有公开的
addr 写端点，改地址只能改配置）。

**协调语义（`/api/v1/query`，两条路径）**：设备谓词（`device_id = / IN`、标签字段的正向
`=`/`IN`，与标签改写同款纪律；否定类不参与）能把相关设备裁到**单个节点** ⇒ **委托**：
原句转发（`/internal/query`）或本节点直接执行，**任意 SQL 不限**（窗口/CTE/JOIN/EXPLAIN
照常），单节点语义（早停、截断、错误形状、`elapsed_ms`）逐字保持（例外：`elapsed_ms`
**一律是协调者侧整请求时钟**，含网络与分片耗时 —— 分片自报的执行段只进逐查询 info 账目）；裁不到 ⇒ **扇出**：
只发给相关节点（无可用谓词/未归属设备 ⇒ 全部已声明节点），分片跑"形状改写后的 SQL"，
协调者把分片结果当输入再跑**终段**（值/类型/NULL 排序由 DataFusion 决定，不手写算子）。
只查 `devices` ⇒ 本地直答（全量复制）。

**扇出白名单（v1；白名单外一律 4xx）**：纯行集/过滤、`ORDER BY … LIMIT n`、
`ORDER BY … LIMIT n OFFSET m`（各分片取 top-(n+m)、终段越过 m 行再取 n）、
`OFFSET`（无 LIMIT 时终段越过；无 ORDER BY 的行集同样支持）、
`SUM/COUNT/MIN/MAX`（± `GROUP BY`）、`AVG`、`HAVING`、`DISTINCT [± LIMIT [OFFSET]]`，以及它们的
**单形状**组合。分页语义：无 ORDER BY 的形状取哪几行本就没有确定性顺序，与单机可以不同
（行数/截断不劣化）；排序键有并列时分页边界跨实现不保证稳定，分页请用唯一排序键。
被拒的形状**逐个点名 + 指向设计文档 §5.1/§5.3**（不静默给错值）：
窗口函数、CTE、子查询/派生表、UNION 等集合运算、跨表 JOIN（除非所有相关表裁到同一节点）、
`GROUP BY` 表达式、不可合并的聚合（`stddev`/`median`/`percentile`/
`count(DISTINCT …)`…）、**投影里的易变函数**（`now()`/`random()`/`uuid()`… —— 每分片各自
求值必然不一致）、无法裁到单节点的 `EXPLAIN`、解析失败、多语句。
**parity 提示**：`ORDER BY <未投影的聚合>`（如 `SELECT region … GROUP BY region ORDER BY
sum(x)`）**单机上也报错**（DF 40 的行为）—— 看到 422 不是集群引入的。

**滞后纪律**：归属与地址都从**本地 cache** 解析，窗口 = 复制滞后（≤2s），协调层不做
"跨节点再确认"。**放置变更 / 搬迁收敛之后，等 `/api/v1/status` 的 `cluster.lag` 归零再发
跨节点查询**——否则会按旧的归属集扇出（漏掉刚搬过去的节点、白问一台已经没有数据的节点）。

**失败语义**：共享超时预算（= `[query] query_timeout_secs`）；**任一分片失败/超时 = 整查询
失败**并点名节点 + 分片侧 code/消息（v1 无部分结果）。分片上的"该表在本节点尚无数据"
（plan-class `table … not found`，**前提是这张表在协调者的元数据里**）按**空分片**
处理 —— **仅限扇出分片**：Local/Delegate
保持单机 parity（新表在没有数据的节点上照旧报错，与单机一致）；表在**哪里都不存在**
（表名打错/表已删）时扇出也**整查询失败并点名表名**，绝不回 200/0 行（那与"表是空的"
无法区分；单机同句本就是 422）。协调者的合并缓冲受自身
`max_rows` 约束：扇出最坏 = **N × max_rows**（N = 参与的节点数），`truncated` 自任一分片
冒泡。realtime 是另一套口径：分片失败**不整失败** —— 批量把该设备降级成逐条 `errors`
（保持 200），单设备 `GET /api/v1/realtime/<id>` 没有逐条通道 ⇒ **502 `BAD_GATEWAY`**
（消息点名节点）。

**空表聚合（2026-10-30）**：全分片都"确无数据"且形状是聚合（`count/sum/avg/min/max`，
有无 `GROUP BY` 均可）时，协调者用权威 schema 建一张空表重跑**用户原句**：`count(*)`
答 `0`、`sum/avg/min/max` 答 `NULL`、`GROUP BY` 答空；只对**库限定、大小写精确**的表引用
生效（裸名/大小写不匹配保持空行集）。`Local`/`Delegate` 臂不变：`WHERE device_id=…` 把
扇出变成单节点委托时，空表仍按单机语义报 422。回退失败明确报错，不答 `[]`。未加别名的
聚合列名在"空/非空"之间会翻转（空表走原句名，非空走协调者改写名）——需要稳定列名的
客户端请用 `AS alias`。

**realtime 跨节点合并口径（`/api/v1/realtime` 单个与批量）**：未知设备**本地**判
（设备全量复制，不扇出）；已知设备按**属主**分组：未归属 ⇒ 问全部已声明节点，双属主 ⇒
两头都问。多来源合并 = **逐字段按 `time` 取大**（单侧有值直接取），`updated_at` = 各字段
时间的最大值。与单机 LatestCache"按**到达**时间"的差异：**冻结时钟设备**（上报时间不推进）
在集群下由"字段 `time` 更大者"胜出、单机由"后到者"胜出 —— 结论可能不同（协调设计 §8）。

**内部端点（部署级信任）**：`/internal/query` 与 `/internal/realtime` 在**每个声明了角色的
节点的 HTTP 端口**上都可达（扇出的分片侧），鉴权只有 `[cluster] secret`（错密钥 401、
协议版本不匹配 400）。**拿到密钥 = 可以用部署身份执行任意 SQL**（任意表任意读）——与
`/internal/changes` 是同一个信任模型：密钥就是部署级信任，**不要把这些端口暴露到不可信
网络**。内部调用 v1 **只走明文**：控制面配 `[server]` TLS 会**拒绝启动**（节点永远握不上
手），节点侧配了 TLS 则它的分片接口收不到扇出（`[cluster] addr` 必须是 `http://`，见
§9.10"部署注意"）。请求体没有长度上限（密钥门内，与既有内部端点同款）。

**观测（`GET /api/v1/status` → `cluster.coordinator`；内存计数、重启清零；集群视图下顶层是
全簇求和 —— 本节点那份见 `?node=`，§9.13）**：
`delegated_local`（本节点直接执行，含 devices-only 与空集裁剪）/ `delegated_remote`
（原句委托到某个远端节点）/ `fanned`（扇出 + 终段合并）——**逐查询各 +1**；`fanned` 是
**分派前**计数：算的是"发起过的扇出"，扇出后分片够不着的失败查询照样 +1；
`rejected_shape` 只数**计划期**被拒的查询（4xx 一条）；`shard_failures` 按**分片调用**
计数（一次扇出里两个分片失败 = +2），**委托 / 扇出 / realtime 三条路径共用**这一个数 ——
"哪些节点在挂"看它。逐查询账目（形状、相关节点、各分片耗时/行数）默认不落盘，要
`RUST_LOG=info`（target `iotp::server::query_engine`；拒绝路径的样本/汇总按限流规则默认可见，2026-10-30）。

**回滚**：整支回退二进制 —— 旧二进制协议版本不匹配时管理面停旧、数据面照常（见 §9.10）；
协调逻辑消失后跨节点查询退化为**单节点查询**（安全方向：不静默给错值，只是看不到别的
节点的行）。只回退节点/控制面的后果见 §9.10 的"回滚口径"。

### 9.13 集群状态聚合（五个端点的跨节点视图，2026-10-07 起）

设计权威：`docs/specs/2026-10-07-iotp-cluster-state-aggregation-design.md`。**声明了角色的
集群**下，五个公共端点变成**集群视图**：`GET /api/v1/status`、`GET /api/v1/meta/mqtt`、
`GET /api/v1/alerts/state`、`GET /api/v1/rollup/state`、`GET /api/v1/meta/realtime`
（2026-11-07 新增 WS 客户端明细）—— **接收请求的节点当聚合器**（本节点
直读 + 并发拉各对等节点的 `/internal/*`，共享密钥），按端点各自的规则表合并。**独立模式
（无 `[cluster]` 或未声明角色）逐字同今天**（连 `?node=` 也照旧忽略，形状一个字节不变）。
鉴权档位不变（Authenticated / Admin / Query / Query）：聚合器在本地闸门后**代表调用方**
访问对等节点。**不聚合的**：`/health` 与 `/api/v1/version`（探针保持本地语义）、**其余**
`/meta/*` 的读（全量复制 ⇒ 任一直答；`/meta/realtime` 是例外，它和其他四个状态端点一样
按本节聚合）；`/alerts/recent` 走**同一 SQL 协调层**（§9.12 的
机制）而不是本节的合并 —— 告警仍由各节点**自治产生**（各写各的表），这条 SQL 经协调查到
各节点再汇总，所以**在任一节点**看到的"最近告警"都是**全簇**的。两档"直接答空、不去查"：
**① 表不在本地元数据里**（两模式皆然 —— 集群下判据是**本地**那份副本，它只在复制窗口内
可达，窗口内可能答空，等一拍即自愈）；**② 本地两个行源都空且没有别的已声明节点**（独立
模式，或孤立声明节点 —— 那时本地就是全簇）。**有对端时本地空也必须去协调**：本地空分片
是对端的常态，fanout 的空分片分类就是为它设计的。

**对账口径（读聚合视图的第一条纪律）**：轮询页面用聚合视图；排障用 `?node=n02`。

- **顶层计数器类 = 各已答节点之和**（本节点 + 成功答复的对等节点；全簇合计）：`/status` 的
  `mqtt.accepted/rejected/dropped`、
  `query_guard.queries_rejected`、早停两计数、`alerts` 四计数、`rollup` 十计数、
  `ws_connections` / `ws_subscriptions`（当前值，按节点求和得全簇总数）、
  `ws_pushes` / `ws_push_dropped` / `control_writes` / `control_timeouts` /
  `control_rate_limited` / `control_response_dropped` / `audit_forwarded` /
  `audit_forward_failures` / `audit_outbox_overflow`、`audit_outbox_pending`（求和）、
  `cluster.coordinator.*`、`cluster.rejected_not_owner*`、`cluster.agg_*`；`/meta/mqtt` 的
  `clients_connected`、`messages_total`、`bytes_total`、`acl.denied_total`；
  `/meta/realtime` 的 `connections`（sum）与 `clients[]`（union，每行带 `node`）。
  **唯一取最大值的是 `audit_outbox_oldest_age_secs`**（"最旧的那个节点有多旧"）。
  **没列进求和表的计数不求和**（如 `alerts.ticks`/`published`/`publish_failed`/
  `tick_errors`，以及 `mqtt.clients_connected` 在 `/status` 这一行）：它们是**接收请求
  那个节点自己**的值 —— 别当全簇读（要全簇口径就用上面列出的那些，或逐节点看
  `nodes[].counters`）。
- **机器/节点态 = 接收请求的那个节点自己那份**：`process`/`system`、`uptime_secs`、
  `collection.{buffer,wal,parquet}` 尺寸、`cluster.lag`/`role`/`last_applied_seq`/分片图与
  邻居字段…… 例：顶层 `cluster.lag` 是**聚合器自己的**复制滞后，不是全簇的。
- **逐节点值看 `nodes[]` 或 `?node=<id>`**（`nodes[]` 本节点在前，随后对等节点按 id 排序）。
  `nodes[].counters` 是**该节点自己**的每个求和类值（键 = 点分路径，如 `mqtt.accepted`；
  叶子名会撞 —— `rows_written` 在 alerts 与 rollup 各有一个 —— 所以给全路径），与顶层和
  **可对账**：顶层 = 各**已答**节点之和。
- **缺 ≠ 0**：某节点没报某个路径（老版本节点、引擎没开）就不贡献、也不置零 —— 逐节点看
  `nodes[].counters` 的**缺键**，别读成零。不可达节点的块**不发明数据**，形状分两组：
  `/status` 的块保留全字段、值全 `null`（每一格都能说"未知"）；其余端点（`/meta/mqtt` 等）
  的块只有 `{id, reachable: false, counters: {}}`，`/meta/realtime` 的不可达块只有
  `{id, reachable: false}` —— 值**缺**而不是 `null`（那些字段本来就不摊开）。
- `cluster.agg_requests` / `cluster.agg_node_failures` 是**读时快照**：这次聚合本身会让它们
  +1（`?node=` 代理不计），而响应里带的是**请求前**的值 —— 本节点那份总比"刚发生的这一次"
  少 1。要数聚合次数就**连取两次**：第二次的 `agg_requests` = 第一次 + 1；**别拿它做"原始
  值相加 == 聚合值"的朴素对账**（读数本身会扰动差值）。
- 节点集合与对等地址都取自**本节点的复制 cache**（滞后 ≤2s）：刚声明的节点要等声明、以及它
  拉取时自报的 `[cluster] addr` 复制到达才进视图（在那之前是"还没看见"，不是不可达；地址
  未到则它按"缺 addr"进 `errors[]`）。
- `nodes[]` 与 `errors[]` **恒在**（`errors[]` 可为空数组）。

| 端点 | 顶层合什么 | 逐节点摊开（`nodes[]`） |
|---|---|---|
| `/status` | 计数器类求和（见上）；`cluster` 段**只补** `config_fingerprint` 与 `config_divergence`，其余字段照抄本节点 | 全字段块（机器/节点态 + 该节点自己的计数） |
| `/meta/mqtt` | `clients_connected` 与各计数求和；`ingest[]` 为各节点**同名规则**（topic/db/format）计数之和（逐节点那份见 `?node=`） | `listeners` 逐节点；`online_clients` 并集，**每行加 `node` 键** |
| `/alerts/state` | 规则集全量复制 ⇒ 规则定义唯一、不合并；`rejected` 逐节点列出（每行带 `node`，不静默丢） | 逐节点 verdict（`state`/`devices_matched`/`reload_error` 是节点态 ⇒ 不合并）；另给 `rule_states` 规则级汇总（只作提示） |
| `/rollup/state` | 同上；另给 `watermark_spread`（各节点水位的最大/最小差，只作提示） | 逐节点 watermark/积压/`last_run_at`/failures（水位是每节点内存态 ⇒ 不合并） |
| `/meta/realtime` | `connections` 求和；`enabled` = 任一可达节点启用；`clients[]` 并集，每行带 `node` | `{id, reachable, enabled, host, port, connections}`（端口逐节点，同机部署可不同） |

注意 `/status.mqtt` 只求和 accepted/rejected/dropped，`clients_connected`/`messages_total`
在**该端点**留在本节点值上（求和类在 `/meta/mqtt` 那一行）—— 同名概念在两个端点口径不同，
别混用。

**不可达语义（不要只看数字）**

- 对等节点不可达 / 超时 / 缺 addr ⇒ 该节点进 `errors[]`（`{node, error}`，点名节点与原因或
  地址）、`nodes[].reachable: false`，**端点照样 200** —— 逐节点失败不拖垮端点（状态面是
  观测视图，不是数据正确性面：部分节点挂掉时控制台仍要可用；与查询路径的"整失败"有意不同）。
- 所以**汇总值缺了某个节点，只能从 `errors[]` 读出来**：数字静默偏小，光看和看不出少人 ——
  `errors[]` 与 `nodes[].reachable` 是第一信号。
- 预算**硬 3s**（内部常量，与 `query_timeout_secs` 无关）：有节点挂起时页面也 ≤3s 返回并把
  它标不可达，轮询页面不会被挂死的节点拖住；已答的节点照常合并。

**配置指纹纪律（`config_fingerprint`，`/status.cluster`）**

- 指纹 = 对**解析后的有效配置**做规范化哈希（16 位十六进制），**结构性 per-node 字段**
  （`[node].id`、`[cluster].addr`、`[cluster].role`、`[cluster].control`）与**部署位置字段**
  （`[data].dir`、`[server].port`、`[mqtt.listeners.*].port`、`[realtime].port`，2026-10-08 / 2026-11-02 追加）**置空后**再算 ——
  这些字段各节点本就该不同（role/control 与 node.id/addr 在"控制面 + 节点"的集群里必然不同；
  同机多进程/多实例时端口与数据目录也必须错开），不置空必然全簇"分叉"。
- 聚合视图：`nodes[].config_fingerprint` + `fingerprint_ok`（与本节点是否一致），顶层
  `cluster.config_divergence` 列出分叉节点（空数组 = 一致）。**只报分叉、不改行为**（不拒启、
  不阻断）。
- 纪律：**配置改动要一致发布** —— 只差上面那批结构性/部署位置字段（评估 §4.5）。**其余
  字段的任何差异都会报分叉**（`secret`、TLS、端口以外的监听字段、MQTT 采集规则等），以及
  "运行期才派生的默认值不归一"的情形（如 `[server].log_dir` 不设 = `{data.dir}/logs`，把同一
  路径显式写出来却是另一种写法）——**过报是漂移探测的安全方向**：宁可多提示一次，不可漏报。
  ⚠️ 部署位置字段（`data.dir`/端口）的漂移**不再由指纹发现**（2026-10-08 裁定）：这类错误
  由各自的数据面表征（节点没数据、端口不通）暴露。
- 一方没有指纹（老版本节点）⇒ `fingerprint_ok: null`（**未知，不是分叉**），不进
  `config_divergence`；对端**不可达**（拿不到载荷）同样是 `null` —— 不可比 ≠ 一致。

**`?node=<id>`（运维 curl 的原始视图）**：这些聚合端点都接受，返回**该节点的原始响应**（本节点
直读 / 远端内部端点），**不套合并** —— 没有 `nodes[]`/`errors[]`，也没有任何汇总。

```bash
curl -H "Authorization: Bearer $TOK" 'http://n01:8080/api/v1/status?node=n02'        # n02 自己那份
curl -H "Authorization: Bearer $TOK" 'http://n01:8080/api/v1/rollup/state?node=n02'  # 逐节点水位
```

- 未知/未声明节点 ⇒ **404**（点名；先 `iotp cluster node list` 核对拼写与声明是否已复制到
  本节点）；已声明但打不通/缺 addr ⇒ **502**（点名节点与原因）。代理**不计** `agg_*`。
- `?node=` 重复或空值 ⇒ 400（静默挑一个会让运维看错节点）；独立模式没有聚合器，`?node=`
  照旧被忽略。

**控制台「集群」块（状态页）**：只在 `cluster.active` 时渲染（独立模式整块不出现，连取数
都不发生）。内容：

1. **节点表**：每节点一行 —— id、角色、可达（不可达标红）、运行时长、复制滞后、配置指纹
   （一致 ✓ / 分叉 ⚠ / 未知 —）、关键计数（**MQTT accepted**（行率由面板用该节点
   `mqtt.accepted ÷ uptime` 现算 —— **只数 MQTT 采集**，不含 HTTP `/write`，标签里写明了）、
   查询 本地/委托/扇出、拒绝 护栏/形状/分片失败、聚合 请求/节点失败）；
2. **网关与放置概览**：`GET /api/v1/meta/gateways`（**Admin 档**；全量复制 ⇒ 任一直答）——
   声明网关数、未放置数、已归属设备合计，逐网关列出并交叉 `nodes_seen`；
3. **错误列表**：`errors[]` 逐条显式（如 `n02：aggregate n02: …`）；
4. **指纹分叉提示条**：`config_divergence` 非空时出现并列出节点。

**状态页本身只要任意已认证身份**；非 Admin 令牌只让第 2 项那一行降级并说明原因，节点表与
错误列表照常。

**前端部署纪律**：控制台页面 `frontend/index.html` 在 release 构建里**编译期内嵌** —— 改
HTML 必须重新 `cargo build --release` 并部署新二进制（debug 构建读盘，别拿 debug 的现象
推断 release）。

**回滚**：整支回退二进制 —— 旧二进制不做聚合，这些端点退化为**单节点视图**（安全方向：
**不静默给错值**，只是看不到别的节点）。只回退节点/控制面的后果见 §9.10 的"回滚口径"。

**浏览器点检清单（手工，发版前）**

1. **独立模式**（无 `[cluster]`）：状态页**没有**「集群」块。
2. **集群正常态**：块出现；节点表两台都可达、指纹「✓ 一致」；网关概览几行数字出得来
   （Admin 令牌）；换非 Admin 令牌时只有网关那一行降级且给了原因。
3. **不可达态**：停一台节点（或让它的 `[cluster] addr` 指死端口）后刷新 —— 该行**变红**、
   「节点错误」列出它、顶部「对等失败」计数上涨；其余数据照常在、页面不报错（仍 200）。
4. **分叉态**：两节点配置故意不同（如改一台的 `[flush] memory_limit`）⇒ 该行指纹标 ⚠、
   卡片**底部**出现分叉提示条。

---

## 10. 已知限制与常见问题

本章保留当前版本的已知限制和排查建议。出现问题时，建议依次检查配置、权限、数据模型、服务状态和日志。


- MQTT v1 仅 **3.1.1**（TCP 与 WebSocket 均如此；rumqttd 一端口一协议版本，v5 需未来迭代的第二监听配置）；
- **MQTT 订阅按"覆盖"判，只能收窄不能放宽**：规则的匹配范围必须 **≥** 过滤器的匹配范围。
  `topic:devices/+` 不授权 `devices/#`（`#` 更宽），`topic:devices/+/data` 也不授权
  `devices/#`。要订大范围就给大范围规则（`mqtt_sub:topic:devices/#`）。发布不受此限 ——
  发布的是真实主题（无通配符），按普通通配匹配判；
- **TOML 配置文件**修改需**重启生效**（无热更新）；主体与告警/rollup 规则经管理 API/控制台修改会热装载（≤5 秒 / ≤1 tick）；
- 保留名 `time`/`device_id`/`table`/`name`/`gateway` 不可作字段名（API 400；`gateway` 自 2026-10-06 起，见 §9.11）；
- JSON 字段值严格一一对应：无自动类型猜测（设备固件须保证值类型与模式一致，或依赖无损转换规则）；
- Q：写入返回 200 但 rejected_rows > 0？A：部分接受（好行已写入，坏行拒收；原因见 warn 日志与响应 `tables.*.reasons`）。
- Q：查询一张刚建的表报 `table not found`？A：该表尚无快照文件 —— 查询引擎按 `data/{db}/{table}` 目录解析表，**单机/非聚合场景下首次落盘前任何 mode 都查不到**（快照间隔到点后自然可查）。**例外**：集群扇出时全分片都无数据的 `count/sum/avg/min/max` 聚合会用权威 schema 答 `0`/`NULL`/空集（§9.12）；`[alerts]` 的告警表同理，且它整个库都是引擎新建的。
- Q：WS 连 `ws://<ip>:8085/ws/` 连不上或握手 401？A：`[realtime] enabled` 缺省 `false`，该端口是独立 listener 且只接受 `/ws` 或 `/ws/`；`[auth] enabled = true` 时浏览器用 `?token=`、非浏览器用 `Bearer`；8085 默认明文。见 §3、§7.6。
- Q：`POST /api/v1/meta/gateways/{client_id}/attach` 返回 409？A：该 client id 同时在多个节点在线（`CONFLICT`，整体不修改），或调用节点是 `role = "node"`（`NOT_CONTROL_NODE`）。先查 `/api/v1/meta/mqtt` 的 `online_clients`，清理残留连接后重试。见 §4.8、§9.11。
- Q：attach 返回 200，但部分设备没盖到目标网关？A：看响应 `conflicts[]`：已属于另一个已声明网关的设备不会被抢（仍是 200）；`repaired[]` 是悬挂章自愈、`unchanged[]` 是本来已属于本网关。确需强制改归属用 `POST /api/v1/meta/devices/gateway`。见 §9.11。
- Q：升级后网关数据"发出去了"（QoS1 有 PUBACK）但表里没有？A：`[auth] enabled = true` 时 `devices/<gw>/data` 强制 client id == `<gw>`，不一致会被 ACL 拒绝但协议仍回 PUBACK，表现为**静默丢数据**（无开关）。逐个核对网关 MQTT client id 与 data 主题里的 gwId。见 §7.6、§9.8。
- Q：升级后看板/消费者收不到数据，订阅返回 SUBACK `0x80`？A：`[auth] enabled = true` 时 `#`、`devices/#`、`devices/+/write` 等覆盖控制命名空间的宽订阅一律拒绝；改用 `devices/+/data`、`iotp/alerts/#` 之类不覆盖控制主题的过滤器。见 §3、§7.6。
- Q：WS `type=write` 总是返回 `{"code":400,...,"message":"Write Error"}`？A：常见原因是 `[mqtt]` 未启用（`control_unavailable`）、连错节点（必须连网关所在节点）、网关没有精确订阅 `devices/<gw>/write`、没有 `device_gateway` 映射，或设备归属不在本节点。具体 reason 在服务日志与 `control_rejected` 审计里。见 §7.6。
- Q：控制台「网关管理」打开后设备列表是空的，是数据丢了吗？A：0.9.2 起设备列表不在打开页面时全量加载，只在「设备归属」子 tab 点「查询设备/重新查询」后按需拉取，默认 20 行/页。见 §8。
- Q：崩溃重启后仍看到重复行，回放去重没有生效？A：0.8.6 只从源头阻止**新增**重复；升级前已存在的重复、升级前未标记的旧文件、首次快照尚无 meta、node 改名、保留期删除最高 seq、人工篡改等仍在残余范围；meta 不可读时覆盖整体禁用（宁重复不丢行）。用 `collection.wal.replay_*` 观察本次启动跳过量。见 §9.1、§9.8。
- Q：升级后服务拒绝启动，stderr 提到 `[node].id` 含保留标记段？A：0.8.6 起 node id 的 `_` 分段不得为 `iotp3w` 或 `iotp3p<数字>`（否则旧文件名会被误读成覆盖标记），改 node id 后重启。见 §3。
- Q：`merge-once` 脚本只检查退出码 2，为什么有时漏判失败？A：`--config` 路径错误/TOML 解析失败发生在子命令分发前，退出码是 1；应判「非 0」而不是只认 2。见 §9.9。
