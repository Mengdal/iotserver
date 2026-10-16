<p align="center">
	<img src="manifest/image/logo.png" width="100">
<br>

</p>

# 产品简介

iotserver 是 Golang 语言实现的超轻量级物联网开发平台，他的核心服务完全由团队自行研发，从这个平台设计之初，轻量级、性能优异当作我们的一条开发准则。所以它服务都是采用Golang语言编写，占用内存极低。在数据存储上，我们创新的使用了SQLite和LevelDB当作了我们存储数据库，得益于SQLite的轻量级和LevelDB对数据的高吞吐和高压缩的特点， 经过实际测试，在1C/1G 100G存储的物理机上，可实现数千设备的连接和8000W条的消息存储。

当然，如果您对数据有可靠行、海量存储等要求，您完全可以把SQLite和LevelDB换为MySQL和TDengine，iotserver 兼容多种数据库连接，用户可以根据自己的业务需要选择适合自己的数据库。

“麻雀虽小，五脏俱全”，虽然 iotserver 作为一个超轻量级物联网平台，但是他的核心功能并不会消减。设备接入、产品管理、物模型、告警中心、规则引擎...这些所有物联网平台的标配功能，都可以在本平台中找到。



![](manifest/image/architecture.png)



# 部分页面展示

| ![](manifest/image/1.png) | ![](manifest/image/2.png)  |
|:--------------------------|:---------------------------|
| ![](manifest/image/3.png) | ![](manifest/image/4.png)  |
| ![](manifest/image/5.png) | ![](manifest/image/6.png)  |
| ![](manifest/image/9.png) | ![](manifest/image/10.png) |
| ![](manifest/image/7.png) | ![](manifest/image/8.png)  |

# 小程序页面展示

| ![](manifest/image/miniapp-merge1.png) | ![](manifest/image/miniapp-merge2.png) |
|:---------------------------------------|:---------------------------------------|
| ![](manifest/image/miniapp-merge3.png) | ![](manifest/image/miniapp-merge4.png) |

# 快速启动

前置：Docker（含 buildx）、Node 18+、Go 1.24（本机开发用）。

## 1. 构建镜像（从本仓库源码构建）

```bash
# 前端产物：Dockerfile 会把 iotserver/cmd/iotserver-ui/build 打进镜像，必须先构建
make ui

# 构建并推送镜像（IMAGE 换成自己的仓库地址；只本地用可去掉 --push）
make docker IMAGE=<你的仓库>/iotserver-core:v1.0

# 仅本地构建、不推送：
docker buildx build --platform linux/amd64 -t iotserver-core:v1.0 \
  -f cmd/iotserver-core/Dockerfile --load .
```

## 2. 启动

```bash
cd manifest/docker
docker compose up -d
# 镜像名不同时用环境变量覆盖（compose 默认 iotserver-core:v1.0）
IOTSERVER_IMAGE=<你的仓库>/iotserver-core:v1.0 docker compose up -d
```

## 3. 访问

| 入口 | 地址 | 说明 |
| --- | --- | --- |
| 平台（前端 + API） | `http://<host>:3000` | 默认账号 `admin` / `admin123` |
| 内部 API | `http://<host>:58081` | eKuiper 规则回调平台用；容器内走 `http://iotserver-core:58081` |
| eKuiper REST | `http://127.0.0.1:9081` | compose 只绑定本机回环 |
| eKuiper Manager | `http://<host>:9082` | 规则/流可视化管理 |

## 4. 数据与持久化

| 数据 | 位置 | 说明 |
| --- | --- | --- |
| 元数据（用户/菜单/设备/规则/场景…） | `manifest/docker/db-data/core-data/core.db` | SQLite（已开 WAL） |
| 历史数据（数据中心） | `manifest/docker/db-data/ducklake/` | DuckLake：SQLite catalog + Parquet 数据文件 |
| eKuiper 数据 | `manifest/docker/kuiper/` | 规则、流与运行数据 |

