package controllers

import (
	"fmt"
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
	menuCtrl := &MenuController{}
	tree := menuCtrl.treeEnable()
	c.Success(tree)
}

// GetRoleList @Title 分页获取角色列表
// @Description 分页获取角色列表，仅有主账户含角色
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   page     query   int     false   "当前页码，默认1"
// @Param   size     query   int     false   "每页数量，默认10"
// @Success 200 {object} controllers.SimpleResult "请求成功"
// @router /getRoleList [post]
func (c *RoleController) GetRoleList() {
	page, _ := c.GetInt("page", 1)
	size, _ := c.GetInt("size", 10)

	//userId := c.Ctx.Input.GetData("user_id")

	o := orm.NewOrm()
	var roles []models.Role
	qs := o.QueryTable(new(models.Role))
	pageResult, err := utils.Paginate(qs, page, size, &roles)
	if err != nil {
		c.Error(400, "查询失败: "+err.Error())
		return
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

	o := orm.NewOrm()
	role := models.Role{Id: int64(id)}
	if err := o.Read(&role); err != nil {
		c.Error(400, "角色不存在")
		return
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

	var currentUser models.User
	userId, ok := c.Ctx.Input.GetData("user_id").(int64)
	currentUser.Id = userId
	err := o.Read(&currentUser)
	if !ok || err != nil || currentUser.ParentId != nil {
		c.Error(400, "用户无权限")
	}

	role := models.Role{
		Name:        name,
		Description: description,
		Permission:  permission,
		UserId:      userId,
	}

	id, err := o.Insert(&role)
	if err != nil {
		c.Error(400, "创建失败")
		return
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
	if err := o.Read(&role); err != nil {
		c.Error(400, "角色不存在")
	}

	var currentUser models.User
	userId, ok := c.Ctx.Input.GetData("user_id").(int64)
	currentUser.Id = userId
	err := o.Read(&currentUser)
	if !ok || err != nil || currentUser.ParentId != nil || currentUser.Id != role.UserId {
		c.Error(400, "用户无权限")
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

	o := orm.NewOrm()
	role := models.Role{Id: int64(id)}

	if !CheckModelOwnership(o, "role", int64(id), userId, "") {
		c.Error(400, "用户无权限或已删除")
	}
	// 先检查是否有用户引用这个角色
	cnt, err := o.QueryTable(new(models.User)).Filter("Role__Id", id).Count()
	if err != nil {
		c.Error(500, "数据库查询失败: "+err.Error())
	}
	if cnt > 0 {
		c.Error(400, "该角色已被用户使用，无法删除")
	}
	if _, err := o.Delete(&role); err != nil {
		c.Error(400, "删除失败")
	}

	c.SuccessMsg()
}

// CheckModelOwnership 检查某个资源是否属于当前用户
// orm 表名 资源ID 当前用户ID 资源中用户ID字段
func CheckModelOwnership(o orm.Ormer, model string, resourceID int64, userID int64, userIDField string) bool {
	if userIDField == "" {
		userIDField = "user_id"
	}

	// 获取当前用户信息
	var currentUser models.User
	currentUser.Id = userID
	if err := o.Read(&currentUser); err != nil {
		fmt.Println("CheckModelOwnership: 用户不存在 - ID:", userID)
		return false
	}

	// 获取资源的归属用户ID
	var resourceUserID int64

	switch model {
	case "product":
		var p models.Product
		if err := o.QueryTable("product").Filter("id", resourceID).One(&p, "user_id"); err != nil {
			fmt.Println("CheckModelOwnership: 查询 product 失败:", err)
			return false
		}
		resourceUserID = p.User.Id

	case "role":
		var r models.Role
		if err := o.QueryTable("role").Filter("id", resourceID).One(&r); err != nil {
			fmt.Println("CheckModelOwnership: 查询 role 失败:", err)
			return false
		}
		resourceUserID = r.UserId

	case "user":
		var u models.User
		if err := o.QueryTable("user").Filter("id", resourceID).One(&u); err != nil {
			fmt.Println("CheckModelOwnership: 查询 user 失败:", err)
			return false
		}
		resourceUserID = u.Id
	default:
		fmt.Println("CheckModelOwnership: 不支持的模型:", model)
		return false
	}

	// 情况 1：资源归属就是当前用户
	if resourceUserID == userID {
		return true
	}

	// 情况 2：子用户不能访问他人资源
	if currentUser.ParentId != nil {
		return false
	}

	// 情况 3：主用户可访问其子用户资源
	var owner models.User
	owner.Id = resourceUserID
	if err := o.Read(&owner); err != nil {
		fmt.Println("CheckModelOwnership: 资源所属用户不存在 - ID:", resourceUserID)
		return false
	}

	if owner.ParentId != nil && *owner.ParentId == userID {
		return true
	}

	fmt.Println("CheckModelOwnership: 权限不足 - user:", userID, ", resource user:", resourceUserID)
	return false
}
