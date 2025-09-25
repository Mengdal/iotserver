package controllers

import (
	"github.com/beego/beego/v2/client/orm"
	"iotServer/models"
	"iotServer/models/dtos"
	"iotServer/utils"
	"sort"
)

type MenuController struct {
	BaseController
}

// List @Title 菜单列表（全部）
// @Description 获取完整菜单树
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Success 200 {object} controllers.SimpleResult "返回菜单树"
// @router /list [post]
func (c *MenuController) List() {
	tree := c.tree()
	c.Success(tree)
}

// Create @Title 创建菜单
// @Description 创建新菜单（名称不能重复）
// @Param   Authorization    header   string  true        "Bearer YourToken"
// @Param   path             query    string  false       "路径"
// @Param   component        query    string  false       "组件"
// @Param   name             query    string  true        "名称"
// @Param   redirect         query    string  false       "重定向"
// @Param   status           query    int    true         "是否启用"
// @Param   type             query    int     true        "类型"
// @Param   parentId         query    int     false       "父级菜单ID"
// @Param   permissionList   query    string  false       "按钮权限列表"
// @Param   meta             query    string  true        "元数据"
// @Success 200 {object} controllers.SimpleResult "操作成功"
// @Failure 400 "参数错误或创建失败"
// @router /create [post]
func (c *MenuController) Create() {
	path := c.GetString("path")
	component := c.GetString("component")
	name := c.GetString("name")
	redirect := c.GetString("redirect")
	status, _ := c.GetInt("status")
	typ, _ := c.GetInt("type")
	parentId, _ := c.GetInt("parentId")
	var parentIdPtr *int64
	if parentId > 0 {
		temp := int64(parentId)
		parentIdPtr = &temp
	}
	permissionList := c.GetString("permissionList")
	meta := c.GetString("meta")

	if name == "" || meta == "" {
		c.Error(400, "必填字段缺失")
	}
	if existing, _ := c.findByName(name); existing != nil {
		c.Error(400, "菜单名称重复，请更换后重试！")
	}

	o := orm.NewOrm()
	menu := models.Menu{
		Name:           name,
		Path:           path,
		Component:      component,
		Redirect:       redirect,
		Status:         status,
		Type:           typ,
		ParentId:       parentIdPtr,
		PermissionList: permissionList,
		Meta:           meta,
	}

	if _, err := o.Insert(&menu); err != nil {
		c.Error(400, "创建失败")
	}
	c.SuccessMsg()
}

// Edit @Title 编辑菜单
// @Description 修改已有菜单信息（避免重名）
// @Param   Authorization    header   string  true        "Bearer YourToken"
// @Param   id               query    int     true        "菜单ID"
// @Param   path             query    string  false       "路径"
// @Param   component        query    string  true        "组件"
// @Param   name             query    string  true        "名称"
// @Param   redirect         query    string  false       "重定向"
// @Param   status           query    bool    true        "是否启用"
// @Param   type             query    int     true        "类型"
// @Param   parentId         query    int     false       "父级菜单ID"
// @Param   permissionList   query    string  false       "按钮权限列表"
// @Param   meta             query    string  true        "元数据"
// @Success 200 {object} controllers.SimpleResult "操作成功"
// @Failure 400 "参数错误或更新失败"
// @router /edit [post]
func (c *MenuController) Edit() {
	id, _ := c.GetInt64("id")
	path := c.GetString("path")
	component := c.GetString("component")
	name := c.GetString("name")
	redirect := c.GetString("redirect")
	status, _ := c.GetBool("status")
	typ, _ := c.GetInt("type")
	parentId, _ := c.GetInt("parentId")
	permissionList := c.GetString("permissionList")
	meta := c.GetString("meta")
	priority, _ := c.GetInt("priority")

	if id <= 0 || component == "" || name == "" || meta == "" {
		c.Error(400, "参数错误")
	}

	var parentIdPtr *int64
	if parentId > 0 {
		temp := int64(parentId)
		parentIdPtr = &temp
	}

	o := orm.NewOrm()
	menu := models.Menu{Id: id}
	if err := o.Read(&menu); err != nil {
		c.Error(400, "菜单不存在")
	}
	existing, _ := c.findByName(name)
	if existing != nil && existing.Id != id {
		c.Error(400, "菜单名称重复，请更换后重试！")
	}

	if name != menu.Name {
		menu.Name = name
	}
	if path != menu.Path {
		menu.Path = path
	}
	if component != menu.Component {
		menu.Component = component
	}
	if meta != menu.Meta {
		menu.Meta = meta
	}
	if redirect != menu.Redirect {
		menu.Redirect = redirect
	}
	if permissionList != menu.PermissionList {
		menu.PermissionList = permissionList
	}
	if menu.Status != convertStatus(status) {
		menu.Status = convertStatus(status)
	}
	if typ != menu.Type {
		menu.Type = typ
	}
	if parentIdPtr != menu.ParentId {
		menu.ParentId = parentIdPtr
	}
	menu.Priority = priority

	if _, err := o.Update(&menu); err != nil {
		c.Error(400, "更新失败")
	}
	c.SuccessMsg()
}