> ⚠️ DuckLake 首次启动需要联网安装 `ducklake` 扩展（装在 catalog 同目录的 `extensions/`，之后可离线）。
> 纯离线环境请预热该目录，或把 `Databases.Data.Primary.Type` 改成 `leveldb` / `tstorage`。

## 5. 常用运维命令

```bash
make test                                  # 后端全部测试
go run ./tools/lakectl stats               # DuckLake 行数 / 快照数 / 数据文件数
go run ./tools/lakectl flush|purge|compact # 刷盘 / 按保留期清理 / 压实（需先停服务）
go run ./tools/gen-permissions             # 调整权限后重新生成前端权限常量
make start                                 # 用 go run 直接启动（本机开发；启动前自动保证前端产物最新）
make ui                                    # 前端：构建 + 同步到 cmd/iotserver-ui/build
make sync-ui                               # 前端：只同步（已构建过、只差拷贝时用）
```

> `make start` / `make run` 会先检查前端：源码比产物新就自动 `make ui`（构建 + 同步），
> 否则只做必要的同步或直接跳过——不会出现"重启了但页面还是旧的"。
> 前端产物是服务运行时从磁盘读的，所以只改前端时不用重启服务，浏览器强刷即可。

## 6. 关键配置（`cmd/iotserver-core/res/configuration.toml`）

| 配置项 | 说明 |
| --- | --- |
| `Databases.Data.Primary.Type` | 数据层：`ducklake`（默认）/ `leveldb` / `tstorage` / `tdengine` |
| `Databases.Data.Primary.DataSource` / `Dsn` | DuckLake 的 catalog（SQLite）与 Parquet 数据目录 |
| `Service.CallbackUrl` | eKuiper 规则动作回调平台的基地址；容器内用服务名，默认 `http://iotserver-core:58081` |
| `Service.CallbackToken` | 回调共享密钥；非空时回调必须带 `x-callback-token`（规则动作自动携带）。改后重启服务会自动重下发运行中规则的动作 |
| `Pprof.Enabled` | 调试端点 `/debug/pprof`，默认 `false`；开启后需 `x-token` + `system:debug:pprof` 权限 |
| `Viz.BoardsFile` | 历史字段（已不读取）：看板早已落库（`viz_board` 表、按租户隔离），新装自带内置看板（首页/演示/监控，编译期内嵌，见 `internal/pkg/seed/boards`） |

# 主要功能

- 产品管理
- 设备管理
- 网关管理（第三方工业网关 MQTT 接入：实时上报 + 断点续传补传 + 配置/命令下发）
- 机器模板（7 个内置工业模板：冷库/压缩机/灌装机/包装机/注塑机/产线/能源监测，一键生成产品+告警）
- 租户管理（多租户：一个实例服务多个租户，行级数据隔离，平台管理员可切换代管）
- 镜像管理
- 驱动管理
- 驱动市场
- 系统监控
- 告警中心
- 资源管理
- 规则引擎
- 场景联动
- 插件市场
- 数据大屏
- 系统管理（用户 / 角色 / 菜单 / 部门 / 审查日志 / 系统配置；个人中心在顶栏头像下拉）

# 多租户与网关接入

## 多租户（租户隔离）

- 平台管理员（内置 `admin`，不属于任何租户）在「租户管理」里创建租户，顶栏可切换「当前租户」代管；
  切换后前端会带 `x-tenant-id` 请求头，后端只放行平台管理员使用该头。
- 数据隔离由 GORM 租户插件执行：`models.TenantScopedTableNames()` 登记的表会自动追加 `tenant_id` 条件，
  业务代码通过 `dbClient.WithCtx(ctx)` 携带租户上下文；未携带上下文时按 `[Tenant] Enforce` 配置处理
  （`false`=迁移期仅告警，`true`=fail-closed 返回空集）。
- 组织树：租户 → 工厂 → 车间 → 产线（`sys_dept.dept_type`），设备/网关可挂到具体节点。
- 隔离回归测试：`internal/iotserver/core/e2e/tenant_isolation_test.go`（租户间列表/详情/改删/看板/平台接口越权）。

