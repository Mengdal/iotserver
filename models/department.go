package models

import (
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"time"
)

// Department 部门信息
type Department struct {
	Id        int64       `orm:"pk;auto" json:"id"`
	Name      string      `orm:"size(128)" json:"name"`
	LevelType string      `orm:"size(50)" json:"level_type"`
	Parent    *Department `orm:"rel(fk);column(parent_id);null;on_delete(set_null)" json:"parent"`
	TenantId  int64       `orm:"index" json:"tenant_id"` // 最高级租户ID
	Leader    string      `orm:"size(64);null" json:"leader"`
	Phone     string      `orm:"size(32);null" json:"phone"`
	Email     string      `orm:"size(128);null" json:"email"`
	Status    string      `orm:"type(text);null" json:"status"`
	Sort      int         `orm:"default(0)" json:"sort"`
	Remark    string      `orm:"size(255);null" json:"remark"`
	Created   int64       `orm:"null" json:"created"`
	Modified  int64       `orm:"null" json:"modified"`

	Factory     string `orm:"size(255);null" json:"factory"`
	Active      int64  `orm:"null" json:"active"`
	Description string `orm:"size(255);null" json:"desc"`
	GIS         string `orm:"size(255);null;column(gis)" json:"gis" `
	Address     string `orm:"size(255);null" json:"address" `
	AreaId      int64  `orm:"index" json:"area_id"`
	Capacity    string `orm:"null" json:"capacity" `
}

func init() {
	orm.RegisterModel(new(Department))
}

// BeforeUpdate 更新前钩子
func (a *Department) BeforeUpdate() error {
	a.Modified = time.Now().Unix()
	return nil
}

// BeforeInsert 插入前钩子
func (a *Department) BeforeInsert() error {
	now := time.Now().Unix()
	if a.Created == 0 {
		a.Created = now
	}
	a.Modified = now
	return nil
}

// DepartmentLevelType 部门层级类型枚举
const (
	TenantLevel     = "TENANT"     // 集团层级
	DepartmentLevel = "DEPARTMENT" // 部门层级
	ProjectLevel    = "PROJECT"    // 项目层级（最低层级）
)

// IsValidLevelType 验证层级类型是否有效
func IsValidLevelType(levelType string) bool {
	validTypes := []string{DepartmentLevel, ProjectLevel}
	for _, validType := range validTypes {
		if levelType == validType {
			return true
		}
	}
	return false
}

// GetUserIsRoot 获取用户L1机构
func GetUserIsRoot(userId int64) (int64, error) {
	o := orm.NewOrm()

	user := User{Id: userId}
	if err := o.Read(&user); err != nil {
		return 0, fmt.Errorf("获取用户信息失败: %v", err)
	}

	// 获取用户所在部门的租户ID
	userDepartment := &Department{Id: user.Department.Id}
	if err := o.Read(userDepartment); err != nil {
		return 0, fmt.Errorf("部门信息查询失败: %v", err)
	}
	if user.Department.Id != userDepartment.TenantId {
		return userDepartment.TenantId, fmt.Errorf("error")
	}

	return userDepartment.TenantId, nil
}

// GetUserTenantId 获取用户L1机构
func GetUserTenantId(userId int64) (int64, error) {
	o := orm.NewOrm()

	user := User{Id: userId}
	if err := o.Read(&user); err != nil {
		return 0, fmt.Errorf("获取用户信息失败: %v", err)
	}

	// 获取用户所在部门的租户ID
	userDepartment := &Department{Id: user.Department.Id}
	if err := o.Read(userDepartment); err != nil {
		return 0, fmt.Errorf("部门信息查询失败: %v", err)
	}

	return userDepartment.TenantId, nil
}

// GetUserProjects 获取用户所属的所有项目（非树形结构）
func GetUserProjects(user User) ([]*Department, error) {
	o := orm.NewOrm()
	var projects []*Department

	// 如果用户没有分配部门，返回空列表
	if user.Department == nil {
		return projects, nil
	}

	// 获取用户所在部门的租户ID
	userDepartment := &Department{Id: user.Department.Id}
	if err := o.Read(userDepartment); err != nil {
		return nil, fmt.Errorf("部门信息查询失败: %v", err)
	}

	// 根据租户ID查询该租户下的所有项目（PROJECT层级）
	_, err := o.QueryTable(new(Department)).
		Filter("tenant_id", userDepartment.TenantId).
		Filter("level_type", ProjectLevel).
		All(&projects)

	if err != nil {
		return nil, fmt.Errorf("查询项目列表失败: %v", err)
	}

	return projects, nil
}

// GetUserProjectIds 获取用户关联的项目ID列表
func GetUserProjectIds(userId int64, projectId int64) ([]int64, error) {
	var projectIds []int64

	// 如果指定了具体项目ID，直接使用
	if projectId != 0 {
		projectIds = append(projectIds, projectId)
		return projectIds, nil
	}

	// 否则获取用户所属的所有项目
	o := orm.NewOrm()
	user := User{Id: userId}
	if err := o.Read(&user); err != nil {
		return nil, fmt.Errorf("获取用户信息失败: %v", err)
	}

	projects, err := GetUserProjects(user)
	if err != nil {
		return nil, err
	}

	for _, project := range projects {
		projectIds = append(projectIds, project.Id)
	}

	return projectIds, nil
}
