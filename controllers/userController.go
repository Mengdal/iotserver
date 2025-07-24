package controllers

import (
	"encoding/json"
	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/validation"
	"iotServer/models"
	"iotServer/models/dtos"
	"iotServer/utils"
)

type UserController struct {
	BaseController //继承父类
}

// GetAll @Title 获取用户列表
// @Description 主账户获取所有子用户
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Success 200 {object} controllers.SimpleResult "请求成功,返回通用结果"
// @router /get [post]
func (c *UserController) GetAll() {
	userId := c.Ctx.Input.GetData("user_id")
	o := orm.NewOrm()
	var users []dtos.UserDto
	_, err := o.QueryTable(new(models.User)).Filter("ParentId", userId).All(&users)
	if err != nil {
		c.Error(400, "查询失败")
	} else {
		c.Success(users)
	}
}

// Put @Title UpdateUser
// @Description 更新用户信息
// @Param   body  body  dtos.UpdateUserRequest  true  "请求体"
// @Success 200 {object} controllers.SimpleResult "请求成功,返回通用结果"
// @router /updateUser [post]
func (c *UserController) Put() {
	//必须使用结构体swagger才能识别
	var req dtos.UpdateUserRequest

	//获取request的pwd
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil || req.Id == 0 || req.Password == "" {
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
	if req.RoleId != nil {
		user.RoleId = req.RoleId
	}
	if req.Username != "" {
		user.Username = req.Username
	}
	if req.Email != "" {
		user.Email = req.Email
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
	token, _ := utils.GenerateToken(user.Id)
	user.WebToken = token
	_, err = o.Update(&user)
	if err != nil {
		c.Error(400, "更新用户token失败")
	}
	role := models.Role{Id: *user.RoleId}
	if err = o.Read(&role, "Id"); err != nil {
		c.Error(400, "获取角色出错")
	}

	menuController := &MenuController{}
	menuController.Ctx = c.Ctx
	menuTree := menuController.tree()
	result := map[string]interface{}{
		"token":      token,
		"menu":       menuTree,
		"roleId":     user.RoleId,
		"permission": role.Permission,
	}
	c.Success(result)
}

// Register @Title 用户注册
// @Description 基于当前用户创建，token不填时创建父账户，有token则基于当前用户创建
// @Param Authorization query string false "token检验"
// @Param body body dtos.RegisterDto true "用户信息（email选填）"
// @Success 200 {object} controllers.SimpleResult "请求成功,返回通用结果"
// @Failure 400 参数解析失败
// @router /register [post]
func (c *UserController) Register() {
	token := c.GetString("Authorization")

	var this models.User
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &this); err != nil {
		c.Error(400, "参数解析失败")
	}

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
	if token != "" {
		//token取出当前用户
		claims, ok := utils.ParseToken(token)
		if !ok {
			c.Error(403, "Forbidden: invalid token")
		}
		currentUserId, ok := claims["user_id"].(float64)
		if !ok {
			c.Error(400, "身份验证失败")
		}
		var currentUser models.User
		currentUser.Id = int64(currentUserId)
		if err := newOrm.Read(&currentUser); err != nil || currentUser.ParentId != nil {
			c.Error(400, "用户不存在或无权限")
		}

		this.ParentId = &currentUser.Id
		var role int64 = 2
		this.RoleId = &role
	} else {
		//无token则创建父账户
		this.ParentId = nil
		var role int64 = 1
		this.RoleId = &role
	}

	if _, err := newOrm.Insert(&this); err != nil {
		c.Error(400, "注册失败")
	}
	c.SuccessMsg()
}