## 网关接入

第三方工业网关负责 PLC 协议采集与本地缓存，平台侧通过 MQTT 接入：

- 主题约定：上行 `iot/up/{sn}/{data|batch|status|offline}`，下行 `iot/down/{sn}/{config|cmd}`；
- **断点续传**：`batch` 报文逐条带原始采集时间（毫秒），平台按 `(gateway_id, seq)` 幂等去重，
  数据落到采集时刻而不是接收时刻；
- 接入规范（可直接发给网关厂商）：[docs/gateway-integration.md](docs/gateway-integration.md)
- 回归测试：`internal/iotserver/core/application/gatewayingest/service_test.go`、
  `internal/iotserver/core/application/persistence/batch_backfill_test.go`


# 系统管理（RBAC）

系统管理模块位于前端导航「系统配置」下，包含 6 个子模块：

| 前端路由               | 功能     | 主要接口                                                       |
| ---------------------- | -------- | -------------------------------------------------------------- |
| `/admin/sysUser`       | 用户管理 | `GET/POST/PUT/DELETE /api/v1/sys-user`、`PUT /api/v1/pwd/reset/:id` |
| `/admin/sysRole`       | 角色管理 | `/api/v1/role`、`/api/v1/menurole`                              |
| `/admin/sysMenu`       | 菜单管理 | `/api/v1/menu`、`/api/v1/menu/tree`                             |
| `/admin/sysDept`       | 部门管理 | `/api/v1/dept`、`/api/v1/dept-tree`                             |
| `/admin/sysAuditLog`   | 审查日志（登录 + 操作） | `/api/v1/sys-audit-log`、`/api/v1/sys-audit-log/clear`        |
| `/admin/sysSet`        | 系统配置 | `/api/v1/system-setup`、`/api/v1/system-setup/save`             |

> 个人中心不单独建菜单：入口固定在顶栏头像下拉（`GET /api/v1/user/getinfo`、`PUT /api/v1/user/profile`），页面路由仍为 `/common/userProfile`。

**权限模型**

- 用户通过 `sys_user_role` 关联角色，角色通过 `sys_role_menu` 关联菜单，菜单的 `perms` 字段即接口权限标识（例如 `system:user:add`）。
- 前端登录后按「用户可见菜单 + 权限标识」动态渲染导航与操作按钮。
- 后端在每个系统管理接口前挂载 `middleware.PermRequired` 中间件做接口级鉴权；拥有内置角色 `admin`（`role_key = admin`）的账号直通全部接口。
- 内置管理员账号 `admin` 与内置角色 `超级管理员` 受保护：不可删除、不可停用。

**全站权限覆盖**

除系统管理外，产品/设备/驱动/镜像/告警/监控/文档等模块的接口同样已接入权限校验，共覆盖 97 个接口：

| 模块 | 权限前缀 | 覆盖接口 |
| --- | --- | --- |
| 产品管理 | `gateway:product:*` | 产品增删改查、物模型维护 |
| 设备管理 | `gateway:device:*` | 设备增删改查、导入、绑定驱动/产品、物模型数据查询 |
| 驱动镜像 | `gateway:image:*` | 镜像仓库配置增删改查 |
| 我的驱动 | `gateway:drive:*` | 驱动库与驱动实例的增删改查 |
| 系统监控 | `monitor:system:view` | 系统指标 |
| 告警中心 | `monitor:alert:*` | 告警规则、告警列表、忽略/处理 |
| 工具与文档 | `docs:view` | SDK 文档 |
| 首页 | `home:view` | 首页概览 |

**权限码单一来源**

权限标识只在 `internal/pkg/permissions/permissions.go` 定义一次：

- 后端路由用 `middleware.Perm(...)` 引用；
- 数据库菜单与按钮权限由 `internal/pkg/seed` 依据同一份定义生成；
- 前端常量由 `go run ./tools/gen-permissions` 生成到 `iotserver-ui/src/routes/permissions.ts`。

