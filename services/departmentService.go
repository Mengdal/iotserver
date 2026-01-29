package services

import (
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"iotServer/models"
	"iotServer/utils"
	"strings"
)

// DepartmentService 部门管理服务
type DepartmentService struct{}

// CreateDepartment 创建部门
func (s *DepartmentService) CreateDepartment(name, leader, phone, email, status, remark string, parentID int64, sort int, levelType string, factory string, capacity string, address string, gis string, active int64, description string) (int64, error) {
	o := orm.NewOrm()

	department := &models.Department{
		Name:        name,
		Leader:      leader,
		Phone:       phone,
		Email:       email,
		Status:      status,
		Remark:      remark,
		Sort:        sort,
		LevelType:   levelType,
		Factory:     factory,
		Capacity:    capacity,
		Address:     address,
		GIS:         gis,
		Active:      active,
		Description: description,
	}

	// 验证父部门是否存在
	if parentID > 0 {
		parent := &models.Department{Id: parentID}
		if err := o.Read(parent); err != nil {
			return 0, fmt.Errorf("上级部门不存在: %v", err)
		}
		// 【约束检查】: 禁止在最低层级 (如 PROJECT) 下创建子机构
		if parent.LevelType == "PROJECT" || department.LevelType == "TENANT" {
			return 0, fmt.Errorf("无法在此部门下创建该层级")
		}

		// 【TenantId 继承】: L2/L3 子机构直接继承父机构的 TenantId
		department.Parent = parent // 设置 ParentId
		department.TenantId = parent.TenantId
	} else {
		if department.LevelType != "TENANT" {
			return 0, fmt.Errorf("无法在此部门下创建该层级")
		}
	}

	// 插入前处理
	department.BeforeInsert()

	// 插入部门并获取ID
	id, err := o.Insert(department)
	if err != nil {
		return 0, fmt.Errorf("创建部门失败: %v", err)
	}

	// 【L1 TenantId 最终设置】: 如果是顶级机构 (L1)，需要在插入后更新 TenantId
	if parentID == 0 {
		department.TenantId = id // TenantId 设置为自身的ID
		// 执行更新操作
		if _, err := o.Update(department, "TenantId"); err != nil {
			// 考虑到 TenantId 的关键性，这里可能需要记录日志或执行回滚操作
			return 0, fmt.Errorf("更新上级部门失败: %v", err)
		}
	}

	return id, nil // 返回创建成功的部门ID
}

// UpdateDepartment 更新部门信息
func (s *DepartmentService) UpdateDepartment(departmentId int64, id int64, name, leader, phone, email, status, remark string, parentID int64, sort int, deviceIds []int64, factory string, capacity string, address string, gis string, active int64, description string) error {
	o := orm.NewOrm()

	department := &models.Department{Id: id}
	if err := o.Read(department); err != nil || department.TenantId != departmentId {
		return fmt.Errorf("部门不存在或无操作权限")
	}

	// 验证父部门是否存在且不能设置自己为父部门
	if parentID > 0 {
		if parentID == id {
			return fmt.Errorf("不能设置自己为上级部门")
		}

		// 检查父部门是否存在
		parent := &models.Department{Id: parentID}
		if err := o.Read(parent); err != nil {
			return fmt.Errorf("上级部门不存在: %v", err)
		}

		// 检查是否会产生循环引用
		if err := s.checkCircularReference(id, parentID); err != nil {
			return err
		}

		department.Parent = parent
	} else {
		department.Parent = nil
	}

	department.Name = name
	department.Leader = leader
	department.Phone = phone
	department.Email = email
	department.Status = status
	department.Remark = remark
	department.Sort = sort
	department.Factory = factory
	department.Capacity = capacity
	department.Address = address
	department.GIS = gis
	department.Active = active
	department.Description = description

	// 更新前处理
	department.BeforeUpdate()

	_, err := o.Update(department)
	if err != nil {
		return fmt.Errorf("更新部门失败: %v", err)
	}
	if department.LevelType == "PROJECT" {
		if err := s.ReDistributionFromDepartment(departmentId, deviceIds); err != nil {
			return err
		}
	}

	return nil
}

