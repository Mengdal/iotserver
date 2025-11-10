package services

import (
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"iotServer/models"
	"iotServer/utils"
)

type GroupService struct{}

// Save 保存标签
func (s *GroupService) Save(id *int64, projectId int64, name string, typeVal *int8, description *string) error {
	o := orm.NewOrm()

	var group *models.Group
	if id == nil {
		group = &models.Group{}
	} else {
		group = &models.Group{Id: *id}
		if o.Read(group) != nil {
			return fmt.Errorf("group not found")
		}
	}

	// 设置属性
	group.Name = name
	if typeVal != nil {
		group.Type = *typeVal
	}
	if description != nil {
		group.Description = *description
	}
	group.Project = &models.Project{Id: projectId}

	// 保存或更新
	if id == nil {
		_, err := o.Insert(group)
		return err
	} else {
		_, err := o.Update(group)
		return err
	}
}

// Delete 删除标签
func (s *GroupService) Delete(id int64) error {
	o := orm.NewOrm()
	group := &models.Group{Id: id}

	_, err := o.Delete(group)
	return err
}

// PageByProjectId 分页查询标签
func (s *GroupService) PageByProjectId(page, size int, projectId *int64) (*utils.PageResult, error) {
	o := orm.NewOrm()
	var groups []*models.Group

	// 构建查询条件
	query := o.QueryTable(new(models.Group))

	if projectId != nil {
		query = query.Filter("project_id", *projectId)
	}

	// 分页查询
	result, err := utils.Paginate(query, page, size, &groups)
	return result, err
}

// UnGroupList 未绑定标签的设备列表
func (s *GroupService) UnGroupList(projectId int64) ([]*models.Device, error) {
	o := orm.NewOrm()
	var devices []*models.Device

	// 查询未绑定标签的设备
	_, err := o.Raw(`
		SELECT d.* FROM device d 
		LEFT JOIN device_group dg ON d.id = dg.device_id 
		WHERE d.project_id = ? AND dg.device_id IS NULL
	`, projectId).QueryRows(&devices)

	return devices, err
}

// BatchGroup 批量绑定设备到标签
func (s *GroupService) BatchGroup(projectId, groupId int64, deviceIds string) error {
	o := orm.NewOrm()

	ids, err := GetResourceIds(deviceIds)
	if err != nil || len(ids) == 0 {
		return err
	}

	// 批量更新设备的group外键
	num, err := o.QueryTable(new(models.Device)).
		Filter("project_id", projectId).
		Filter("id__in", ids).
		Update(orm.Params{
			"group_id": groupId,
		})
	fmt.Println("updated rows:", num)
	return err
}

// UnBatchGroup 批量解绑设备标签
func (s *GroupService) UnBatchGroup(projectId int64, deviceIds string) error {
	o := orm.NewOrm()

	ids, err := GetResourceIds(deviceIds)
	if err != nil || len(ids) == 0 {
		return err
	}

	// 批量更新设备，将group_id设置为null或0来解绑
	_, err = o.QueryTable(new(models.Device)).
		Filter("project_id", projectId).
		Filter("id__in", ids).
		Update(orm.Params{
			"group_id": nil, // 或者使用 0，取决于你的数据库设计
		})

	return err
}
