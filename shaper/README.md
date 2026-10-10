# Shaper（本地看板服务 + 平台内嵌）

用 [Shaper](https://www.npmjs.com/package/@taleshape/shaper)（taleshape 开源、MPL-2.0）做 IoT 平台的可视化：
SQL 驱动的看板/BI，内置 DuckDB + NATS，单进程自带前端。

> **现状（2026-10）**：平台看板已改为**平台自己渲染**（前端 `CanvasBoard` + `WidgetCard`，数据走后端 `/api/v1/viz/query`），
> 前端**已不再使用 Shaper 的 JS/React SDK 直嵌**，`shaper-react` 依赖与 `ShaperBoard` 组件已移除；后端 `/api/v1/shaper/embed`
> 接口保留但暂无调用方。下面 SDK 那部分保留作参考（例如以后要嵌 Shaper 原生看板时用）。

**（历史）嵌入方式：官方 JS / React SDK，直嵌 DOM，不用 iframe。**

- JS SDK：Shaper 实例自己提供 `GET /embed/shaper.js`（约 2MB），暴露 `window.shaper.dashboard({ baseUrl, container, dashboardId, getJwt, vars, onVarsChanged })`，返回 `{ update, destroy }`
- React SDK：[`shaper-react`](https://www.npmjs.com/package/shaper-react)（MIT）的 `<ShaperDashboard baseUrl id jwt refreshJwt vars onVarsChanged onLoadError className />`，内部就是加载上面的 `embed/shaper.js` 并调用 `window.shaper.dashboard`

## 1. 安装与启动（Docker）

已从 npm 版（`npm install @taleshape/shaper` + 平台原生二进制）**换成官方 Docker 镜像**：

| 项 | 值 |
| --- | --- |
| 镜像 | `taleshape/shaper:0.25.0`（Docker Hub 官方，183MB 压缩 / 680MB 解压） |
| 架构 | **linux/amd64 + linux/arm64 都有**（Apple Silicon 原生跑，不用 Rosetta） |
| 数据 | 挂载 `./data:/data`（镜像默认 `SHAPER_DIR=/data`），npm 版的数据直接复用，看板和 API Key 都还在 |
| 端口 | `127.0.0.1:5454`（Web/API）+ `127.0.0.1:5455`（内置 NATS） |

```bash
cd shaper
docker compose up -d          # 启动
docker compose logs -f        # 看日志
docker compose down           # 停止（数据在 ./data 里，不会丢）
docker compose up -d --pull always   # 升级到最新镜像
```

`docker-compose.yml` 里两个要点：

```yaml
user: "${UID:-501}:${GID:-20}"     # ← 必须：镜像默认以 root 启动，entrypoint 会 chown 数据目录
                                   #   然后 gosu 到 uid 999；那样写不进宿主机 uid 的 ./data
environment:
  SHAPER_CORS_DOMAINS: "127.0.0.1:3000,localhost:3000"   # 平台来源，不放行浏览器侧静默失败
  SHAPER_NATS_HOST: "0.0.0.0"
  SHAPER_NATS_PORT: "5455"        # ← 不指定就不监听 NATS，嵌入的看板会一直 loading
```

配置也可以用命令行参数覆盖（镜像 entrypoint 会自动补 `shaper` 二进制）：`docker compose run --rm shaper --help`。

打开 <http://127.0.0.1:5454>（本地无登录）。数据、日志都在容器里（`docker compose logs`），持久化的只有 `./data`。

> 旧的 npm 安装方式（`npm install` + `npm start`）已卸载；`shaper.log`、`node_modules`、`package.json` 都已删除。

## 1.5 随平台一起起（已接入平台 docker-compose）

Shaper 已加进 `iotserver/manifest/docker/docker-compose.yml`，跟 mqtt-broker / ekuiper / iotserver-core 一个网络：

```bash
cd iotserver/manifest/docker
docker compose up -d shaper        # 只起看板服务
docker compose up -d               # 或整平台（注意：本地开发时平台后端是 go run 起的，端口会冲突）
```

关键点：

| 配置 | 说明 |
| --- | --- |
| `volumes: ../../../shaper/data:/data` | 看板 SQL / API Key / DuckDB 都在这里；与 `shaper/docker-compose.yml` 是同一份，**两者只能起一个** |
| `./db-data/core-data/core.db:/platform-db/core.db:ro` | 平台真实数据（只读） |
| `SHAPER_INIT_SQL_FILE=/data/init.sql` | 启动时执行 [`data/init.sql`](data/init.sql)：装上 sqlite 扩展、`ATTACH` 平台库、建 `v_login_log` / `v_system_metrics` / `v_asset_count` 三个视图 |
| `SHAPER_ALLOWED_DUCKDB_EXTENSIONS=sqlite,json` | 不白名单放行就装不了扩展 |
| `SHAPER_UID/SHAPER_GID` | 数据目录属主；Mac 默认 `501:20`，Linux 服务器一般 `1000:1000` |

平台后端（容器里）用 `[Shaper] BaseUrl = 'http://shaper:5454'` 换 JWT，
浏览器用 `BrowserBaseUrl = 'http://127.0.0.1:5454'` 加载 SDK —— 两个地址不同，所以配置分成两个键。

## 1.5.1 排错：`attempt to write a readonly database`

**现象**：看板所有图表报「系统错误」，Shaper 日志里 `/api/sql` 返回 400，错误是
`Failed to prepare query "SELECT type FROM sqlite_master ...": attempt to write a readonly database`。

**原因**：平台后端（GORM/sqlite）是 **WAL 模式**，异常退出会留下 `core.db-wal`；SQLite 读 WAL 库时需要能写 `-shm`。
早期 compose 用的是**单文件挂载** `./db-data/core-data/core.db:/platform-db/core.db:ro`，
Docker 会把容器内的 `/platform-db` 建成 **root 所有**，容器用户（501）写不了这个目录 → 只读连接失败。

**修复**（已生效）：挂**目录**而不是单文件，容器用户对目录有写权限：

```yaml
# iotserver/manifest/docker/docker-compose.yml （shaper 服务）
# 挂目录而不是单文件：单文件挂载会把容器内 /platform-db 建成 root 所有，
# 容器用户写不了 SQLite 的 -wal/-shm，读 WAL 库会报 attempt to write a readonly database
- ./db-data/core-data:/platform-db
```

改完 `docker compose up -d shaper` 重建即可（`init.sql` 里的路径 `/platform-db/core.db` 不变，仍然是 `READ_ONLY` 挂载）。

排查用：

```bash
docker exec shaper sh -c 'ls -la /platform-db; touch /platform-db/.probe && echo 可写'
# 直接打一条 SQL 看真实报错
curl -s -X POST -H "Authorization: Bearer $JWT" -H 'Content-Type: application/json' \
  -d '{"sql":"SELECT COUNT(*) FROM core.product;"}' http://127.0.0.1:5454/api/sql
```

> 另外：本地配置 `iotserver/cmd/iotserver-core/res/configuration.toml` 的 `[Shaper] Enabled` 必须是 `true`，
> 否则平台侧会直接返回「系统错误」，且**不会**向 Shaper 发任何请求（现象是 Shaper 日志里没有 `/api/sql`）。

## 1.6 数据目录（直接用平台真实表，不用内置指标视图）

图形化配置里的「数据字段」来自**动态内省平台关系库**（30+ 张真实表），不再硬编码视图：

| 能力 | 说明 |
| --- | --- |
| 表/列 | 只读打开 `Databases.Metadata.Primary`（sqlite）读 `sqlite_master` + `PRAGMA table_info`，列出全部表与列（带中文表名），列按角色分类：`time` / `dimension` / `measure` |
| 角色推断 | 数字列 → 指标；文本列 → 维度；`created/modified/*_time/*_at/timestamp` → 时间列 |
| **JSON 派生列** | TEXT 列里存 JSON 时（如 `system_metrics.data`），采样 80 行把**嵌套标量路径**拍平成虚拟列：`data.cpu_used_percent`、`data.memory.used_percent`、`data.network.en0.bytes_sent`…，SQL 里用 `json_extract` 取值 |
| **epoch 毫秒时间列** | INTEGER 的 `created/login_time/timestamp` 按毫秒处理：过滤用 `epoch_ms(CAST(now() AS TIMESTAMP) - INTERVAL ...)`，分组用 `to_timestamp(col/1000)` |
| SQL 表引用 | 平台表挂载在 attach 的 sqlite 目录下，生成 SQL 时统一写 `core.<表名>` |
| 不再提供 | 早期 `v_system_metrics` / `v_login_log` / `v_asset_count` 三个内置视图**已从目录隐藏**（`data/init.sql` 里还留着，仅兼容历史看板，后续可清理） |

**字段精简**（让用户少做选择）：技术字段不展示 —— 主键/外键（`id`、`*_id`）、审计字段（`created`/`modified`）、编码与密钥（`*_key`、`*_token`、`*_secret`、`password`）、大 JSON 原串（`data`、`thing_model_json`）、以及 `sort`/`remark`/`perms`/`path`/`icon` 等技术列。它们仍保留在目录里并带 `hidden: true`（历史看板与服务端默认时间列仍可用），但界面上不出现。

**默认时间列**：每张表在目录里带一个 `timeColumn`（优先非隐藏的时间列，其次隐藏的），用于"设了时间范围 / 折线柱状没选分组字段"时自动分桶，用户不需要理解"时间字段"。

目录带 30 秒缓存；常用表（product/device/thing_model_template/category_template/device_library/sys_login_log/system_metrics…）排前面，方便默认选中。

**当前两块看板都建在真实表上**：

- **平台运行看板**（默认整页）：产品数 / 设备数 / 物模型模板 / 分类模板（KPI）+ 登录趋势（`sys_login_log.login_time` epoch 毫秒）+ 登录最多的账号 + 物模型分类 Top（`thing_model_template.category_name`）+ 驱动库协议分布（`device_library.protocol`）+ 角色表
- **系统监控看板**（运维监控-系统监控）：平均/峰值 CPU、平均内存、平均磁盘（`system_metrics.data.*` JSON 派生列）+ CPU/内存/磁盘/内存占用四条趋势

## 1.7 看板管理（按钮式看板编辑器，参考 app.dash.builders 的 Dashboard）

> **注意参考站有两种产物**：`Canvas`（自由拖拽摆放）和 `Dashboard`（用按钮加组件）。
> 我们采用 **Dashboard** 那一种 —— **不支持拖拽摆放**。

**形态**（对齐 [app.dash.builders/projects](https://app.dash.builders/projects)）：

```
/viz/boards                 项目式卡片列表 + 「新建看板」（名称 + 图标弹窗）
/viz/boards/:id/edit        看板编辑器
   ┌ 工具条：← 返回 | 看板名 | [编辑|查看] | + 添加组件 | 刷新数据 | 保存看板 ┐
   ├ 「+ 添加组件」面板只有三件事（对齐参考站 Add widget，但去掉了输入/筛选块）：
   │    用已有组件   ← 列出**其它看板里已有的组件**，点一下复制过来（可搜索）
   │    + 新建组件   ← 新建一个图表组件
   │    添加文本     ← 文本组件（卡片内直接编辑）
   ├ 组件在 12 列网格里自动紧凑排列；**拖动卡片标题栏即可移动/换位**，右下角可缩放
   └ 组件 ⋯ 菜单：复制组件 / 删除组件 / 查看生成的 SQL
```

- **创建**只能通过「+ 添加组件」按钮（没有左侧拖拽面板、不能把组件"拖"进画布）
- **摆位**拖动卡片**标题栏**即可（`draggableHandle=".ant-card-head"`，`draggableCancel` 排除按钮/输入框），不再提供「上移 / 下移」菜单项
- 尺寸**拖右下角**缩放，位置**拖标题栏**移动（没有上移/下移、也没有宽度预设按钮）；查看态一律不可拖、不可缩放
- 「**编辑 / 查看**」切换对应参考站的 `View`：查看态隐藏配置/菜单/缩放把手
- 空看板提示：*空看板 —— 用右上角「添加组件」开始*（对齐参考站 "Empty dashboard — add a widget from the toolbar."）
- **文本组件**（`type:"text"`）：卡片内直接编辑，不参与查询
- **组件配置只有四件事**：`数据表` → `图形`（数值卡/折线/柱状/饼图/表格）→ `分组字段`（可不选）→ `统计字段`（记录数或某列的合计/平均/最大/最小）
- **时间字段不用选**：设了时间范围、或折线/柱状没选分组字段时，自动用该表的**默认时间列**（`created`/`login_time`/`timestamp`）按天分桶
- 全程**没有 SQL 输入、没有筛选器、没有参数名**；SQL 由平台按配置生成，只在 ⋯ →「查看生成的 SQL」只读展示

### 前端实现要点（`iotserver-ui/src`）

| 关注点 | 做法 |
| --- | --- |
| 渲染入口 | `pages/viz/BoardEdit`（编辑态）、`components/CanvasBoard`（只读态）、`components/WidgetCard`（单卡片） |
| 参数面板 | `components/WidgetConfig`：只有 数据表 / 图形 / 分组 / 统计 四件事；`components/Field` 是共用的表单行 |
| 取数 | `POST /api/v1/viz/query/batch` 一次取回整板；按 `JSON.stringify(spec)` 做签名，配置没变不重复请求 |
| 数值格式 | 后端在结果里返回 `formats`（int/float/percent/bytes/date/text，与 columns 一一对应），前端 `formatValue()` 统一渲染，图表坐标轴与 tooltip 也走同一 formatter |
| 拖动性能 | `WidgetCard` 用 `React.memo` + 引用稳定的 `actions`（操作按组件 id 派发），拖动布局时卡片不重渲染、图表不重画 |
| 图表高度 | `ChartBox` 用 `ResizeObserver` 跟随卡片高度，不再固定 220px |

### 架构：看板由平台渲染，Shaper 只当查询引擎

| | 说明 |
| --- | --- |
| 为什么 | Shaper 的看板是「纵向流式 section」，表达不了网格布局，也无法 per-widget 图形化配置 |
| 渲染 | 平台（antd + `@ant-design/plots` + `react-grid-layout`）按 `canvas.widgets[].layout` 摆放；编辑态可用标题栏拖动 + 缩放（竖向紧凑），查看态全部锁定 |
| 取数 | 后端 `buildVizSQL(spec)` 生成 SQL → 调 Shaper `/api/sql`（管理 JWT）→ CSV 解析 → 行数据 |
| 存储 | `.shaper-boards.json` 里 `kind:"canvas"` 的记录：`canvas.widgets[] = {id,title,layout,spec}` |
| legacy | `kind` 缺省 = `shaper`（早期 SQL 看板）：前端只提示「去看板管理转为平台看板」，不再直嵌 |

### 接口

| 接口 | 说明 |
| --- | --- |
| `GET /api/v1/viz/datasources` | 可选数据视图与列（含 `role` dimension/measure/time、中文 `label`、数值 `format`） |
| `POST /api/v1/viz/query` | 单个组件取数：`{name,spec}` → `{sql,columns,rows}` |
| `POST /api/v1/viz/query/batch` | 整块画布一次取数：`{items:[{id,name,spec}]}` → `{results:[...]}`（并发 4） |
| `POST /api/v1/viz/preview` | 同 query，兼容旧调用（原 `viz/boards/preview`，为避免 gin 静态/参数同段冲突挪到 `viz/preview`） |
| `GET /api/v1/viz/boards` | 列表（canvas 与 legacy shaper 合并；**Shaper 不可用时也能列出 canvas 看板**） |
| `GET /api/v1/viz/boards/:boardId` | 单看板详情（画布编辑器加载用） |
| `POST/PUT /api/v1/viz/boards[/:boardId]` | `{name,kind:"canvas",icon?,canvas}` 只登记；legacy 类型仍走「删旧建新」 |
| `DELETE /api/v1/viz/boards/:boardId` | canvas 只删记录；legacy 连 Shaper 看板一起删 |
| `POST /api/v1/viz/boards/:boardId/active` | 设为「数据看板整页」（`isDefault`；`home` 槽位已无前端消费方） |

### 校验与边界（都在后端强制）

- `view` 白名单、维度/指标/时间列白名单（防注入）；`chart` 只能是 value/line/bar/pie/table；非数值卡必须有维度；选了时间范围必须有时间列
- 布局 clamp：`1 ≤ w ≤ cols`、`x ≥ 0`、`y ≥ 0`、`h ≥ 2`，`x+w > cols` 自动左移；组件 id 唯一；单板最多 60 个组件
- Shaper 不可达 / 管理凭据失效：组件显示错误占位，**不影响保存**（布局与配置照常落库）
- 边界：canvas 类型**不需要** Shaper dashboard、JWT、embed，也不受 Shaper「看板不能重名」限制

### 迁移

早期两块 Shaper DSL 看板（平台运行看板、系统监控看板）已**迁移为平台看板**（各 8 个组件，内容一致；
旧的自由坐标在编辑器里保存一次后会被自动紧凑重排），
legacy Shaper 看板已全部转成平台看板；前端的 embed 代码路径（`ShaperBoard`、`getShaperEmbed`）与 `shaper-react` 依赖已删除。
首页仍然只用平台自己的三块卡片（见 1.8），不参与画布。

## 1.8 首页仪表盘（对齐 demo.winc-link.com，只保留三块）

参考 `https://demo.winc-link.com/home/dashboard`（原 Hummingbird IoT，同一份代码的新版本）重排后，
按需求**只保留 demo 的三块**，其余实验性卡片与看板都已删除：

```
第一行  平台概述（整宽）—— 卡片副标题 + 4 个指标色块（图标/大数字/子标签：本地·第三方、运行中·已停止）
第二行  设备消息总数（宽 16）  |  设备消息量排名（8）
```

| 卡片 | 数据来源 | 说明 |
| --- | --- | --- |
| 平台概述 | `/api/v1/home-page` 的 `pageInfo` | 平台原生接口，样式重做为 demo 的色块风格 |
| 设备消息总数 | 同一个接口的 `msg_gather`（按天的消息总量） | 平台原生组件 `DeviceMessages`（@ant-design/plots 面积图）；本地没有设备流量所以是 0 线，与 demo 一致 |
| 设备消息量排名 | **暂无** | demo 调 `GET /api/v1/devices/topMessage?number=5`，**本地后端没有这个接口**（`msg_gather` 只按天存总量，没有按设备维度）→ 先按 demo 给空状态「暂无设备消息排名数据」，接口就绪后把 `Empty` 换成排行榜即可（见 `components/DeviceMessageRank.tsx` 注释） |

**首页不嵌 Shaper**：首页这两块数据都在平台自己的接口里（消息量在 leveldb/日聚合表，Shaper 读不到），
所以首页回归平台原生组件；Shaper 保留在**两处真正合适的地方**：

- 「数据看板」整页（`/viz/dashboard`）—— 平台运行看板
- 「运维监控 → 系统监控」—— 系统监控看板

为此删掉的实验看板（`运行趋势 · 一小时/24小时/一周`、`登录量排名`、`告警消息`、`系统运行看板`、`告警看板`），
现在只剩两块：`平台运行看板`（默认整页）与 `系统监控看板`。`.env.shaper` 里的首页槽位已清空。

> 如果想恢复「首页用一张 Shaper 看板」的版本，重建对应看板并把 `ShaperBoard slot="home"` 放回首页即可
> （组件与接口都还在，见 1.7 的 `boardName` / `slot` 用法）。原「平台概述」实现备份在
> `iotserver-ui/src/pages/home/Dashboard/components/Overview.original.txt`。

## 1.9 各页面用的看板（现状）

| 位置 | 现在 |
| --- | --- |
| 首页 | **三块平台卡片**（平台概述 + 设备消息总数 + 设备消息排名），不再挂看板 |
| 运维监控 → 系统监控 | 平台看板「系统监控看板」（8 个组件：平均/峰值 CPU、平均内存、平均磁盘 + CPU/内存/磁盘/内存占用四条趋势），原实现备份在 `index.original.txt` |
| 数据看板（整页） | 平台看板「平台运行看板」（9 个组件：产品数/设备数/物模型模板/分类模板 + 登录趋势 + 登录最多的账号 + 模型分类 Top + 驱动库协议分布 + 用户与角色） |

**没有替换的两处**（说明一下，不是漏了）：

- **告警中心的饼图**：画的是「当前条件下这一页告警」的分布（前端用列表数据现算），不是平台存储的看板。
- **历史查询的折线**：画的是用户当次查询返回的时序数据（走查询接口，不在 sqlite 里）。

## 2. 平台内的嵌入（历史，前端已移除）

```
浏览器 ──GET /api/v1/shaper/embed──> iotserver-core ──POST {shaper}/api/auth/token──> Shaper
   ▲                                     （API Key 只在这里）                │
   └──────────── { enabled, baseUrl, dashboardId, jwt } ────────────────────┘
   │
   └─ shaper-react 注入 {baseUrl}/embed/shaper.js → window.shaper.dashboard(...) 渲染到页面 div
```

- 后端：[`controller/http/gateway/shaper.go`](../iotserver/internal/iotserver/core/controller/http/gateway/shaper.go)
  用配置里的 API Key 换 15 分钟短期 JWT（`vars` 支持按用户/组织传变量），**API Key 不出服务端**
- 前端：[`pages/viz/Dashboard/index.tsx`](../iotserver-ui/src/pages/viz/Dashboard/index.tsx) + 路由 `/viz/dashboard` + 侧边栏菜单「数据看板」（菜单写在种子 [`seed/sysmanage.go`](../iotserver/internal/pkg/seed/sysmanage.go)，否则启动迁移会当成遗留菜单清掉）

配置（`iotserver/cmd/iotserver-core/res/configuration.toml`，也支持环境变量覆盖）：

```toml
[Shaper]
Enabled = true
BaseUrl = 'http://127.0.0.1:5454'
ApiKey = ''          # 建议留空，用环境变量注入
DashboardId = ''
```

| 环境变量 | 作用 |
| --- | --- |
| `IOTSERVER_SHAPER_BASEURL` | 覆盖 BaseUrl |
| `IOTSERVER_SHAPER_APIKEY` | 覆盖 ApiKey（推荐，避免密钥进仓库） |
| `IOTSERVER_SHAPER_DASHBOARDID` | 覆盖默认看板 ID |

`Shaper.LocalEnvFile`（默认 `.env.shaper`）是本地未入库的 `KEY=VALUE` 文件；
后端的取值顺序是 **环境变量 > configuration.toml > 该文件**，所以**别人用 `make run` / `go run` 直接起服务也能拿到 API Key**，
不依赖启动时有没有 export 环境变量。

启动示例（推荐用脚本；**即使不跑脚本，后端也会自己去读 `.env.shaper`**，见下面的 `LocalEnvFile`）：

```bash
cd iotserver
cp .env.shaper.example .env.shaper    # 填 IOTSERVER_SHAPER_APIKEY / IOTSERVER_SHAPER_DASHBOARDID（.env.shaper 已在 .gitignore）
./scripts/start-with-shaper.sh        # 或 make start-shaper
```

也可以临时用环境变量直接起：

```bash
cd iotserver
IOTSERVER_SHAPER_APIKEY='shaperkey.xxx.yyy' \
IOTSERVER_SHAPER_DASHBOARDID='<dashboard-id>' \
./iotserver-core -c cmd/iotserver-core/res/configuration.toml
```

### 创建 API Key（需 `jwt` 权限）

Shaper 的 Admin → API Keys 里点，或者：

```bash
curl -X POST http://127.0.0.1:5454/api/keys \
  -H 'Authorization: Bearer <Shaper 管理端 JWT>' -H 'Content-Type: application/json' \
  -d '{"name":"iotserver-embed","permissions":["jwt"]}'
```

### 创建看板

看板内容就是一段 SQL（Shaper 的 DSL：`::SECTION` / `::LABEL` / `::XAXIS` / `::BARCHART` …），
可以在 <http://127.0.0.1:5454/new> 的编辑器里写，也可以直接建：

```bash
curl -X POST http://127.0.0.1:5454/api/dashboards \
  -H 'Authorization: Bearer <Shaper 管理端 JWT>' -H 'Content-Type: application/json' \
  -d '{"name":"演示看板","content":"SELECT '\''标题'\''::SECTION; SELECT '\''指标'\''::LABEL; SELECT col0::XAXIS, col1::BARCHART, FROM ( VALUES ('\''设备A'\'', 12), ('\''设备B'\'', 20) );"}'
```

注意：文本块用 `SELECT '文字';`（`::MARKDOWN` 会编译失败），`FROM ( VALUES ... )` 模板里带尾逗号，照抄即可。

## 3. 前端依赖说明（React 版本）

`shaper-react@0.6.x` 的 peer 是 `react@^19`，本项目是 **React 18.2**：

```bash
cd iotserver-ui
npm install shaper-react --legacy-peer-deps
```

它的实现只用 `useEffect/useState/useRef` + `react/jsx-runtime`（不含 React 19 专有 API），React 18 下实测可用；
但后续 `npm install` 会因为 peer 冲突报 ERESOLVE，需要带上 `--legacy-peer-deps`（或在 `.npmrc` 里加 `legacy-peer-deps=true`）。

另外 SDK 的约定要遵守（否则看板会一直转圈）：

1. `jwt` 初始必须是 `undefined`，SDK 会先回调 `refreshJwt`；
2. 拿到 token 后它用 `useEffect([jwt])` 兑现，所以**每次 refreshJwt 都要换来新的 token 字符串**——
   同一秒内用同一 API Key/变量签发的 JWT 完全相同，依赖不变化就不会兑现。

## 4. 接平台数据（下一步）

DuckDB 能直接读的：平台关系库（sqlite `ATTACH ... (TYPE sqlite)`，就是设备/产品/告警/用户那些表）、MySQL、parquet/csv/json、对象存储（httpfs）。
读不了的：设备时序数据现在落在 leveldb / tdengine（私有格式），要么后续把时序落到可查询的库，要么走 Shaper 的 NATS ingest 推指标。

## 5. 多实例 / 容器尺寸实验页（评估遗留，可删）

`shaper/multi-embed-test.html`：同页放两块 Shaper 看板，验证
① 多实例可共存 ② 各自随容器宽度自适应（拖 splitter 后两块的图表尺寸独立变化）。
用法（临时静态服务 + token 走 URL query，不落盘）：

```bash
python3 -m http.server 3210 --bind 127.0.0.1 --directory shaper   # 需要在 Shaper 的 --cors-domains 里放行 127.0.0.1:3210
# 打开 http://127.0.0.1:3210/multi-embed-test.html?t1=<jwt1>&t2=<jwt2>&d1=<id1>&d2=<id2>
```

## 6. 验证脚本

```bash
python3 .preview/ui_driver.py --base http://127.0.0.1:5454 --script .preview/scenarios/shaper.json
python3 .preview/ui_driver.py --script .preview/scenarios/shaper_embed.json   # 平台内嵌（断言无 iframe、SDK 注入、看板内容渲染）
```
