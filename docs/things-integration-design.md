# IoTServer × things 集成设计

> 状态：Draft v1
> 日期：2026-10-26
> 范围：租户与 things 项目绑定、网关配置接入、本地镜像与同步
> 参考系统：things / iotddc（`things.iotddc.com/api`）

---

## 1. 结论摘要

1. **一个 IoTServer 租户绑定一个 things 项目**，项目是网关归属和权限边界。
2. **things 是网关配置的唯一事实来源**；IoTServer 不复制其业务规则。
3. **平台管理员只配置一个 things 服务账号**（账号/密码），所有租户共用；租户不接触 things 账号。
4. **网关认证参数全部由用户/公司提供并原样透传**。IoTServer 不计算 `secret`、不校验 `license`、不判断试用/正式规则。
5. **IoTServer 保留一份网关本地镜像**，用于 `/edge` MQTT 上行归属、租户隔离、设备注册和断线续传；镜像不是第二套配置源。
6. **控制面与数据面分离**：
   - 控制面：IoTServer → things HTTP API，管理项目/网关。
   - 数据面：网关 → things MQTT `/edge/...` → IoTServer `edgeingest`。
7. **things 故障不阻断已绑定网关的数据上行**；新增/删除等控制面操作失败时不做本地半成功。

---

## 2. 背景与目标

### 2.1 背景

IoTServer 当前已有：

- 多租户行级隔离；
- 网关本地管理表 `gateway`；
- 兼容 things `/edge` 协议的接入服务 `application/edgeingest`；
- 参考系统网关的重启、属性写、标签树同步能力（走 MQTT）。

现状里的“网关同步”是为了快速接入数据而做的本地登记，不是真正的网关生命周期管理。后续希望：

- 租户创建/删除时同步创建/删除 things 项目；
- 租户新增网关时直接调用 things 的网关创建接口；
- things 中已有的网关可以自动进入 IoTServer；
- 不再维护一套与 things 重复的密钥/授权逻辑。

### 2.2 目标

- 租户 ↔ things 项目 1:1 绑定，可查询绑定状态。
- 平台管理员统一配置 things 服务账号。
- 租户新增/删除网关时，同步操作 things。
- things 中直接新增的网关，能被 IoTServer 自动发现并纳入正确租户。
- 网关删除、项目删除有明确的一致性和失败处理。
- 所有敏感字段不落日志、不回显。
- 保留现有 `/edge` 数据接入链路，不因 things HTTP 故障中断。

### 2.3 非目标

- 不在 IoTServer 内实现 things 的密钥算法、License 规则、试用期规则。
- 不在 IoTServer 内重建 things 的网关运维控制台。
- 不把 things 的组织（Organization）作为租户映射；本期只用项目（Project）。
- 不实现一个租户绑定多个项目。
- 不自动删除本地设备、测点、时序数据来“跟随” things 删除。
- 不实现 things 多实例并发调用（见 §12 约束）。

---

## 3. 职责边界

这是本设计最重要的部分。

| 能力 | 负责方 | IoTServer 的行为 |
| --- | --- | --- |
| 租户隔离 | IoTServer | 本地 `tenant_id` 行级隔离 |
| 项目归属 | things | 只保存 `project_id`，不重算 |
| 项目创建/删除 | things | 调用接口、记录结果 |
| 网关归属项目 | things | 以 things 的 `project_id` 为准 |
| 网关 SN/名称/型号 | things | 从 things 读取/写入 |
| 网关 `secret` | 公司提供 + things 校验 | **原样透传，不计算、不校验、不持久化** |
| 网关 `license`/`licenseType` | 公司提供 + things 校验 | **原样透传，不解释** |
| 网关在线状态 | IoTServer 数据面 | 由 `/edge` 报文/LWT 更新，不用 things 列表覆盖 |
| `/edge` 数据接入 | IoTServer | 按本地镜像订阅主题、归属租户 |
| 子设备/测点注册 | IoTServer | 由 `/edge/connect` 清单驱动 |
| 设备/测点历史 | IoTServer | 不因 things 删除而删除 |

一句话：

> **things 管“网关是什么、属于哪个项目、授权是否有效”；IoTServer 管“数据属于哪个租户、如何落库、如何展示”。**

### 3.1 明确不做的事情

