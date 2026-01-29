package controllers

import (
	"encoding/json"
	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/validation"
	"iotServer/models"
	"iotServer/models/dtos"
	"iotServer/services"
	"iotServer/utils"
)

type UserController struct {
	BaseController //继承父类
}

// GetAll @Title 获取用户列表
// @Description 主账户获取所有子用户
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   page     query   int     false   "当前页码，默认1"
// @Param   size     query   int     false   "每页数量，默认10"
// @Success 200 {object} controllers.SimpleResult "请求成功,返回通用结果"
// @router /get [post]
func (c *UserController) GetAll() {
	page, _ := c.GetInt("page", 1)  // 当前页，默认 1
	size, _ := c.GetInt("size", 10) // 每页条数，默认 10

	userId := c.Ctx.Input.GetData("user_id")

	o := orm.NewOrm()
	qs := o.QueryTable(new(models.User)).Filter("ParentId", userId)

	var users []models.User
	result, err := utils.Paginate(qs, page, size, &users)
	if err != nil {
		c.Error(400, "查询失败")
	}

	// 转换成 DTO
	var userDtos []dtos.UserDto
	for _, u := range users {
		o.LoadRelated(&u, "Role")
		var roleId int64
		var roleName string
		if u.Role != nil {
			roleId = u.Role.Id
			roleName = u.Role.Name
		}
		var departmentId int64
		if u.Department != nil {
			departmentId = u.Department.Id
		}
		userDtos = append(userDtos, dtos.UserDto{
			Id:           u.Id,
			Email:        u.Email,
			Username:     u.Username,
			ParentId:     u.ParentId,
			WebToken:     u.WebToken,
			CreateTime:   u.CreateTime,
			RoleId:       roleId, // 如果只要 roleId
			RoleName:     roleName,
			DepartmentId: departmentId,
		})
	}
	result.List = userDtos
	c.Success(result) // 返回分页结果，而不是只返回数据数组
}