// checkCircularReference 检查部门循环引用
func (s *DepartmentService) checkCircularReference(currentID, parentID int64) error {
	o := orm.NewOrm()

	// 获取父部门的所有祖先部门
	var checkParent func(int64) error
	checkParent = func(deptID int64) error {
		var dept models.Department
		if err := o.QueryTable(new(models.Department)).
			Filter("id", deptID).One(&dept); err != nil {
			return err
		}

		if dept.Parent != nil {
			if dept.Parent.Id == currentID {
				return fmt.Errorf("设置上级部门会产生循环引用")
			}
			return checkParent(dept.Parent.Id)
		}

		return nil
	}

	return checkParent(parentID)
}

// DeleteDepartment 删除部门
func (s *DepartmentService) DeleteDepartment(departmentId int64, id int64) error {
	o := orm.NewOrm()

	department := &models.Department{Id: id}
	if err := o.Read(department); err != nil || department.TenantId != departmentId {
		return fmt.Errorf("部门不存在或无操作权限")
	}

	// 检查是否有子部门
	childCount, err := o.QueryTable(new(models.Department)).
		Filter("parent_id", id).Count()
	if err != nil {
		return fmt.Errorf("检查下级部门失败: %v", err)
	}
	if childCount > 0 {
		return fmt.Errorf("该部有下级部门，请先删除或转移下级部门")
	}

	// 检查是否有设备关联
	deviceCount, err := o.QueryTable(new(models.Device)).
		Filter("department_id", id).Count()
	if err != nil {
		return fmt.Errorf("检查设备关联失败: %v", err)
	}
	if deviceCount > 0 {
		return fmt.Errorf("该部门下有配置设备，请先移除设备")
	}

	// 检查是否有用户关联
	userCount, err := o.QueryTable(new(models.User)).
		Filter("department_id", id).Count()
	if err != nil {
		return fmt.Errorf("检查用户关联失败: %v", err)
	}
	if userCount > 0 {
		return fmt.Errorf("该部门下有分配用户，请先转移用户")
	}

	// 删除部门
	_, err = o.Delete(department)
	if err != nil {
		return fmt.Errorf("删除部门失败: %v", err)
	}

	return nil
}

// GetDepartmentTree 获取部门树形结构
func (s *DepartmentService) GetDepartmentTree(tenantId int64, keyword string) ([]map[string]interface{}, error) {
	o := orm.NewOrm()

	// 获取所有部门
	var departments []models.Department
	_, err := o.QueryTable(new(models.Department)).Filter("tenant_id", tenantId).
		RelatedSel("Parent").
		OrderBy("sort", "id").
		All(&departments)
	if err != nil {
		return nil, fmt.Errorf("查询部门失败: %v", err)
	}

	// 构建部门树
	return s.buildDepartmentTree(departments, keyword), nil
}

// buildDepartmentTree 构建部门树形结构
func (s *DepartmentService) buildDepartmentTree(departments []models.Department, keyword string) []map[string]interface{} {
	// 创建部门映射
	deptMap := make(map[int64]*models.Department)
	for i := range departments {
		deptMap[departments[i].Id] = &departments[i]
	}

	// 构建树形结构
	var roots []map[string]interface{}

	for _, dept := range departments {
		if dept.Parent == nil || dept.Parent.Id == 0 {
			rootNode := s.buildDepartmentNode(dept, deptMap, keyword)
			// 如果有关键词搜索，只保留包含匹配结果的树
			if keyword == "" || s.containsMatchingNode(rootNode, keyword) {
				roots = append(roots, rootNode)
			}
		}
	}

	return roots
}

// buildDepartmentNode 构建部门节点
func (s *DepartmentService) buildDepartmentNode(dept models.Department, deptMap map[int64]*models.Department, keyword string) map[string]interface{} {
	node := make(map[string]interface{})
	node["id"] = dept.Id
	node["name"] = dept.Name
	node["leader"] = dept.Leader
	node["phone"] = dept.Phone
	node["email"] = dept.Email
	node["status"] = dept.Status
	node["sort"] = dept.Sort
	node["remark"] = dept.Remark
	node["created"] = dept.Created
	node["modified"] = dept.Modified
	node["level_type"] = dept.LevelType
	node["parentId"] = int64(0) // 默认值
	if dept.Parent != nil {
		node["parentId"] = dept.Parent.Id
	}
	// 获取部门直接关联的设备数量
	deviceCount, err := s.getDepartmentDirectDeviceCount(dept.Id)
	if err != nil {
		node["deviceCount"] = 0
	} else {
		node["deviceCount"] = deviceCount
	}
	// 查找子部门
	var children []map[string]interface{}
	for _, childDept := range deptMap {
		if childDept.Parent != nil && childDept.Parent.Id == dept.Id {
			childNode := s.buildDepartmentNode(*childDept, deptMap, keyword)
			// 如果有关键词搜索，只保留包含匹配结果的节点
			if keyword == "" || s.containsMatchingNode(childNode, keyword) {
				children = append(children, childNode)
			}
		}
	}

	node["children"] = children

	return node
}