- 不计算 `MD5(sn + 固定盐)`。
- 不判断 `secret` 是否符合某种格式。
- 不判断 `licenseType` 只能是 trial/permanent。
- 不判断试用期是否到期。
- 不因为 license 问题在本地拦截网关创建；由 things 返回错误，IoTServer 翻译提示。
- 不把 things 返回的 `secret`、`license` 写入日志或接口响应。

---

## 4. 总体架构

```text
┌──────────────────────────────┐
│        IoTServer UI          │
│  租户管理 / 网关管理 / 系统配置 │
└──────────────┬───────────────┘
               │ HTTPS + JWT
┌──────────────▼───────────────┐
│      IoTServer Core API      │
│  tenantapp / gatewayapp      │
│  thingsapp（绑定 + 同步）      │
└───────┬───────────────┬──────┘
        │               │
        │ 控制面         │ 本地读
┌───────▼────────┐  ┌───▼─────────────────┐
│ things HTTP API│  │ SQLite 本地镜像       │
│ /edge-user     │  │ tenant_things_binding│
│ /edge-mgmt     │  │ gateway              │
└───────┬────────┘  └───▲─────────────────┘
        │               │ 订阅主题/租户归属
        │               │
┌───────▼────────┐  ┌───┴─────────────────┐
│ things MQTT    │  │ edgeingest           │
│ /edge/...      │──▶│ 上行解析/设备注册     │
└───────▲────────┘  └─────────────────────┘
        │
    网关设备
```

关键点：

- HTTP API 只用于管理面和同步，不参与实时数据。
- `edgeingest` 仍然按本地 `gateway.pk + sn` 订阅 MQTT 主题。
- 即使 things HTTP 暂时不可用，已存在的本地镜像仍能接收数据。
- 本地镜像必须存在，否则无法判断 `/edge/{pk}/{sn}` 属于哪个租户。

---

## 5. 领域映射

### 5.1 租户 ↔ 项目

```text
IoTServer Tenant 1 ──── 1 things Project
```

选择项目而不是组织的原因：

- things 的 `edge_gateway.project_id` 是必填，网关天然归属项目。
- `Organization` 是“用户可访问哪些项目”的层级，不是网关归属。
- 一个租户对应一个项目，权限判断简单、边界清晰。
- 后续如果公司要求项目挂到组织下，只需在 things 侧配置，不影响 IoTServer 映射。

字段映射：

| IoTServer | things | 说明 |
| --- | --- | --- |
| `tenant.id` | 不直接对应 | 本地主键 |
| `tenant.code` | 可作为项目名候选 | 租户编码 |
| `tenant.name` | `project.name` | 项目名需满足 things 的 4~30 字符且同账号唯一 |
| `tenant.remark` | `project.memo` | 可选同步 |
| `tenant.status` | 无 | 仅控制 IoTServer 访问，不影响 things |
| `binding.things_project_id` | `project.id` | 稳定外键 |
| `binding.things_project_name` | `project.name` | 便于展示/对账 |

**项目名规则**：things `ProjectController.create` 要求 `@Length(min=4,max=30)`，且同一账号下不能重名。IoTServer 租户名可能不满足，因此租户新增表单需要单独提供“things 项目名称”，默认用租户名预填，由平台管理员确认。

### 5.2 网关

| IoTServer `gateway` | things `edge_gateway` | 说明 |
| --- | --- | --- |
| `id` | 无 | 本地主键 |
| `sn` | `sn` | 全局业务键；本地仍要求全局唯一 |
| `pk` | `pk` | things 创建时 `pk = sn`；本地用于 `/edge` 主题 |
| `name` | `name` | things 管理 |
| `model` | `model` | things 管理，创建后通常不可改 |
| `protocol` | 固定 `edge` | things 网关走 `/edge` 协议 |
| `secret` | `secret` | **不回填、不持久化到本地** |
| `status` | 无直接等价 | 由本地 `/edge` 数据面维护 |
| `things_gateway_id` | `gateway.id` | 新增绑定字段 |
| `things_project_id` | `gateway.project_id` | 新增绑定字段 |
| `source` | 无 | `local` / `things` |
| `things_sync_status` | 无 | 同步状态 |

### 5.3 本地镜像的边界

本地镜像只保存 IoTServer 运行必需的信息：

