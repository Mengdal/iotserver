package controllers

import (
	"database/sql"
	"github.com/beego/beego/v2/client/orm"
	"iotServer/models"
	"iotServer/utils"
)

type RoleController struct {
	BaseController
}

// Template @Title 获取权限模板
// @Description 获取权限模板
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Success 200 {object} controllers.SimpleResult "请求成功"
// @router /template [post]
func (c *RoleController) Template() {
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)
	menuCtrl := &MenuController{}
	tree := menuCtrl.treeEnable(tenantId)
	c.Success(tree)
}

// GetRoleList @Title 分页获取角色列表
// @Description 分页获取角色列表
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   page     query   int     false   "当前页码，默认1"
// @Param   size     query   int     false   "每页数量，默认10"
// @Success 200 {object} controllers.SimpleResult "请求成功"
// @router /getRoleList [post]
func (c *RoleController) GetRoleList() {
	page, _ := c.GetInt("page", 1)
	size, _ := c.GetInt("size", 10)

	userId := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)

	o := orm.NewOrm()
	var roles []models.Role
	qs := o.QueryTable(new(models.Role)).SetCond(orm.NewCondition().And("department_id", tenantId).Or("department_id__isnull", true))
	pageResult, err := utils.Paginate(qs, page, size, &roles)
	if err != nil {
		c.Error(400, "查询失败: "+err.Error())
	}

	c.Success(pageResult)
}

// GetRole @Title 获取角色详情
// @Description 根据ID获取角色信息
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   id       query   int     true    "角色ID"
// @Success 200 {object} controllers.SimpleResult "请求成功"
// @Failure 400 "参数错误或角色不存在"
// @router /getRole [post]
func (c *RoleController) GetRole() {
	id, _ := c.GetInt("id")

	userId := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)

	o := orm.NewOrm()
	role := models.Role{Id: int64(id)}
	if err := o.Read(&role); err != nil || (role.DepartmentId.Int64 != 0 && role.DepartmentId.Int64 != tenantId) {
		c.Error(400, "role not found or no permission")
	}

	c.Success(role)
}

// Create @Title 创建角色
// @Description 创建新角色
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   name        query   string  true  "角色名称"
// @Param   description query   string  false "角色描述"
// @Param   permission  query   string  false "权限字符串"
// @Success 200 {object} controllers.SimpleResult "返回创建成功的角色ID"
// @Failure 400 "参数错误"
// @router /create [post]
func (c *RoleController) Create() {
	name := c.GetString("name")
	description := c.GetString("description")
	permission := c.GetString("permission")

	if name == "" {
		c.Error(400, "角色名称不能为空")
	}

	o := orm.NewOrm()

	userId := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)

	role := models.Role{
		Name:        name,
		Description: description,
		Permission:  permission,
		DepartmentId: sql.NullInt64{
			Int64: tenantId,
			Valid: true,
		},
	}

	id, err := o.Insert(&role)
	if err != nil {
		c.Error(400, "创建失败")
	}

	c.Success(id)
}

// Edit @Title 编辑角色
// @Description 修改已有角色的信息
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   id          query   int     true  "角色ID"
// @Param   name        query   string  true  "角色名称"
// @Param   description query   string  false "角色描述"
// @Param   permission  query   string  false "权限字符串"
// @Success 200 {object} controllers.SimpleResult "操作成功"
// @Failure 400 "参数错误或角色不存在"
// @router /edit [post]
func (c *RoleController) Edit() {
	id, _ := c.GetInt("id")
	name := c.GetString("name")
	description := c.GetString("description")
	permission := c.GetString("permission")

	if name == "" {
		c.Error(400, "角色名称不能为空")
	}

	o := orm.NewOrm()

	role := models.Role{Id: int64(id)}
	if err := o.Read(&role); err != nil || !role.DepartmentId.Valid {
		c.Error(400, "无操作权限")
	}

	role.Name = name
	if description != "" {
		role.Description = description
	}
	if permission != "" {
		role.Permission = permission
	}
	if _, err := o.Update(&role); err != nil {
		c.Error(400, "更新失败")
	}

	c.SuccessMsg()
}

// Delete @Title 删除角色
// @Description 删除指定角色
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   id       query   int     true  "角色ID"
// @Success 200 {object} controllers.SimpleResult "删除成功，返回1"
// @Failure 400 "系统角色不能删除 或 删除失败"
// @router /del [post]
func (c *RoleController) Delete() {
	id, _ := c.GetInt("id")
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)

	o := orm.NewOrm()
	role := &models.Role{Id: int64(id)}

	// 先检查是否有用户引用这个角色
	cnt, err := o.QueryTable(new(models.User)).Filter("Role__Id", id).Count()
	if err != nil {
		c.Error(500, "数据库查询失败: "+err.Error())
	}
	if cnt > 0 {
		c.Error(400, "该角色已被用户使用，无法删除")
	}
	if err = o.Read(role); err != nil || !role.DepartmentId.Valid || role.DepartmentId.Int64 != tenantId {
		c.Error(400, "无操作权限")
	}
	if _, err := o.Delete(role); err != nil {
		c.Error(400, "删除失败")
	}

	c.SuccessMsg()
}
