package userapp

import (
	"sort"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
)

func nowMilli() int64 {
	return time.Now().UnixMilli()
}

/******************************* 模型 -> DTO *******************************/

func (uapp *userApp) toSysUserResponse(u models.User) dtos.SysUserResponse {
	res := dtos.SysUserResponse{
		Id:        u.Id,
		Username:  u.Username,
		Nickname:  u.Nickname,
		Email:     u.Email,
		Phone:     u.Phone,
		DeptId:    u.DeptId,
		Status:    u.Status,
		Remark:    u.Remark,
		RoleIds:   []string{},
		RoleNames: []string{},
		LastLogin: u.LastLogin,
		Created:   u.Created,
		Modified:  u.Modified,
	}
	if u.DeptId != "" {
		if dept, err := uapp.dbClient.SysDeptById(u.DeptId); err == nil {
			res.DeptName = dept.Name
		}
	}
	if roleIds, err := uapp.dbClient.GetUserRoleIds(u.Id); err == nil {
		res.RoleIds = roleIds
		if len(roleIds) > 0 {
			if roles, rErr := uapp.dbClient.SysRolesByIds(roleIds); rErr == nil {
				for _, r := range roles {
					res.RoleNames = append(res.RoleNames, r.Name)
					if r.RoleKey == "admin" {
						res.Admin = true
					}
				}
			}
		}
	}
	return res
}

func toRoleResponse(r models.Role, menuIds []string) dtos.SysRoleResponse {
	if menuIds == nil {
		menuIds = []string{}
	}
	return dtos.SysRoleResponse{
		Id:        r.Id,
		Name:      r.Name,
		RoleKey:   r.RoleKey,
		Sort:      r.Sort,
		DataScope: r.DataScope,
		Status:    r.Status,
		Remark:    r.Remark,
		MenuIds:   menuIds,
		Created:   r.Created,
		Modified:  r.Modified,
	}
}

func toMenuResponse(m models.Menu) dtos.SysMenuResponse {
	return dtos.SysMenuResponse{
		Id:        m.Id,
		ParentId:  m.ParentId,
		Name:      m.Name,
		Path:      m.Path,
		Component: m.Component,
		Icon:      m.Icon,
		MenuType:  m.MenuType,
		Perms:     m.Perms,
		Sort:      m.Sort,
		Visible:   m.Visible,
		Status:    m.Status,
		IsFrame:   m.IsFrame,
		Remark:    m.Remark,
		Created:   m.Created,
		Modified:  m.Modified,
		Children:  []dtos.SysMenuResponse{},
	}
}

func toDeptResponse(d models.Dept) dtos.SysDeptResponse {
	return dtos.SysDeptResponse{
		Id:        d.Id,
		ParentId:  d.ParentId,
		Ancestors: d.Ancestors,
		Name:      d.Name,
		Sort:      d.Sort,
		Leader:    d.Leader,
		Phone:     d.Phone,
		Email:     d.Email,
		Status:    d.Status,
		Created:   d.Created,
		Modified:  d.Modified,
		Children:  []dtos.SysDeptResponse{},
	}
}

func toLoginLogResponse(l models.LoginLog) dtos.SysLoginLogResponse {
	return dtos.SysLoginLogResponse{
		Id:        l.Id,
		Username:  l.Username,
		Ip:        l.Ip,
		Browser:   l.Browser,
		Os:        l.Os,
		Status:    l.Status,
		Message:   l.Message,
		LoginTime: l.LoginTime,
	}
}

func toConfigResponse(c models.SystemConfig) dtos.SysConfigResponse {
	return dtos.SysConfigResponse{
		Id:          c.Id,
		ConfigKey:   c.ConfigKey,
		ConfigValue: c.ConfigValue,
		Name:        c.Name,
		GroupName:   c.GroupName,
		Type:        c.Type,
		Sort:        c.Sort,
		Remark:      c.Remark,
		Created:     c.Created,
		Modified:    c.Modified,
	}
}

/******************************* 树构建 *******************************/

// buildMenuTree 构建菜单树（仅保留目录与菜单，按钮不下发到树中）
func buildMenuTree(menus []models.Menu) []dtos.SysMenuResponse {
	list := make([]dtos.SysMenuResponse, 0, len(menus))
	for _, m := range menus {
		if m.MenuType == models.MenuTypeButton {
			continue
		}
		list = append(list, toMenuResponse(m))
	}
	return buildMenuChildren(list, "0")
}

func buildMenuChildren(list []dtos.SysMenuResponse, parentId string) []dtos.SysMenuResponse {
	result := make([]dtos.SysMenuResponse, 0)
	for _, item := range list {
		if item.ParentId != parentId {
			continue
		}
		children := buildMenuChildren(list, item.Id)
		item.Children = children
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Sort < result[j].Sort
	})
	return result
}

// buildDeptTree 构建部门树
func buildDeptTree(list []dtos.SysDeptResponse) []dtos.SysDeptResponse {
	return buildDeptChildren(list, "0")
}

func buildDeptChildren(list []dtos.SysDeptResponse, parentId string) []dtos.SysDeptResponse {
	result := make([]dtos.SysDeptResponse, 0)
	for _, item := range list {
		if item.ParentId != parentId {
			continue
		}
		item.Children = buildDeptChildren(list, item.Id)
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Sort < result[j].Sort
	})
	return result
}

// collectPerms 收集权限标识
func collectPerms(menus []models.Menu) []string {
	perms := make([]string, 0)
	seen := make(map[string]struct{})
	for _, m := range menus {
		if m.Perms == "" {
			continue
		}
		for _, p := range splitPerms(m.Perms) {
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			perms = append(perms, p)
		}
	}
	return perms
}

func splitPerms(perms string) []string {
	result := make([]string, 0)
	start := 0
	for i := 0; i <= len(perms); i++ {
		if i == len(perms) || perms[i] == ',' || perms[i] == ';' {
			item := trimSpace(perms[start:i])
			if item != "" {
				result = append(result, item)
			}
			start = i + 1
		}
	}
	return result
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