- 用哪个 `pk + sn` 订阅 `/edge`；
- 这个网关属于哪个租户；
- 网关下设备/测点如何注册；
- 本地组织、备注、自动注册开关等运行参数。

本地镜像**不保存**：

- things 的 `secret`（除非后续确认 `/edge` 本地接入确实需要，目前不需要）；
- `license`、试用期、授权状态；
- things 的账号密码、token。

---

## 6. 数据模型

### 6.1 新增：`tenant_things_binding`

平台级绑定表，记录租户与项目的一一对应。

```text
id                  text primary key
tenant_id           text not null unique
things_project_id   integer not null
things_project_name text not null
status              text not null   -- pending / bound / error / orphaned / deleting
last_sync_at        integer         -- 毫秒
last_error          text
created             integer
modified            integer
```

约束：

- `tenant_id` 唯一；
- `things_project_id` 唯一（一个项目不能被两个租户绑定）；
- 状态机见 §10.3。

建议模型放在 `internal/models/tenant_things_binding.go`，加入 `AutoMigrate` 清单。

### 6.2 扩展：`gateway`

新增列：

```text
things_gateway_id   integer
things_project_id   integer
source              text default 'local'   -- local / things
things_sync_status  text default 'local'   -- local / synced / pending / missing / conflict / error
things_last_sync_at integer
things_last_error   text
```

说明：

- `source=local`：平台自有 MQTT 网关，继续走现有创建逻辑。
- `source=things`：things 管理的 `/edge` 网关，`protocol=edge`。
- `things_sync_status` 只表示“本地镜像与 things 是否一致”，不表示网关在线状态。
- 保留现有 `status`（在线/离线/从未上线）字段，由数据面维护。

### 6.3 不需要的表

- 不需要 `things_token` 表：token 只放内存。
- 不需要 `things_gateway_license` 表：不管理 license。
- 不需要 `things_organization` 表：本期不映射组织。

---

## 7. things 客户端设计

### 7.1 配置项

新增系统配置组 `things`，仅平台管理员可写：

| key | 说明 | 默认 |
| --- | --- | --- |
| `things.enabled` | 是否启用 things 集成 | `false` |
| `things.baseUrl` | things API 根地址 | `https://things.iotddc.com/api` |
| `things.username` | 服务账号 | 空 |
| `things.password` | 服务账号密码，加密存储、只写不回显 | 空 |
| `things.timeoutSeconds` | HTTP 超时 | `10` |
| `things.reconcileIntervalSeconds` | 镜像对账间隔 | `300` |
| `things.tokenRefreshMinutes` | 提前刷新 token 的分钟数 | `90` |

要求：

- `things.password` 使用 AES-GCM 加密后写入 `sys_config`，密钥来自部署环境变量/本机密钥文件，不硬编码在源码。
- `GET` 系统配置时密码字段返回掩码，例如 `******`。
- 保存空值表示“不修改”，而不是清空。
- 提供“测试连接”按钮：登录 + 拉取项目列表，失败展示 things 原始错误（脱敏后）。

### 7.2 认证与 token 管理

things 网关的规则（来自源码）：

- 登录：`POST /edge-user/user/apiLogin`，参数 `username`、`password`。
- 认证头：`Token: <token>`。
- token 空闲 2 小时后失效（`SESSION_RETAIN_HOUR=2`），每次请求会续期。
- **同一个账号再次调用 `apiLogin` 会让旧 token 立即失效。**
- 鉴权失败时 HTTP 状态仍可能是 200，响应体是 `{"code":401,"message":"UNAUTHORIZED"}`。

因此 TokenManager 必须：

1. 进程内单例，互斥锁保护。
2. 缓存 token + 最近使用时间。
3. 距离上次使用超过 `tokenRefreshMinutes` 时主动重新登录。
4. 收到 `code=401` 时，单飞（single-flight）重新登录并重试一次。
5. 重试仍失败则返回 `things_auth_failed`，不无限重试。
6. 不把 token 写入数据库、日志、接口。
7. 登录失败要区分“账号密码错误”和“things 不可达”。

建议接口：

```go
type TokenManager interface {
    Token(ctx context.Context) (string, error)
    Invalidate(token string)
}
```

### 7.3 HTTP 客户端

建议包结构：