新增模块时只需：① 在 `permissions.go` 加权限码；② 在 `seed` 里加菜单与按钮；③ 路由挂 `middleware.Perm`；④ 重新生成前端常量并给前端菜单项标注 `perm`。

**建表与初始化数据**

服务启动时会自动执行 `AutoMigrate` 创建 `sys_user_role`、`sys_role`、`sys_role_menu`、`sys_menu`、`sys_dept`、`sys_login_log`、`sys_config` 表，并为 `user` 表补齐系统管理所需字段，同时幂等地写入：

- 内置部门「蜂鸟物联网」
- 「系统配置」目录及 7 个子菜单，含全部按钮权限
- 内置角色「超级管理员」并授予全部菜单
- 默认管理员账号 `admin / admin123`（仅当 `admin` 账号不存在时创建）
- 默认系统配置项（平台名称、会话超时、密码策略、数据保留等）

# 产品优势

- **免费开源**：我们源代码全部开源，可供个人学。
- **性能优异**：经过实际测试，在2C4G主机上，可支持60k的设备连接。模拟4k设备以1000ms的间隔向服务端发送16kb长度的属性数据，cpu使用率稳定在百分之60、内存使用率稳定在百分之16。
- **多数据支持**：平台支持SQLite、LevelDB和MySQL、TDengine数据库，用户可以根据自己的项目需要，选择适合自己的数据库。
- **支持可视化大屏**：用户只需要进行简单拖拽，无需写任何代码，即可完成可视化大屏开发设计。
- **内置产品模版库**： 平台内部预置产品模版（390条），帮助用户一键创建产品。
- **文档清晰**：平台文档清晰完整，文档内部有大量使用案例和视频教程，帮助用户快速理解产品和使用产品。
- **部署简单**：平台所有服务都是以docker形式运行，支持docker-compose命令一键启动。
- **成本优势**：在中小型物联网项目（设备数<=5000）中，只需要2C4G主机即可，并且还可以支持本地部署。

#  联系我们

我们开通了交流群，在使用在本品的过程中有任何问题，可以进群沟通。


| 微信群                (先加好友备注加群)                                                     | QQ群                                                                       |
|:------------------------------------------------------------------------|:--------------------------------------------------------------------------|
| ![](manifest/image/qweixin.png) | ![](manifest/image/qq.png) |

# 开源贡献

-   从 `master` 分支 `checkout` 一个新分支（**注**：请务必保证 `master` 代码是最新的）
-   新分支命名格式：`feature/username/description`，例如：`feature/pick/mqtt_driver`
-   在新分支上编辑文档、代码，并提交代码
-   提交 `PR` 合并到 `develop` 分支，等待作者合并即可

# 开源协议

`iotserver` 开源平台遵循 [Apache 2.0 协议](https://www.apache.org/licenses/LICENSE-2.0.html) 如果你想把 iotserver 平台进行商用，请提前找作者授权！

## 时序数据后端

`configuration.toml` 的 `[Databases.Data.Primary]` 支持两种数据层：

| Type | 说明 | 额外配置 |
| --- | --- | --- |
| `duckdb` | **默认**，单文件列存 | `DataSource` = 数据库文件路径 |
| `ducklake` | DuckDB + Parquet 湖仓 | `DataSource` = catalog 路径，`Dsn` = Parquet 目录 |

两者都使用官方 DuckDB Go 驱动，要求 **Go ≥ 1.24** 且 **CGO 开启**，运行镜像需 glibc。

`duckdb` 把数据写在 DuckDB 原生表里：一个事务不再产生一个 Parquet 文件，因此不需要分区、
合并小文件、过期快照那一整套文件生命周期管理，保留策略只需 `DELETE` + `CHECKPOINT`。

`ducklake` 额外依赖 ducklake 扩展（首次需联网安装，之后缓存在 catalog 同级的 `extensions/`
目录，可离线预置）。

> 单写者约束对两者都成立：同一个数据库文件 / 同一个湖，同一时刻只允许一个进程打开，
> 因此 core 无法水平扩展，需要单实例 + 主备或定期快照来兜。
