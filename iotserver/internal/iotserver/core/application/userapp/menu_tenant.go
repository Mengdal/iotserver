package userapp

import (
	"context"
	"strings"

	"iotserver/internal/models"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/tenant"
)

// 菜单多租户改造的作用域工具。
//
// 设计要点（docs/multi-tenant-menu-plan.md §2）：
//   - sys_menu 是「全局默认（tenant_id=''）+ 租户自建（tenant_id=X）」表，**不**登记进
//     models.tenantScopedModels（登记后全局行会被一起过滤掉），作用域由这里显式拼接；
//   - 写入归属：目标租户 = 平台作用域 ? '' : 当前租户；写 '' 必须平台管理员；
//   - 更新/删除前校验行归属 == 目标租户；
//   - 租户自建菜单不允许设置 perms（防自助提权）。

// targetTenant 菜单操作的目标租户：平台作用域（平台视图/后台任务）→ ”（平台菜单）；
// 否则取当前租户（租户用户或平台管理员代管租户）。
func targetTenant(ctx context.Context) string {
	tc := tenant.FromContext(ctx)
	if tc.Platform {
		return ""
	}
	return tc.TenantID
}

// isPlatformAdminCtx 当前请求是否是平台管理员（ROBX 运营账号）
func isPlatformAdminCtx(ctx context.Context) bool {
	return tenant.FromContext(ctx).IsPlatformAdmin
}

// requireMenuWriteScope 写菜单本体前的权限收口（P0-1）：
// 目标为 ” （平台原始菜单）时必须是平台管理员，租户用户一律拒绝（不做静默忽略）。
func requireMenuWriteScope(ctx context.Context, target string) error {
	if target == "" && !isPlatformAdminCtx(ctx) {
		return errort.NewCommonErr(errort.AppPermissionDenied,
			errort.NewMessageErr("平台原始菜单只有平台管理员可以维护", nil))
	}
	return nil
}

// requireMenuRowOwnership 更新/删除前校验该行归属必须等于目标租户。
func requireMenuRowOwnership(ctx context.Context, m models.Menu, target string) error {
	if m.TenantId != target {
		return errort.NewCommonErr(errort.AppPermissionDenied,
			errort.NewMessageErr("菜单 {{.p1}} 属于 {{.p2}} 作用域，当前作用域无权修改", map[string]interface{}{"p1": m.Id, "p2": scopeName(m.TenantId)}))
	}
	return requireMenuWriteScope(ctx, target)
}

// requireNoTenantPerms 租户自建菜单不允许设置 perms：否则等于租户自助提权，
// 可以造出任意权限码给自己。平台菜单（target=”）不受影响。
func requireNoTenantPerms(target, perms string) error {
	if target == "" {
		return nil
	}
	if strings.TrimSpace(perms) != "" {
		return errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("租户自建菜单不允许设置权限码(perms)，平台菜单的权限由平台管理员维护", nil))
	}
	return nil
}

// menuInScope 行是否落在当前菜单作用域内：平台菜单对所有视图可见，
// 租户自建菜单只对本租户（含平台管理员代管该租户）可见。
func menuInScope(m models.Menu, target string) bool {
	return m.TenantId == "" || m.TenantId == target
}

func scopeName(tenantId string) string {
	if tenantId == "" {
		return "平台"
	}
	return "租户 " + tenantId
}

// menuPrefs 读取某租户的展示层覆盖（平台作用域读取，prefs 不是插件隔离表）
func (uapp *userApp) menuPrefs(tenantId string) ([]models.TenantMenuPref, error) {
	if tenantId == "" {
		return nil, nil
	}
	return uapp.dbClient.WithCtx(tenant.Platform(context.Background())).SysMenuPrefsByTenant(tenantId)
}

// applyMenuPrefs 套用租户展示层覆盖：隐藏（含其子树）+ 自定义名称/图标/排序。
//
// 只影响**展示**：权限集合始终按角色授权计算，隐藏不等于收回权限 ——
// 否则租户把自己租户的「菜单管理」隐藏一下就再也进不去了。
// path / perms / component 永不被覆盖（平台修路径、修权限 bug 时租户立刻跟随）。
func (uapp *userApp) applyMenuPrefs(tenantId string, menus []models.Menu) ([]models.Menu, error) {
	if tenantId == "" || len(menus) == 0 {
		return menus, nil
	}
	prefs, err := uapp.menuPrefs(tenantId)
	if err != nil {
		return nil, err
	}
	if len(prefs) == 0 {
		return menus, nil
	}

	prefById := make(map[string]models.TenantMenuPref, len(prefs))
	for _, p := range prefs {
		prefById[p.MenuId] = p
	}
	// 隐藏项连同其子树一起屏蔽（子树关系取自本次可见集合；祖先已由 completeMenuAncestors 补齐）
	childIds := make(map[string][]string, len(menus))
	for _, m := range menus {
		childIds[m.ParentId] = append(childIds[m.ParentId], m.Id)
	}
	blocked := make(map[string]struct{})
	queue := make([]string, 0, len(prefs))
	for _, p := range prefs {
		if p.Hidden == 1 {
			queue = append(queue, p.MenuId)
		}
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if _, ok := blocked[id]; ok {
			continue
		}
		blocked[id] = struct{}{}
		queue = append(queue, childIds[id]...)
	}

	out := make([]models.Menu, 0, len(menus))
	for _, m := range menus {
		if _, hidden := blocked[m.Id]; hidden {
			continue
		}
		if p, ok := prefById[m.Id]; ok {
			if p.CustomName != "" {
				m.Name = p.CustomName
			}
			if p.CustomIcon != "" {
				m.Icon = p.CustomIcon
			}
			if p.Sort != 0 {
				m.Sort = p.Sort
			}
		}
		out = append(out, m)
	}
	return out, nil
}