```text
internal/tools/things/
  client.go       # HTTP + envelope + token
  token.go        # 单飞 token 管理
  types.go        # Project / Gateway / envelope
  errors.go       # 错误码映射
internal/iotserver/core/application/thingsapp/
  service.go      # 绑定、项目、网关、对账
internal/iotserver/core/interface/things.go
```

客户端方法：

```go
Login(ctx) (token string, err error)

ListProjects(ctx, page, size, name) ([]Project, total, error)
CreateProject(ctx, name, description) (Project, error)
UpdateProject(ctx, projectID, name, description) (Project, error)
DeleteProject(ctx, projectID, password) error

ListGateways(ctx, projectID, page, size) ([]Gateway, total, error)
CreateGateway(ctx, req CreateGatewayRequest) (Gateway, error)
UpdateGateway(ctx, req UpdateGatewayRequest) (Gateway, error)
DeleteGateway(ctx, projectID, gatewayID, password) error
```

`CreateGatewayRequest` 直接对应 things 参数，不做业务加工：

```go
type CreateGatewayRequest struct {
    ProjectID   int64
    SN          string
    Secret      string
    Name        string
    Model       string
    LicenseType string
    License     string
}
```

### 7.4 接口清单

以实测/源码为准，统一使用 `application/x-www-form-urlencoded`：

| 用途 | 方法 | 路径 |
| --- | --- | --- |
| 登录 | POST | `/edge-user/user/apiLogin` |
| 项目列表 | POST | `/edge-user/project/list` |
| 项目详情 | POST | `/edge-user/project/detail` |
| 创建项目 | POST | `/edge-user/project/create` |
| 修改项目 | POST | `/edge-user/project/setting` |
| 删除项目 | POST | `/edge-user/project/delete` |
| 网关列表 | POST | `/edge-mgmt/gateway/list` |
| 创建网关 | POST | `/edge-mgmt/gateway/create` |
| 修改网关 | POST | `/edge-mgmt/gateway/modify` |
| 删除网关 | POST | `/edge-mgmt/gateway/delete` |

注意：

- `project/delete`、`gateway/delete` 都需要 `pwd`，传服务账号密码。
- `gateway/create` 的 `secret` 由 things 校验，错误通常是 `Secret error`；`license` 错误是 `License error`。IoTServer 原样透传并提示，不自行判断。
- `gateway/list` 返回中可能包含 `secret` 等敏感字段，**IoTServer 解析时必须白名单取字段**，只保留本地需要的 id/sn/pk/name/model/projectId 等。
- 项目名长度 4~30，且同一服务账号下唯一。

### 7.5 响应与错误

统一 envelope：

```json
{
  "code": 200,
  "message": "success",
  "data": {}
}
```

处理规则：

- 只有 `code == 200` 视为成功。
- `code == 401`：token 失效，刷新后重试一次。
- `code == 403`：权限不足，提示检查服务账号。
- 其他 `code`：视为业务错误，保留 `message` 作为可读详情。
- HTTP 非 2xx：网络/网关错误，返回“things 平台不可用”，不伪造业务错误。
- 超时：不自动重试写操作（避免重复创建），转为对账/查询确认。
- 所有错误都经过敏感信息清洗，不能带出 password/token/secret/license。

---

## 8. 业务流程

### 8.1 平台管理员配置 things

1. 平台管理员进入“系统配置 → things 平台”。
2. 填写地址、服务账号、密码，保存（密码加密）。
3. 点击“测试连接”。
4. 后端执行 `apiLogin` + `project/list`，返回成功/失败。
5. 成功后 `things.enabled` 才允许开启；也可以保存但暂不启用。

约束：

- 服务账号必须是**专用账号**，不要和人工登录 things 控制台共用。
- 服务账号应当是项目/网关的管理员，且所有租户项目都归属于它。
- 服务账号密码同时用于项目/网关删除，必须妥善保管。

### 8.2 新建租户

目标：本地租户和 things 项目同时成功，或都不成功。

```text
1. 校验租户编码、名称、配额、项目名称（4~30、未绑定）
2. 调 things project/create
3. 成功后本地事务：
   - 创建 tenant
   - 创建根组织节点
   - 创建 tenant_things_binding(status=bound)
4. 若本地事务失败：
   - 尝试 project/delete 补偿
   - 补偿失败则记录 orphaned 项目，后台对账提示人工处理
5. 返回租户 + things 项目信息
```

