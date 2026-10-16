# 多租户安全排查：全局表 vs 租户可写接口

> 排查动机：修复"菜单/系统配置"这类 **表是全局一份、但接口对租户开放** 的问题。
> 方法：把库里全部 **41 张表** 按"有没有 `tenant_id`"分类（22 有 / 19 无），再对 19 张无租户列的表逐张核对"是否有写接口、谁能写、影响范围"。
> 结论：**确认 2 处真实问题（已修）+ 1 处死代码（已清理）+ 1 处由用户决定暂不处理**，并顺带修掉 1 个菜单可见性泄漏。

---

## 1. 19 张无 `tenant_id` 表的逐张结论

| 表 | 结论 | 依据 |
|---|---|---|
| `properties` / `events` / `actions` | ✅ 安全（间接隔离） | 物模型能力表，始终通过 `product_id` 访问；写接口 `thingmodel` 按 productId 操作 → 由 product 的租户归属保证 |
| `sys_role_menu` / `sys_user_role` | ✅ 安全（间接隔离） | 关联表，操作入口是 role / user（两者都已隔离） |
| `tenant` | ✅ 安全 | 租户表本身：路由在 `PlatformOnly()` 组 + 菜单 `platform_only=1` |
| `category_template` / `thing_model_template` | ✅ 正常只读 | 正常路径只读（建产品时读模板）；写入口只有"废弃 sync" |
| `driver_classify` | ✅ 无写接口 | — |
| `language_sdk` | ✅ 无正常写接口 | 只有"废弃 sync" |
| `sys_menu` | ✅ **已修** | 加 `tenant_id` + 写收口 + 平台菜单只读/租户定制（见 `multi-tenant-menu-plan.md`） |
| **`sys_config`** | ⚠️ **确认问题 → 已修** | 见 §2 |
| `unit` / `doc` / `quick_navigation` | ⚠️ **只有"废弃 sync"能写 → 已修** | 见 §3 |
| `advance_config` | 🟡 死代码 → 已清理 | 见 §4 |
| `device_library` / `device_service` / `docker_config` | ⏭️ 用户决定暂不处理 | 驱动资源按"平台级共享"设计，租户可写（新建的驱动镜像配置其他租户可见/可用） |

## 2. `sys_config`（系统配置）—— 已修

**问题**：表全局唯一、页面不是 `platform_only`、4 个写接口只校验权限码，而租户管理员天然拥有 `system:config:*`。
实测：租户A管理员可创建全局配置，平台管理员立刻可见；且 `security.passwordMinLength`、`security.sessionTimeout` 被 `userapp/user.go` 真实读取 → **单个租户可改全平台安全策略**。

**修复**：
- 「系统配置」菜单加 `PlatformOnly: 1`；
- 菜单 upsert 的 updates 补 `platform_only` 列（否则存量库不会刷新该标记）；
- 4 个写接口（`POST/PUT/DELETE system-setup`、`POST system-setup/save`）移入 `PlatformOnly()` 组；
- **读接口原地保留**（`GET system-setup`、`GET system-setup/:id`），避免内部/页面读配置被拦。

## 3. 6 条"废弃"写路由 —— 已摘除

```
POST thingmodel/unit-sync             → 全局 unit
POST thingmodel/docs-sync             → 全局 doc
POST thingmodel/quicknavigation-sync  → 全局 quick_navigation
POST category-template/sync           → 全局 category_template
POST thingmodel-template/sync         → 全局 thing_model_template
POST language-sdk-sync                → 全局 language_sdk
```
**问题**：注释标着"废弃"却仍注册，且**没有 `Perm(...)`** → 任何登录用户（含租户用户）都能触发全表同步/覆盖全局字典表。前端确认未使用。
**修复**：路由注册摘除（handler 代码保留）；调用现在的返回是 `1108 url path is invalid`。

## 4. `advance_config` 死代码 —— 已清理

有模型、仓储与接口声明（`GetAdvanceConfig` / `UpdateAdvanceConfig`），但**零调用方、零路由**。
已删除：模型文件、两个仓储文件（该文件仅含 AdvanceConfig）、接口方法、AutoMigrate 条目、常量 `DefaultAdvanceConfigID`。

## 5. 顺带修复：`/user/getinfo` 的菜单泄漏

`MenuRoutesByUsername`（UI 实际读取菜单的来源）**从未过滤 `platform_only` 菜单** —— 只有登录响应 `buildLoginUser` 与 `PermissionsByUsername` 过滤了。
实测租户 token 的 `/user/getinfo` 能拿到 `/platform/tenant`（租户管理）与 `/admin/sysSet`（系统配置）→ 侧边栏会显示这两个平台专属入口。
**修复**：补 `filterPlatformOnlyMenus`（放在 `completeMenuAncestors` 之后，避免祖先补全又加回来），与另两处判定一致。

## 6. 核对为安全的部分

- **OpenAPI**（`/v1.0/openapi/*` 的 product / device / thingModel 写接口）分组里带 `jwt.JWTAuth(false) + middleware.TenantContext()` → AppKey 对应用户决定租户，**不会产生孤儿数据**；
- `solution` 表已有 `tenant_id`，且"内置只读 + 副本覆盖"已实现（见 `multi-tenant-menu-plan.md` §7）。

## 7. 遗留与后续

1. 租户管理员的**权限码集合**里仍有 `system:config:*`（按钮权限行未标 `platform_only`），实际拦截靠路由 `PlatformOnly`，属纵深防御；若要权限集合也干净，需给按钮行也加平台标记；
2. **驱动资源**（`device_library` / `device_service` / `docker_config`）按用户决定暂不处理 —— 现状是"平台共享、租户可写"，如果将来要按租户隔离，需加 `tenant_id` 或做覆盖层；
3. 本次所有修复都需要**重启后端**才生效（路由注册 + 菜单 seed + 菜单过滤逻辑均在启动路径）。