// Put @Title UpdateUser
// @Description 更新用户信息
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body  body  dtos.UpdateUserRequest  true  "请求体"
// @Success 200 {object} controllers.SimpleResult "请求成功,返回通用结果"
// @router /updateUser [post]
func (c *UserController) Put() {
	//必须使用结构体swagger才能识别
	var req dtos.UpdateUserRequest

	//获取request的pwd
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil || req.Id == 0 {
		c.Error(400, "参数错误")
	}

	//查询用户
	newOrm := orm.NewOrm()
	user := models.User{Id: req.Id}
	if err := newOrm.Read(&user); err != nil {
		c.Error(400, "用户不存在")
	}

	//更新密码
	if req.Password != "" {
		user.Password = req.Password
	}
	if req.RoleId != 0 {
		user.Role.Id = req.RoleId
	}
	if req.Username != "" {
		user.Username = req.Username
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.DepartmentId != 0 {
		user.Department.Id = req.DepartmentId
	}

	if _, err := newOrm.Update(&user); err != nil {
		c.Error(400, "更新失败")
	} else {
		c.SuccessMsg()
	}
}

// Update @Title UpdateUserPassword
// @Description 更新用户密码，主账号token可以修改子账户密码
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   id       query   int     false    "User ID"
// @Param   password query   string  true    "New password"
// @Success 200 {object} controllers.SimpleResult "请求成功,返回通用结果"
// @Failure 400 "请求参数错误或更新失败"
// @router /updatePassword [post]
func (c *UserController) Update() {
	//当前token用户
	currentUser := c.Ctx.Input.GetData("user_id")
	currentUserId := currentUser.(int64)
	//需要更改密码的用户
	id, err := c.GetInt64("id")
	password := c.GetString("password")
	if err != nil || password == "" {
		c.Error(400, "参数错误")
	}

	//查询用户
	newOrm := orm.NewOrm()
	user := models.User{Id: id}
	if err := newOrm.Read(&user); err != nil {
		c.Error(400, "用户不存在或无权限")
	}

	//更新密码
	if currentUserId == id || (user.ParentId != nil && *user.ParentId == currentUserId) {
		user.Password = password
	} else {
		c.Error(400, "用户无权限")
	}

	if _, err := newOrm.Update(&user); err != nil {
		c.Error(400, "更新失败")
	} else {
		c.SuccessMsg()
	}
}

// Login @Title 用户登录
// @Description 用户登录
// @Param   username query   string  true    "username"
// @Param   password query   string  true    "password"
// @Success 200 {object} controllers.SimpleResult "请求成功,返回通用结果"
// @Failure 400 参数解析失败
// @router /login [post]
func (c *UserController) Login() {
	username := c.GetString("username")
	password := c.GetString("password")
	// 参数校验
	if username == "" || password == "" {
		c.Error(400, "用户名和密码不能为空")
	}
	// 查询用户
	o := orm.NewOrm()
	user := models.User{Username: username}
	err := o.Read(&user, "Username")
	if err != nil || user.Password != password {
		c.Error(400, "用户不存在或密码错误")
	}
	token, _ := utils.GenerateToken(user.Id, user.Department.Id)
	user.WebToken = token
	_, err = o.Update(&user)
	if err != nil {
		c.Error(400, "更新用户token失败")
	}
	role := models.Role{Id: user.Role.Id}
	if err = o.Read(&role); err != nil {
		c.Error(400, "获取角色出错")
	}

	projects, err := models.GetUserProjects(user)
	if err != nil {
		c.Error(400, err.Error())
	}
	enable := true
	tenant := &models.Tenant{Id: user.Department.Id}
	err = o.Read(tenant)
	if err != nil {
		enable = false
	}
	var menus []dtos.MenuDTO
	err = json.Unmarshal([]byte(role.Permission), &menus)
	if err != nil {
		c.Error(400, "菜单数据解析失败")
	}
	result := map[string]interface{}{
		"token":        token,
		"menu":         menus,
		"roleId":       role.Id,
		"roleName":     role.Name,
		"userId":       user.Id,
		"username":     username,
		"departmentId": user.Department.Id,
		"tenantId":     tenant.Id,
		"project":      projects,
		"enable":       enable,
	}
	c.Success(result)
}

// Register @Title 用户注册
// @Description 基于当前用户创建，token不填时创建父账户，有token则基于当前用户创建
// @Param body body dtos.RegisterDto true "用户信息（email选填）"
// @Success 200 {object} controllers.SimpleResult "请求成功,返回通用结果"
// @Failure 400 参数解析失败
// @router /register [post]
func (c *UserController) Register() {
	var req dtos.RegisterDto

	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "参数解析失败")
	}

	var this models.User
	this.Role = &models.Role{Id: req.RoleId}
	this.Username = req.UserName
	this.Password = req.Password
	//orm验证器
	valid := validation.Validation{}
	ok, _ := valid.Valid(&this)
	if !ok {
		for _, e := range valid.Errors {
			c.Error(400, e.Key+": "+e.Message)
		}
	}

	newOrm := orm.NewOrm()
	exist := newOrm.QueryTable(new(models.User)).Filter("Username", this.Username).Exist()
	if exist {
		c.Error(400, "用户名已存在")
	}
	this.WebToken = ""

	//说明有token基于当前用户注册
	if req.UserId != 0 {
		var currentUser models.User
		currentUser.Id = req.UserId
		if err := newOrm.Read(&currentUser); err != nil || currentUser.ParentId != nil || req.DepartmentId == 0 {
			c.Error(400, "用户不存在或无权限")
		}

		this.ParentId = &currentUser.Id
		this.Department = &models.Department{Id: req.DepartmentId}
	} else {
		//无token则创建父账户
		this.ParentId = nil
		this.Role = &models.Role{Id: 0}
	}

	if _, err := newOrm.Insert(&this); err != nil {
		c.Error(400, "注册失败")
	}
	if this.ParentId == nil {
		departmentService := services.DepartmentService{}
		id, err := departmentService.CreateDepartment(this.Username+"_TENANT", this.Username, "", this.Username, "1", "", 0, 0, "TENANT",
			"", "", "", "", 0, "")
		if err != nil {
			c.Error(400, err.Error())
		}
		this.Department = &models.Department{Id: id}
		newOrm.Update(&this, "Department")
		tenantService := services.TenantService{}
		_, err = tenantService.Create(this.Id, id, this.Username+"_TENANT", "", "", "")
		if err != nil {
			c.Error(400, err.Error())
		}
		// 初始化租户菜单权限
		menuService := services.MenuService{}
		err = menuService.InitTenantMenus(id)
		if err != nil {
			c.Error(400, "初始化菜单失败")
		}
	}
	c.SuccessMsg()
}

// Delete @Title 删除用户
// @Description 删除指定用户
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   id       query   int     true  "用户ID"
// @Success 200 {object} controllers.SimpleResult "删除成功，返回1"
// @Failure 400 "系统角色不能删除 或 删除失败"
// @router /del [post]
func (c *UserController) Delete() {
	id, _ := c.GetInt("id")
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)

	o := orm.NewOrm()
	user := &models.User{Id: int64(id)}
	if err := o.QueryTable(user).Filter("Id", id).RelatedSel("Department").One(user); err != nil || user.Department == nil || user.Department.TenantId != userId {
		c.Error(400, "user not found or no permission")
	}

	if _, err := o.Delete(&user); err != nil {
		c.Error(400, "删除失败")
	}

	c.SuccessMsg()
}