失败策略：

- things 调用失败：不创建本地租户，直接返回错误。
- 本地写失败：尽量补偿删除项目；不能静默留下“本地无、things 有”的孤儿项目。
- 网络超时：先按项目名重新查询一次；查到则视为成功并接管，避免用户重试产生重复项目。

项目名：

- 必填，默认取租户名称；不满足 4~30 或重名时要求修改。
- 推荐同时展示租户编码，避免同名。

### 8.3 修改租户

本地字段（名称、联系人、配额、状态、备注）直接更新本地。

things 拥有的字段（项目名称、备注）单独处理：

- 提供显式“重命名 things 项目”操作，调用 `project/setting`。
- 失败时本地租户信息不回滚，绑定表记录 `last_error`，UI 显示“项目名称未同步”。
- 不把租户停用/启用同步到 things；`tenant.status` 只控制 IoTServer 登录和访问。

这样避免“改个联系人电话因为 things 故障而失败”。

### 8.4 删除租户

前置条件（保留现有规则）：

- 该租户下没有用户、设备、网关、产品、自定义看板。
- 内置看板不阻止删除。

流程：

```text
1. 平台管理员二次确认（明确提示会删除 things 项目）
2. 本地统计并校验非空
3. 调 things project/delete(projectId, servicePassword)
4. 成功或项目已不存在 → 删除本地 tenant + binding + 菜单定制 + 内置看板
5. 失败 → 保留本地租户，返回 things 错误
```

幂等：

- things 返回“project not exist”时视为已删除，继续本地删除。
- 本地删除失败但 things 已删除：绑定标记 `orphaned`，提示管理员重试本地清理。

### 8.5 新增网关

分两种模式：

- `source=local`：平台自有 MQTT 网关，保留现有逻辑。
- `source=things`：things 管理的 `/edge` 网关，走本流程。

things 网关新增：

```text
1. 从当前租户绑定解析 project_id（绝不接受前端传 projectId）
2. 校验 SN 全局未被占用
3. 调 things gateway/create，原样传：
   projectId, sn, secret, name, model, licenseType, license
4. 成功后在本地创建 gateway 镜像：
   protocol = edge
   pk = things 返回的 pk（若为空则用 sn）
   source = things
   things_gateway_id / things_project_id
   things_sync_status = synced
5. 返回网关信息；不返回 secret/license
```

失败：

- things 返回 `Secret error` / `License error` / 名称重复 / SN 已被其他项目绑定：原样翻译后返回。
- 网络超时：按 `projectId + sn` 查网关列表确认；查到则接管，查不到则提示重试。
- 本地写失败：things 已创建，绑定记为 `pending`，后台对账补建；不要重复调用 create。

前端字段：

- SN、名称、型号：必填。
- `secret`：密码框，公司提供，提交后不回显。
- `licenseType`：按 things 支持的选项提供，不做本地判断。
- `license`：可选，公司提供。

> 不在前端展示“平台生成的 MQTT 密码”。things 网关的数据面走 things MQTT `/edge`，不是 IoTServer 自有 MQTT。

### 8.6 修改网关

拆分两类字段：

| 类型 | 字段 | 行为 |
| --- | --- | --- |
| things 拥有 | 名称、型号、secret、license | 只读；名称可通过显式“在 things 重命名”修改 |
| 本地运行 | 组织节点、自动注册、排除设备、备注 | 直接改本地 |

原则：

- 型号、secret、license 不提供编辑入口；需要变更去 things 控制台。
- 网关名称如果允许修改，调用 `gateway/modify`，只传 `name`；失败则拒绝本地改名，避免不一致。
- 本地运行参数不调用 things。

### 8.7 删除网关

```text
1. 平台/租户确认（提示会删除 things 网关）
2. 调 things gateway/delete(projectId, gatewayId, servicePassword)
3. 成功或已不存在 → 删除本地 gateway 记录
4. 本地设备不删除，只解除 gateway_id 绑定（保持现有行为）
5. 失败 → 保留本地记录并返回错误
```

原因：设备/测点/时序历史不能因为删除网关而丢失。

### 8.8 网关列表与对账

