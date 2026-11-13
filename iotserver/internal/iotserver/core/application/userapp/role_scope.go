package userapp

import (
	"context"
	"fmt"

	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/seed"
	"iotserver/internal/pkg/tenant"
)

// platformDB 角色表同时存全局角色（tenant_id=”）和租户角色。角色相关读写
// 统一使用平台作用域，再在 SQL/应用层显式拼接可见性条件，避免 GORM 租户插件
// 把全局内置角色过滤掉。
func (uapp *userApp) platformDB(ctx context.Context) interfaces.DBClient {
	return uapp.dbClient.WithCtx(tenant.Platform(ctx))
}

// tenantContext 按显式目标租户构造上下文；目标为空表示平台作用域。
// 用于平台管理员在平台视图操作某个租户的用户/部门时，避免沿用平台视图
// 而跳过目标租户的数据校验。
func tenantContext(ctx context.Context, targetTenantId string) context.Context {
	if targetTenantId == "" {
		return tenant.Platform(ctx)
	}
	return tenant.With(ctx, tenant.Context{TenantID: targetTenantId})
}

func isReservedRoleName(name string) bool {
	switch name {
	case "超级管理员", seed.RoleTenantAdminName, seed.RoleTenantUserName:
		return true
	}
	return false
}

// roleVisibleInScope 判断角色是否在当前视图可见。
//   - 平台视图：只看 tenant_id=”（全局内置 + 平台自建）；
//   - 租户视图：本租户自建 + 全局「管理员/普通用户」。
func roleVisibleInScope(role models.Role, targetTenantId string) bool {
	if targetTenantId == "" {
		return role.TenantId == ""
	}
	if role.TenantId == targetTenantId {
		return true
	}
	return role.TenantId == "" && isTenantBuiltin(role.BuiltinKey)
}

// roleAssignableInScope 判断角色能否分配给目标作用域的用户。
//   - 平台账号只能分配平台角色（不含租户内置角色）；
//   - 租户用户只能分配本租户自建角色 + 全局管理员/普通用户。
func roleAssignableInScope(role models.Role, targetTenantId string) bool {
	if targetTenantId == "" {
		return role.TenantId == "" && !isTenantBuiltin(role.BuiltinKey)
	}
	if role.TenantId == targetTenantId {
		return true
	}
	return role.TenantId == "" && isTenantBuiltin(role.BuiltinKey)
}

// loadVisibleRole 按当前视图读取角色；不可见的角色按不存在处理。
func (uapp *userApp) loadVisibleRole(ctx context.Context, id string) (models.Role, error) {
	role, err := uapp.platformDB(ctx).SysRoleById(id)
	if err != nil {
		return role, err
	}
	if !roleVisibleInScope(role, targetTenant(ctx)) {
		return models.Role{}, errort.NewCommonEdgeX(
			errort.DefaultResourcesNotFound, fmt.Sprintf("role %s not found", id), nil)
	}
	return role, nil
}

// rolesForUser 读取用户实际可见的角色：全局内置角色 + 用户所属租户的角色。
// 平台账号只认平台角色；异常残留的跨作用域绑定会被忽略。
func (uapp *userApp) rolesForUser(user models.User) ([]models.Role, error) {
	roleIds, err := uapp.dbClient.WithCtx(tenant.Platform(context.Background())).GetUserRoleIds(user.Id)
	if err != nil {
		return nil, err
	}
	if len(roleIds) == 0 {
		return []models.Role{}, nil
	}
	all, err := uapp.dbClient.WithCtx(tenant.Platform(context.Background())).SysRolesByIds(roleIds)
	if err != nil {
		return nil, err
	}
	result := make([]models.Role, 0, len(all))
	for _, role := range all {
		if user.TenantId == "" {
			// 平台账号：全局角色，且不能拿到租户内置角色。
			if role.TenantId == "" && !isTenantBuiltin(role.BuiltinKey) {
				result = append(result, role)
			}
			continue
		}
		// 租户用户：本租户角色 + 全局管理员/普通用户。
		if role.TenantId == user.TenantId || (role.TenantId == "" && isTenantBuiltin(role.BuiltinKey)) {
			result = append(result, role)
		}
	}
	return result, nil
}

// checkRolesAssignable 校验角色是否可分配给目标作用域。
func (uapp *userApp) checkRolesAssignable(ctx context.Context, targetTenantId string, roleIds []string) error {
	if len(roleIds) == 0 {
		return nil
	}
	roles, err := uapp.platformDB(ctx).SysRolesByIds(roleIds)
	if err != nil {
		return err
	}
	if len(roles) != len(dedupStrings(roleIds)) {
		return errort.NewCommonEdgeX(errort.DefaultResourcesNotFound, "some roles not found", nil)
	}
	for _, role := range roles {
		if !roleAssignableInScope(role, targetTenantId) {
			return errort.NewCommonErr(errort.AppPermissionDenied,
				errort.NewMessageErr("角色 {{.p1}} 不属于当前作用域，不能分配", map[string]interface{}{"p1": role.Id}))
		}
	}
	return nil
}
