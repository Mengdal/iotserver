package services

import (
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"iotServer/models"
)

type PositionService struct{}

// Create 创建位置
func (s *PositionService) Create(departmentId int64, name string, parentId *int64) error {
	o := orm.NewOrm()

	// 检查是否已存在根节点
	if parentId == nil {
		rootPosition := &models.Position{Department: &models.Department{Id: departmentId}, ParentPosition: nil}
		if o.QueryTable(new(models.Position)).Filter("department_id", departmentId).Filter("parent_position__isnull", true).One(rootPosition) == nil {
			return fmt.Errorf("only one root node is allowed to be created")
		}
	}

	// 检查同级是否有重名
	var position models.Position
	if parentId == nil {
		if o.QueryTable(new(models.Position)).Filter("department_id", departmentId).Filter("name", name).Filter("parent_position__isnull", true).One(&position) == nil {
			return fmt.Errorf("same level, duplicate names")
		}
	} else {
		if o.QueryTable(new(models.Position)).Filter("department_id", departmentId).Filter("name", name).Filter("parent_position", *parentId).One(&position) == nil {
			return fmt.Errorf("same level, duplicate names")
		}
	}

	// 创建新位置
	newPosition := &models.Position{
		Name:       name,
		Department: &models.Department{Id: departmentId},
	}

	if parentId == nil {
		newPosition.FullName = name
		newPosition.ParentPosition = nil
	} else {
		parent := &models.Position{Id: *parentId}
		if o.Read(parent) != nil {
			return fmt.Errorf("the parent node does not exist")
		}
		newPosition.FullName = parent.FullName + "/" + name
		newPosition.ParentPosition = parent
	}

	_, err := o.Insert(newPosition)
	return err
}

// Delete 删除位置
func (s *PositionService) Delete(id int64) error {
	o := orm.NewOrm()
	position := &models.Position{Id: id}

	if o.Read(position) != nil {
		return fmt.Errorf("position not found")
	}

	_, err := o.Delete(position)
	return err
}

// DeleteAll 递归删除位置及其子节点
func (s *PositionService) DeleteAll(departmentId, id int64) error {
	o := orm.NewOrm()
	position := &models.Position{Id: id}

	if o.Read(position) != nil || position.Department == nil || position.Department.Id != departmentId {
		return fmt.Errorf("position not found or no permission")
	}
	return s.deleteRecursive(position)
}

// deleteRecursive 递归删除位置
func (s *PositionService) deleteRecursive(node *models.Position) error {
	o := orm.NewOrm()

	// 递归删除所有子节点
	children := make([]*models.Position, 0)
	_, err := o.QueryTable(new(models.Position)).Filter("parent_position", node).All(&children)
	if err != nil {
		return err
	}

	for _, child := range children {
		err = s.deleteRecursive(child)
		if err != nil {
			return err
		}
	}

	// 删除当前节点
	_, err = o.Delete(node)
	return err
}

// Edit 编辑位置
func (s *PositionService) Edit(departmentId int64, id int64, name string) error {
	o := orm.NewOrm()

	// 查找要编辑的位置
	position := &models.Position{Id: id}
	if o.QueryTable(new(models.Position)).RelatedSel("ParentPosition").Filter("id", id).One(position) != nil || position.Department == nil || position.Department.Id != departmentId {
		return fmt.Errorf("the position does not exist or no permission")
	}
	var parentId int64
	if position.ParentPosition != nil {
		parentId = position.ParentPosition.Id
	} else {
		parentId = 0
	}
	// 检查同级是否有重名
	var existingPosition models.Position
	if parentId == 0 {
		if o.QueryTable(new(models.Position)).Filter("department_id", departmentId).Filter("name", name).Filter("parent_position__isnull", true).Exclude("id", id).One(&existingPosition) == nil {
			return fmt.Errorf("same level, duplicate names")
		}
	} else {
		if o.QueryTable(new(models.Position)).Filter("department_id", &models.Department{Id: departmentId}).Filter("name", name).Filter("parent_position", &models.Position{Id: parentId}).Exclude("id", id).One(&existingPosition) == nil {
			return fmt.Errorf("same level, duplicate names")
		}
	}

	oldFullName := position.FullName
	position.Name = name

	if parentId == 0 {
		position.FullName = name
	} else {
		position.FullName = position.ParentPosition.FullName + "/" + name
	}

	_, err := o.Update(position)
	if err != nil {
		return err
	}

	// 批量更新子节点的全名
	return s.batchUpdateFullName(departmentId, oldFullName, position.FullName)
}

// batchUpdateFullName 批量更新子节点全名
func (s *PositionService) batchUpdateFullName(departmentId int64, oldFullName string, newFullName string) error {
	o := orm.NewOrm()

	// 查找所有以旧全名为前缀的位置
	positions := make([]*models.Position, 0)
	_, err := o.QueryTable(new(models.Position)).Filter("department_id", &models.Department{Id: departmentId}).Filter("full_name__startswith", oldFullName+"/").All(&positions)
	if err != nil {
		return err
	}

	// 更新这些位置的全名
	for _, pos := range positions {
		pos.FullName = newFullName + pos.FullName[len(oldFullName):]
		_, err = o.Update(pos)
		if err != nil {
			return err
		}
	}

	return nil
}

// TreeOnly 获取位置树
func (s *PositionService) TreeOnly(departmentId int64) (map[string]interface{}, error) {
	o := orm.NewOrm()

	// 查找根节点
	root := &models.Position{}
	err := o.QueryTable(new(models.Position)).Filter("department_id", departmentId).Filter("parent_position__isnull", true).One(root)
	if err != nil {
		// 如果没有找到根节点，返回空树
		return make(map[string]interface{}), nil
	}

	// 构建树结构
	tree := make(map[string]interface{})
	tree["id"] = root.Id
	tree["parentId"] = nil
	tree["name"] = root.Name
	tree["fullName"] = root.FullName
	children, err := s.childrenOnly(root.Id)
	if err != nil {
		return nil, err
	}
	tree["children"] = children

	return tree, nil
}

// childrenOnly 递归获取子节点
func (s *PositionService) childrenOnly(parentId int64) ([]map[string]interface{}, error) {
	o := orm.NewOrm()

	// 查找所有子节点
	var positions []*models.Position
	_, err := o.QueryTable(new(models.Position)).Filter("parent_position", parentId).All(&positions)
	if err != nil {
		return nil, err
	}

	// 构建子节点数组
	array := make([]map[string]interface{}, 0)
	for _, position := range positions {
		object := make(map[string]interface{})
		object["id"] = position.Id
		object["parentId"] = position.ParentPosition.Id
		object["name"] = position.Name
		object["fullName"] = position.FullName

		// 递归获取子节点
		children, err := s.childrenOnly(position.Id)
		if err != nil {
			return nil, err
		}
		object["children"] = children

		array = append(array, object)
	}

	return array, nil
}