本地列表以镜像为准，提供“从 things 刷新”按钮；后台定时对账。

对账逻辑（每个已绑定租户）：

```text
1. 拉取 project 下全部网关
2. 按 things_gateway_id 匹配；没有则按 sn 匹配
3. things 有、本地无：
   - 若租户未绑定 → 跳过并告警
   - 否则创建本地镜像（source=things, synced）
4. 两边都有：
   - 更新 things 拥有的展示字段（name/model/pk）
   - 不覆盖本地 status/last_online_at/config
5. 本地有、things 无：
   - 标记 things_sync_status=missing
   - 不自动删除本地记录，不删除设备/时序数据
   - 停止该网关的新订阅（可配置宽限期），避免已删除网关继续注入数据
6. 同一 SN 出现在不同项目：
   - 标记 conflict，停止自动处理，告警人工确认
```

对账不做的：

- 不把 things 的在线状态写回本地。
- 不把 things 的 secret/license 写回本地。
- 不自动跨租户移动网关。
- 不自动清理本地数据。

### 8.9 存量租户/网关迁移

为了不破坏现有部署：

1. 升级后 `things.enabled=false`，现有行为不变。
2. 平台管理员配置并测试连接。
3. 为每个存量租户选择/创建 things 项目，写入 binding。
4. 执行一次对账：
   - 本地已有网关按 SN 匹配 things 网关，补 `things_gateway_id`；
   - things 中有、本地无的网关自动导入；
   - 本地有、things 无的标记 `local` 或 `missing`，由管理员决定是否在 things 补建。
5. 存量 `source=local` 网关继续可用；不强制迁移。
6. 确认无误后再把新租户/新网关的默认路径切到 things。

---

## 9. 权限与租户隔离

- 只有平台管理员可以：
  - 配置 things 账号；
  - 创建/删除租户；
  - 绑定/解绑项目；
  - 触发全量对账。
- 租户管理员可以：
  - 在本租户绑定的项目内新增/删除 things 网关；
  - 修改本地运行参数。
- 租户不能：
  - 选择或传入 `projectId`；
  - 查看 things 服务账号；
  - 查看其他租户的项目/网关；
  - 修改 secret/license。

实现要求：

- `project_id` 一律由后端从 `tenant_things_binding` 解析。
- `things_gateway_id` 由后端根据本地网关记录解析。
- 平台管理员使用 `x-tenant-id` 代管时，也按目标租户解析绑定，不能用请求体覆盖。
- GORM 租户插件继续保护 `gateway` 和绑定表；对账任务显式使用平台作用域。
- 审计日志记录操作，但不记录 token/password/secret/license。

---

## 10. 错误处理与一致性

### 10.1 错误码建议

| 场景 | 内部错误 | 用户提示 |
| --- | --- | --- |
| things 未配置 | `things_not_configured` | 请先配置 things 平台 |
| 账号密码错误 | `things_auth_failed` | things 账号或密码错误 |
| token 失效且刷新失败 | `things_auth_expired` | things 登录已失效，请检查服务账号 |
| 网络超时/不可达 | `things_unavailable` | things 平台暂时不可用，请稍后重试 |
| 项目不存在 | `things_project_not_found` | things 项目不存在，可能已被删除 |
| 项目重名 | `things_project_duplicate` | things 中已存在同名项目 |
| 网关 SN 重复 | `things_gateway_duplicate` | 该 SN 已在 things 其他项目中使用 |
| 密钥错误 | `things_secret_invalid` | things 拒绝该网关密钥，请核对公司提供的 Secret |
| License 错误 | `things_license_invalid` | things 拒绝该 License，请核对授权信息 |
| 本地绑定缺失 | `things_binding_missing` | 当前租户尚未绑定 things 项目 |

### 10.2 一致性策略

- 没有分布式事务。
- 控制面操作采用“先外部、后本地”或“先本地、后外部 + 补偿”，并在绑定表记录状态。
- 所有写操作都要有幂等确认路径：
  - 项目：按名称查询；
  - 网关：按 `projectId + sn` 查询；
  - 删除：把“不存在”视为成功。
- 对账任务只读 things、修本地，不自动反向修改 things。
- 本地数据面不依赖 things 实时可用。

### 10.3 状态机

租户绑定：