// getDepartmentDirectDeviceCount 获取部门直接关联的设备数量（不包括子部门）
func (s *DepartmentService) getDepartmentDirectDeviceCount(departmentID int64) (int64, error) {
	o := orm.NewOrm()

	// 统计直接关联该部门的设备数量
	count, err := o.QueryTable(new(models.Device)).
		Filter("department_id", departmentID).
		Count()
	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetDirectDepartmentDevices 获取部门直接关联的设备（不包括子部门）
func (s *DepartmentService) GetDirectDepartmentDevices(tenantId, departmentID int64, page, size int) (*utils.PageResult, error) {
	o := orm.NewOrm()

	// 查询部门下的所有设备
	query := o.QueryTable(new(models.Device)).RelatedSel("Product", "Department").Filter("tenant_id", tenantId).Filter("department_id", departmentID)

	return utils.Paginate(query, page, size, &[]models.Device{})
}

// containsMatchingNode 检查节点或其子节点是否包含匹配关键词
func (s *DepartmentService) containsMatchingNode(node map[string]interface{}, keyword string) bool {
	// 检查当前节点是否匹配
	if name, ok := node["name"].(string); ok && containsIgnoreCase(name, keyword) {
		return true
	}

	// 检查子节点是否匹配
	if children, ok := node["children"].([]map[string]interface{}); ok {
		for _, child := range children {
			if s.containsMatchingNode(child, keyword) {
				return true
			}
		}
	}

	return false
}

// containsIgnoreCase 检查字符串是否包含子字符串（忽略大小写）
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// AssignDevicesToDepartment 分配设备到部门
func (s *DepartmentService) AssignDevicesToDepartment(departmentID int64, deviceIDs []int64) error {
	o := orm.NewOrm()

	// 验证部门是否存在
	department := models.Department{Id: departmentID}
	if err := o.Read(&department); err != nil {
		return fmt.Errorf("部门不存在: %v", err)
	}

	if department.LevelType != "PROJECT" {
		return fmt.Errorf("设备需要分配于项目层级下")
	}

	// 批量更新设备部门关联
	if len(deviceIDs) > 0 {
		_, err := o.QueryTable(new(models.Device)).
			Filter("id__in", deviceIDs).
			Update(orm.Params{
				"department_id": departmentID,
			})
		if err != nil {
			return fmt.Errorf("分配设备到部门失败: %v", err)
		}
	}

	return nil
}

// ReDistributionFromDepartment 从部门再分配设备
func (s *DepartmentService) ReDistributionFromDepartment(departmentId int64, deviceIds []int64) error {
	o := orm.NewOrm()
	// 1. 先清除该部门下所有设备的部门关联
	_, err := o.QueryTable(new(models.Device)).
		Filter("department_id", departmentId).
		Update(orm.Params{
			"department_id": nil,
		})
	if err != nil {
		return fmt.Errorf("清空原设备关联失败: %v", err)
	}
	// 2. 重新分配新设备
	err = s.AssignDevicesToDepartment(departmentId, deviceIds)
	if err != nil {
		return err
	}
	return nil
}

// GetDepartmentDevices 获取部门设备列表（包括子部门）
func (s *DepartmentService) GetDepartmentDevices(departmentID int64, page, size int) (*utils.PageResult, error) {
	o := orm.NewOrm()

	// 获取部门树的所有ID
	departmentIDs, err := s.getDepartmentTreeIDs(departmentID)
	if err != nil {
		return nil, fmt.Errorf("获取部门树失败: %v", err)
	}

	// 查询部门树下的所有设备
	query := o.QueryTable(new(models.Device)).
		RelatedSel("Product", "Project", "Department").
		Filter("department_id__in", departmentIDs)

	return utils.Paginate(query, page, size, &[]models.Device{})
}

// GetNoDepartmentDevices 获取未绑定部门设备
func (s *DepartmentService) GetNoDepartmentDevices(tenantId, departmentID int64, page, size int) (*utils.PageResult, error) {
	o := orm.NewOrm()

	query := o.QueryTable(new(models.Device)).
		RelatedSel("Product", "Department").Filter("tenant_id", tenantId)
	if departmentID == 0 {
		query = query.Filter("department_id__isnull", true)
	} else {
		query = query.Filter("department_id", departmentID)
	}

	return utils.Paginate(query, page, size, &[]models.Device{})
}

// GetDepartmentDeviceTree 获取部门设备树形结构
func (s *DepartmentService) GetDepartmentDeviceTree(departmentID int64) ([]map[string]interface{}, error) {
	o := orm.NewOrm()

	// 获取部门树的所有ID
	departmentIDs, err := s.getDepartmentTreeIDs(departmentID)
	if err != nil {
		return nil, err
	}

	// 查询部门树下的所有设备
	var devices []*models.Device
	_, err = o.QueryTable(new(models.Device)).
		RelatedSel("Product", "Department").
		Filter("department_id__in", departmentIDs).
		All(&devices)
	if err != nil {
		return nil, fmt.Errorf("查询设备失败: %v", err)
	}

	// 按部门分组设备
	departmentDevicesMap := make(map[int64][]*models.Device)
	for _, device := range devices {
		if device.Department != nil {
			departmentDevicesMap[device.Department.Id] = append(departmentDevicesMap[device.Department.Id], device)
		}
	}

	// 构建部门树
	return s.buildDepartmentDeviceTree(departmentID, departmentDevicesMap)
}

// buildDepartmentDeviceTree 构建部门设备树形结构
func (s *DepartmentService) buildDepartmentDeviceTree(rootDepartmentID int64, departmentDevicesMap map[int64][]*models.Device) ([]map[string]interface{}, error) {
	o := orm.NewOrm()

	// 获取根部门信息
	var rootDepartment models.Department
	if err := o.QueryTable(new(models.Department)).
		Filter("id", rootDepartmentID).One(&rootDepartment); err != nil {
		return nil, err
	}

	// 获取所有子部门
	var children []models.Department
	_, err := o.QueryTable(new(models.Department)).
		Filter("parent_id", rootDepartmentID).All(&children)
	if err != nil {
		return nil, err
	}

	// 构建当前部门节点
	node := make(map[string]interface{})
	node["id"] = rootDepartment.Id
	node["name"] = rootDepartment.Name
	node["type"] = "Department"
	node["leader"] = rootDepartment.Leader
	node["phone"] = rootDepartment.Phone
	node["email"] = rootDepartment.Email

	// 添加设备子节点
	var deviceNodes []map[string]interface{}
	if devices, exists := departmentDevicesMap[rootDepartmentID]; exists {
		for _, device := range devices {
			deviceNode := make(map[string]interface{})
			deviceNode["id"] = device.Id
			deviceNode["name"] = device.Name
			deviceNode["type"] = "Device"
			deviceNode["status"] = device.Status
			deviceNode["productName"] = ""
			if device.Product != nil {
				deviceNode["productName"] = device.Product.Name
			}
			deviceNodes = append(deviceNodes, deviceNode)
		}
	}
	node["devices"] = deviceNodes

	// 递归构建子部门节点
	var childrenNodes []map[string]interface{}
	for _, child := range children {
		childTree, err := s.buildDepartmentDeviceTree(child.Id, departmentDevicesMap)
		if err != nil {
			return nil, err
		}
		childrenNodes = append(childrenNodes, childTree...)
	}
	node["children"] = childrenNodes

	return []map[string]interface{}{node}, nil
}

// getDepartmentTreeIDs 获取部门树的所有ID（包括自身和所有子部门）
func (s *DepartmentService) getDepartmentTreeIDs(departmentID int64) ([]int64, error) {
	o := orm.NewOrm()
	var ids []int64

	// 递归获取所有子部门ID
	var getChildrenIDs func(int64) error
	getChildrenIDs = func(parentID int64) error {
		var children []models.Department
		if _, err := o.QueryTable(new(models.Department)).
			Filter("parent_id", parentID).All(&children); err != nil {
			return err
		}

		for _, child := range children {
			ids = append(ids, child.Id)
			if err := getChildrenIDs(child.Id); err != nil {
				return err
			}
		}
		return nil
	}

	// 添加当前部门ID
	ids = append(ids, departmentID)

	// 获取所有子部门ID
	if err := getChildrenIDs(departmentID); err != nil {
		return nil, err
	}

	return ids, nil
}
