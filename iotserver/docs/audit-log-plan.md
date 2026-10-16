# 审查日志（登录 + 操作）改造方案

> 状态：**已实施**（后端 + 前端）。README/权限常量/菜单均已同步；重启后端即生效。目标是把「登入日志」升级成**一个「审查日志」模块，同时包含登录记录与操作记录**。

## 1. 目标

同一个页面既能看"谁登录了、成功还是失败"，也能看"谁在什么时候对哪个模块做了什么、结果如何、耗时多少"。

- 采用**单表 + 类型字段**（`type = login | operation`）：一条时间线、一套筛选、一套保留策略、一套权限；
- 现状：`sys_login_log` 只记登录，**没有任何操作日志基础设施**。

成功标准：① 登录（成功/失败）与写操作都能查到且字段完整；② 两类记录都按租户隔离；③ 密码等敏感字段不落库；④ 超期自动清理；⑤ `go build` + 全量测试通过。

## 2. 数据模型

`sys_login_log` **改名为 `sys_audit_log`**（不建新表、不搬数据），并补列：

| 列 | 说明 |
|---|---|
| `tenant_id` | 已有（租户隔离） |
| `type` | `login` / `operation` —— 新增，索引 |
| `username` | 登录账号 / 操作人（已有） |
| `module` | 认证 / 产品管理 / 设备管理 / 网关管理 / 方案中心 / 租户管理 / 用户管理 / 菜单管理 / 系统配置 … 新增，索引 |
| `action` | 登录 / 新增 / 修改 / 删除 / 导入 / 发布 / 启动 / 停止 / 下发 / 应用 / 清空 … 新增 |
| `target` | 操作对象（资源 id 或名称，尽量短）新增 |
| `method` / `path` | HTTP 方法与路径（登录为空）新增 |
| `status` | 1 成功 / 0 失败（已有，索引） |
| `error_code` / `message` | 业务错误码与提示（`message` 已有，`error_code` 新增） |
| `ip` / `browser` / `os` | 已有 |
| `params` | 操作参数：**脱敏 + 截断 2KB** 新增 |
| `duration_ms` | 耗时 新增 |
| `created` | 事件时间（新增 `Timestamps`；登录记录同时写 `login_time` 兼容）索引 |

## 3. 写入链路

**登录事件**：沿用现有 5 个分支（`application/userapp/user.go`），写 `type=login`、`module=认证`、`action=登录`，失败带原因。

**操作事件**：新增中间件 `internal/pkg/middleware/auditlog.go`，挂在 `route/gateway.go` 的 `v1Auth`（含 `platform` 子组）与 `route/openapi.go` 的 `v1` 组。

- 只记**非 GET**；
- 白名单排除：`auth/login`、`auth/init-password`、`ekuiper/*`、token 刷新、swagger、静态资源、健康检查；
- 响应捕获：包装 `gin.ResponseWriter`，提取 `success`/`errorCode`/`errorMsg`（截断），结合 HTTP 状态判断成败；
- 模块映射：按路径首段查表（`product→产品管理`、`device→设备管理`、`gateway→网关管理`、`sys-user→用户管理`、`sys-role→角色管理`、`menu→菜单管理`、`dept→部门管理`、`system-setup→系统配置`、`tenant→租户管理`、`solution→方案中心`、`alert-rule→告警中心`、`scene→场景联动`、`rule-engine→规则引擎`、`dataresource→资源管理`、`viz→数据看板`…），未命中回落原始路径；
- 动作映射：`POST→新增`、`PUT→修改`、`DELETE→删除`，特征路径覆盖：`*-release→发布`、`*-unrelease→取消发布`、`*/start→启动`、`*/stop→停止`、`*/import→导入`、`*/apply→应用`、`*/clear→清空`、`*/command→下发命令`、`*/config→下发配置`、`*/sync→同步`；
- 对象：优先路径参数（`productId`/`deviceId`…），其次请求体顶层 `name`/`id`（浅解析），否则用路径；
- 脱敏：字段名含 `password`/`oldPassword`/`newPassword`/`secret`/`token`/`appKey`/`privateKey` → `***`；body > 2KB 截断并追加 `…(截断)`；
- 异步落库：`chan`（缓冲 4096）+ 后台协程批量写（满 100 条或每 1 秒 flush），队列满丢弃并 WARN，**不阻塞业务**；
- 租户归属：用请求捕获到的 `tenant.Context` 写库（插件自动落 `tenant_id`）。