```text
pending ──成功──▶ bound
   │                │
   └──失败──▶ error ─┘（可重试）
bound ──项目在 things 消失──▶ orphaned
bound ──删除中──▶ deleting ──▶ (删除本地 binding)
```

网关同步：

```text
local       平台自有网关，不参与 things 同步
synced      本地镜像与 things 一致
pending     things 已成功，本地待补建
missing     things 中已不存在
conflict    SN/项目归属冲突，需人工处理
error       同步失败，可重试
```

---

## 11. 前端改造

### 11.1 系统配置

新增分组“things 平台”：

- 启用开关；
- 地址；
- 服务账号；
- 密码（只写、掩码）；
- 超时、对账间隔；
- “测试连接”按钮。

### 11.2 租户管理

列表新增：

- things 项目名称 / 项目 ID；
- 绑定状态；
- 最近同步时间；
- 失败原因（悬浮/展开）；
- “重试绑定”“打开 things 项目”操作（可选）。

新增/编辑弹窗：

- 新增时必须填写/确认 `things 项目名称`（4~30）。
- 展示“该租户将自动创建 things 项目”。
- 编辑时项目名称单独操作，不与联系人等本地字段混在一起。

删除弹窗：

- 明确提示：“将同时删除 things 项目，项目下的网关也会被 things 删除”。
- 需要二次确认；如未配置服务账号密码则禁止删除。

### 11.3 网关管理

列表新增：

- 来源标签：`things` / `本地`；
- 同步状态；
- `things 网关 ID`（详情里展示）；
- “从 things 刷新”。

新增弹窗：

- 网关类型：`things 网关` / `平台直连网关`。
- things 网关字段：SN、名称、型号、Secret、LicenseType、License。
- Secret/License 使用密码框；提交后不再回显。
- 不展示平台 MQTT 接入信息。

编辑弹窗：

- things 拥有字段只读；
- 本地字段可编辑；
- 提供“在 things 中重命名”独立操作（可选）。

删除：

- 提示“将删除 things 中的网关；本地设备和历史数据保留”。

原有“同步设备”“重启”“属性写”继续走现有 `/edge` 控制通道，不改为 HTTP。

---

## 12. 约束与风险

1. **单 token 限制**：things 的 `apiLogin` 会让旧 token 失效。IoTServer 多实例部署时，多个进程会互相踢下线。
   - 一期要求 things 集成由单实例执行，或通过部署配置只让一个实例启用。
   - 后续如需多实例，需要共享 token + 分布式锁/leader，或由 things 提供长期 token。
2. **服务账号专用**：不能用该账号登录 things Web 控制台，否则会踢掉 IoTServer 的 token。
3. **删除需要密码**：项目和网关删除都需要服务账号密码；密码必须加密存储，接口只写不回显。
4. **项目名约束**：4~30 字符、同账号唯一，租户名不能直接保证，必须单独确认。
5. **无幂等键**：things 创建接口没有客户端幂等键；超时后用“按名称/SN 查询”确认，不能盲目重试。
6. **列表可能返回敏感字段**：必须白名单解析，避免 secret 进入日志/响应。
7. **things 可用性**：控制面故障时新增/删除失败是预期行为；数据面继续。
8. **项目/网关在 things 被手工改动**：对账能发现新增和缺失，但跨租户移动会标记 conflict，不自动处理。
9. **旧数据兼容**：存量网关可能没有 `things_gateway_id`，需按 SN 匹配；匹配不到时保留 `local`，不删除。

---

## 13. 实施计划

### 阶段 1：基础设施

- 新增 things 配置组与加密存储。
- 实现 `internal/tools/things` HTTP 客户端、TokenManager、错误映射。
- 增加“测试连接”接口。
- 单元测试：envelope、401 重试、超时、敏感字段过滤。

### 阶段 2：租户 ↔ 项目

- 新增 `tenant_things_binding` 表与迁移。
- 租户新增/删除接入 project create/delete。
- 租户列表/详情返回绑定状态。
- 前端租户页面改造。
- 失败补偿与孤儿项目对账。

### 阶段 3：网关 ↔ things

- `gateway` 增加 things 字段。
- 网关新增/删除接入 things API。
- 网关列表区分来源、同步状态。
- 前端新增/编辑/删除改造。
- 对账任务与手动刷新。

### 阶段 4：存量迁移与验收