// Delete @Title 删除菜单
// @Description 根据菜单ID删除菜单
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   id             query   int     true  "菜单ID"
// @Success 200 {object} controllers.SimpleResult "删除成功"
// @Failure 400 "参数错误或删除失败"
// @router /del [post]
func (c *MenuController) Delete() {
	id, _ := c.GetInt64("id")
	if id <= 0 {
		c.Error(400, "菜单ID无效")
	}

	o := orm.NewOrm()
	if _, err := o.Delete(&models.Menu{Id: id}); err != nil {
		c.Error(400, "删除失败: "+err.Error())
	}

	c.SuccessMsg()
}

func convertStatus(b bool) int {
	if b {
		return 1
	}
	return 0
}

// =====================================service=========================================

func (c *MenuController) getObjects(menus []models.Menu) []dtos.MenuDTO {
	var DTO []dtos.MenuDTO
	for _, menu := range menus {

		dto := dtos.MenuDTO{
			Path:           menu.Path,
			Component:      menu.Component,
			Name:           menu.Name,
			Redirect:       menu.Redirect,
			Status:         menu.Status,
			Id:             menu.Id,
			Type:           menu.Type,
			ParentId:       menu.ParentId,
			PermissionList: utils.ParseJsonArray(menu.PermissionList),
			Meta:           utils.ParseJson(menu.Meta),
			Children:       c.children(&menu.Id),
			Priority:       menu.Priority,
		}
		DTO = append(DTO, dto)
	}
	// 按 Priority 升序排序（小的排前面）
	sort.Slice(DTO, func(i, j int) bool {
		return DTO[i].Priority < DTO[j].Priority
	})
	return DTO
}

func (c *MenuController) children(parentId *int64) []dtos.MenuDTO {
	array, err := c.listByParentId(parentId)
	if err != nil {
		c.Error(400, "查询失败")
	}
	return c.getObjects(array)
}
func (c *MenuController) tree() []dtos.MenuDTO {
	array, err := c.listByRoot()
	if err != nil {
		c.Error(400, "查询失败")
	}
	return c.getObjects(array)
}
func (c *MenuController) treeEnable() []dtos.MenuDTO {
	array, err := c.listByRootEnable()
	if err != nil {
		c.Error(400, "查询失败")
	}
	return c.getObjects(array)
}

func (c *MenuController) listByRoot() ([]models.Menu, error) {
	o := orm.NewOrm()
	qs := o.QueryTable(new(models.Menu)).Filter("ParentId__isnull", true)
	var menus []models.Menu
	_, err := qs.All(&menus)
	return menus, err
}
func (c *MenuController) listByRootEnable() ([]models.Menu, error) {
	o := orm.NewOrm()
	qs := o.QueryTable(new(models.Menu)).Filter("ParentId__isnull", true).Filter("Status", 1)
	var menus []models.Menu
	_, err := qs.All(&menus)
	return menus, err
}

func (c *MenuController) listByParentId(parentId *int64) ([]models.Menu, error) {
	o := orm.NewOrm()
	qs := o.QueryTable(new(models.Menu))

	if parentId == nil {
		qs = qs.Filter("ParentId__isnull", true)
	} else {
		qs = qs.Filter("ParentId", *parentId)
	}

	var menus []models.Menu
	_, err := qs.All(&menus)
	return menus, err
}

func (c *MenuController) findByName(name string) (*models.Menu, error) {
	o := orm.NewOrm()
	var menu models.Menu
	err := o.QueryTable(new(models.Menu)).Filter("Name", name).One(&menu)
	if err != nil {
		return nil, err
	}
	return &menu, nil
}