## 4. 接口 / 权限 / 菜单

- `GET /api/v1/sys-audit-log`（筛选：username / type / module / status / 时间范围 / 分页）
- `DELETE /api/v1/sys-audit-log`（按 id 批量删）、`DELETE /api/v1/sys-audit-log/clear`（按当前租户范围清空）
- 权限码：`system:auditlog:list`、`system:auditlog:remove`；旧 `system:loginlog:*` 保留一版做兼容别名
- 菜单：`MenuAdminLoginLog` → 名称**「审查日志」**，路径 `/admin/sysLoginLog` → **`/admin/sysAuditLog`**，仍在「系统管理」下 Sort 5

## 5. 前端

`pages/admin/SysLoginLog` → `pages/admin/SysAuditLog`：

- 筛选：类型（全部/登录/操作）、操作人、模块、状态、时间范围；
- 列：类型 Tag（登录=蓝 / 操作=绿）、操作人、模块、动作、对象、结果（成功/失败 + 错误码）、耗时、IP、时间；不适用列显示 `-`；
- 操作：`详情`（弹层展示完整字段 + 脱敏 params + 失败原因）、`删除`（Popconfirm）、工具栏 `清空`；
- 路由与菜单路径同步；空态/分页沿用现有风格。

## 6. 保留与清理

- 复用配置 `data.retentionDays`（`bootstrap/database/database.go`，默认 30，**0=永久**）；
- 新增轻量后台任务（`time.Ticker` 每 6 小时），在 `initialize` 里注册；
- 删除 `created < now - retentionDays 天` 的记录，**平台作用域跨租户清理**；`0` 时跳过；打日志记录删除条数。

## 7. 迁移（双后端，幂等）

1. 有 `sys_login_log` 且无 `sys_audit_log` → `ALTER TABLE sys_login_log RENAME TO sys_audit_log`；
2. 缺列则逐列 `AddColumn`（`type/module/action/target/method/path/error_code/params/duration_ms` + `Timestamps`）；
3. 回填 `type='login'`、`module='认证'`；
4. 建索引 `(tenant_id, created)`、`(tenant_id, type, created)`、`(username)`、`(status)`；
5. `models.LoginLog` → `models.AuditLog`，同步仓储/应用/控制器/接口/测试引用。

## 8. 验收测试

| # | 用例 | 期望 |
|---|---|---|
| 1 | 登录成功/密码错/停用/到期 | 各写一条 `type=login`，`status`/`message` 正确 |
| 2 | 平台管理员新增产品 | 写 `type=operation`、`module=产品管理`、`action=新增`、`status=1` |
| 3 | 失败的写操作 | `status=0`，`error_code`/`message` 与接口返回一致 |
| 4 | 新建用户（body 含 password） | `params` 中密码已打码，库里无明文 |
| 5 | 租户隔离 | 租户B 查不到租户A 的记录；平台视图可见全部 |
| 6 | 保留清理 | `retentionDays=1` 时过期记录被删、未过期保留；`0` 时一条不删 |
| 7 | 白名单 | `POST /auth/login`、`/ekuiper/*` 不产生 `type=operation` |
| 8 | 回归 | 后端 build + 全量测试；前端 tsc/eslint/build |

## 9. 风险与取舍

1. 表体积增长：靠保留天数 + 批量写缓解；导出/归档后续再做；
2. 中间件开销：异步写 + 响应体限 2KB；
3. 表改名：仓库内无外部依赖（已核对）；
4. 兼容：旧接口/旧权限码保留一版别名，下版本删；
5. 不做字段级 diff 快照（只记请求参数与结果）。