- 存量租户绑定工具/流程。
- 存量网关 SN 匹配与导入。
- 端到端联调：创建租户 → 创建项目 → 创建网关 → `/edge` 上行 → 删除网关 → 删除项目。
- 故障演练：things 超时、token 失效、项目被手工删除、网关被手工删除。

---

## 14. 测试清单

### 14.1 后端

- [ ] `apiLogin` 成功、失败、超时。
- [ ] token 过期后 401 → 刷新 → 重试一次成功。
- [ ] 并发 20 个请求只触发一次登录。
- [ ] `project/create` 重名错误映射。
- [ ] `project/delete` 密码错误、项目不存在。
- [ ] `gateway/create` 的 `Secret error`、`License error` 原样透传。
- [ ] `gateway/list` 响应中的 secret 不出现在日志/API。
- [ ] 超时后按名称/SN 查询并接管，不重复创建。
- [ ] 本地事务失败触发项目删除补偿。
- [ ] 对账：新增、更新、缺失、冲突四种情况。
- [ ] 租户隔离：A 租户不能读/写 B 租户的网关。
- [ ] 请求体伪造 `projectId` 被忽略。

### 14.2 前端

- [ ] 未配置 things 时，新增网关仍可用本地模式。
- [ ] 配置 things 后，测试连接成功/失败提示正确。
- [ ] 租户新增项目名 4~30 校验。
- [ ] 租户删除提示包含 things 项目。
- [ ] things 网关新增后不回显 secret/license。
- [ ] 同步状态、失败原因展示正确。
- [ ] things 字段只读，本地字段可编辑。

### 14.3 端到端

- [ ] 新建租户 → things 出现项目 → 绑定表正确。
- [ ] 租户新增网关 → things 出现网关 → 本地镜像出现 → `/edge` 数据进入正确租户。
- [ ] 在 things 控制台新增网关 → 对账后本地自动出现。
- [ ] 在 things 控制台删除网关 → 本地标记 missing，数据不再接收，历史保留。
- [ ] 删除网关 → things 删除，本地设备解绑但历史保留。
- [ ] 删除租户 → things 项目删除，本地清理。
- [ ] things 停止服务 → 已有网关数据继续上报，新增/删除操作明确失败。

---

## 15. 待公司确认的接口细节

以下不影响总体设计，但实现前应以公司提供的联调文档为准：

1. 服务账号的角色、项目归属方式，以及是否允许删除项目/网关。
2. `secret` 的提供方式：是否由公司按 SN 生成后交给用户输入；IoTServer 只负责透传。
3. `licenseType` 的取值集合及前端展示方式。
4. things 的 API 根地址、端口、是否强制 HTTPS。
5. 生产环境是否允许多个 IoTServer 实例同时调用 things（当前 token 机制不支持）。

---

## 16. 附录：请求示例

### 16.1 登录

```http
POST /edge-user/user/apiLogin
Content-Type: application/x-www-form-urlencoded

username=service-account&password=******
```

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "token": "******",
    "userId": 1,
    "username": "service-account",
    "roleId": 1
  }
}
```

### 16.2 创建项目

```http
POST /edge-user/project/create
Token: ******
Content-Type: application/x-www-form-urlencoded

name=Tenant%20A&description=IoTServer%20tenant%20tenant_a
```

### 16.3 创建网关

```http
POST /edge-mgmt/gateway/create
Token: ******
Content-Type: application/x-www-form-urlencoded

projectId=101&sn=SN123456&secret=******&name=Gateway%20A&model=IO%20Server&licenseType=trial
```

### 16.4 删除网关

```http
POST /edge-mgmt/gateway/delete
Token: ******
Content-Type: application/x-www-form-urlencoded

projectId=101&gatewayId=2001&pwd=******
```

### 16.5 鉴权失败（HTTP 仍为 200）

```json
{
  "code": 401,
  "message": "UNAUTHORIZED"
}
```

---

## 17. 设计原则回顾

- things 是网关配置和授权的事实来源。
- IoTServer 是租户边界、薄适配层和本地数据面镜像。
- 密钥/授权参数只透传，不解释、不计算、不落库。
- 控制面失败不制造本地半成功；数据面不因控制面故障中断。
- 删除 things 资源不等于删除本地历史数据。
