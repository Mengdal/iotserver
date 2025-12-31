package services

import (
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
	"iotServer/iotp"
	"iotServer/models"
	"iotServer/utils"
	"strconv"
	"sync"
	"time"
)

// DevicesService 设备管理服务
type DevicesService struct{}

// GetAllDevices 获取所有设备（分页）
func (s *DevicesService) GetAllDevices(page, size int, tenantId int64, departmentId []int64, productId int64, status, name string, isTenant bool) (*utils.PageResult, error) {
	var devices []*models.Device
	o := orm.NewOrm()
	query := o.QueryTable(new(models.Device)).RelatedSel("Product", "Position", "Group")

	query = query.Filter("tenant_id", tenantId)
	if !isTenant && len(departmentId) > 0 {
		query = query.Filter("department_id__in", departmentId)
	}
	// 按产品ID筛选
	if productId > 0 {
		query = query.Filter("product_id", productId)
	}

	// 按状态筛选
	if status != "" {
		if status == "0" {
			// 当status为"0"时，筛选status为"0"或为空的设备
			query = query.Filter("status__in", []string{"0", ""})
		} else {
			query = query.Filter("status", status)
		}
	}

	// 按名称模糊查询
	if name != "" {
		query = query.Filter("name__icontains", name)
	}

	// 按组ID排序
	query = query.OrderBy("-group__name", "name", "id")

	result, err := utils.Paginate(query, page, size, &devices)
	if err != nil {
		return nil, fmt.Errorf("分页查询设备失败: %v", err)
	}

	ids := make([]int64, len(devices))
	for i, device := range devices {
		ids[i] = device.Id
	}

	// 手动设置 ProductId 和 ProductName
	for _, device := range devices {
		if device.Product != nil {
			device.ProductId = device.Product.Id
			device.ProductName = device.Product.Name
		}
		if device.Position != nil {
			device.PositionId = device.Position.Id
			device.PositionName = device.Position.FullName
		}
		if device.Department != nil {
			device.ProjectId = device.Department.Id
		}
		if device.Group != nil {
			device.GroupId = device.Group.Id
			device.GroupName = device.Group.Name
		}
	}

	return result, nil
}

// GetNoBindDevices 获取未绑定产品的设备
/*func (s *DevicesService) GetNoBindDevices() ([]*models.Device, error) {
	var devices []*models.Device
	o := orm.NewOrm()

	// 查询没有绑定产品的设备
	_, err := o.QueryTable(new(models.Device)).
		Filter("product_id__isnull", true).
		All(&devices)

	if err != nil {
		return nil, fmt.Errorf("查询未绑定设备失败: %v", err)
	}

	return devices, nil
}*/

// BindDeviceToProduct 将设备绑定到产品
func (s *DevicesService) BindDeviceToProduct(productId int64, deviceNames []string, customTags map[string]string) error {
	// 参数校验
	if productId <= 0 {
		return fmt.Errorf("产品ID必须大于0")
	}

	if len(deviceNames) == 0 {
		return fmt.Errorf("设备ID列表不能为空")
	}

	o := orm.NewOrm()
	var product models.Product
	product.Id = productId
	err := o.Read(&product)
	if err != nil {
		return fmt.Errorf("产品ID无效: %v", err)
	}

	// 批量绑定设备到产品
	for _, deviceName := range deviceNames {
		device := &models.Device{Name: deviceName}
		err := o.Read(device, "name")
		if err != nil {
			return fmt.Errorf("设备 %s 不存在: %v", deviceName, err)
		}

		// 更新设备的产品信息
		device.Product = &product

		_, err = o.Update(device)
		if err != nil {
			return fmt.Errorf("绑定设备 %s 到产品失败: %v", deviceName, err)
		}
	}

	// 加载超级表缓存
	go LoadAllDeviceCategoryKeys()

	return nil
}

// DeleteDevice 删除设备
func (s *DevicesService) DeleteDevice(deviceName string, tenantId int64) error {
	if deviceName == "" {
		return fmt.Errorf("设备ID不能为空")
	}

	o := orm.NewOrm()
	count, err := o.QueryTable(new(models.Device)).Filter("tenant_id", tenantId).Filter("name", deviceName).Delete()
	if err != nil || count == 0 {
		return fmt.Errorf("device not found ")
	}

	dropQuery := fmt.Sprintf("DROP TABLE IF EXISTS %s.`%s`", DBName, deviceName)

	t, _ := NewTDengineService()
	if _, execErr := t.db.Exec(dropQuery); execErr != nil {
		return fmt.Errorf("删除原子表失败: %v", execErr)
	}

	err = tagService.RemoveTag(deviceName, "productId")
	if err != nil {
		return fmt.Errorf("删除设备失败: %v", err)
	}
	return nil
}

func (s *DevicesService) GetDevicesTree(departmentIds []int64) ([]map[string]interface{}, error) {
	o := orm.NewOrm()

	// 查询所有产品
	var products []*models.Product
	_, err := o.QueryTable(new(models.Product)).All(&products)
	if err != nil {
		return nil, fmt.Errorf("查询产品失败: %v", err)
	}

	// 查询所有设备并关联产品信息
	var devices []*models.Device
	query := o.QueryTable(new(models.Device)).RelatedSel("Product")
	if len(departmentIds) > 0 {
		query = query.Filter("department_id__in", departmentIds)
	}
	_, err = query.All(&devices)
	if err != nil {
		return nil, fmt.Errorf("查询设备失败: %v", err)
	}

	// 创建产品ID到设备列表的映射
	productDevicesMap := make(map[int64][]*models.Device)
	for _, device := range devices {
		if device.Product != nil {
			productDevicesMap[device.Product.Id] = append(productDevicesMap[device.Product.Id], device)
		}
	}

	// 构建树形结构
	var tree []map[string]interface{}

	// 添加所有产品节点
	for _, product := range products {
		productNode := make(map[string]interface{})
		productNode["id"] = product.Id
		productNode["name"] = product.Name
		productNode["type"] = "Product"

		// 构建设备子节点
		children := make([]map[string]interface{}, 0)
		if devices, exists := productDevicesMap[product.Id]; exists {
			for _, device := range devices {
				deviceNode := make(map[string]interface{})
				deviceNode["id"] = device.Id
				deviceNode["name"] = device.Name
				deviceNode["type"] = "Device"
				children = append(children, deviceNode)
			}
		}

		productNode["children"] = children
		tree = append(tree, productNode)
	}

	return tree, nil
}

func BindDeviceTags(s iotp.TagService, tenantId int64, deviceNames []string, productID int64, productName string, productKey string, categoryId int64, tags map[string]string) error {
	if len(deviceNames) == 0 {
		return nil
	}

	o := orm.NewOrm()
	t, _ := NewTDengineService()
	// 如果自定义模型则使用product_key作为超级表Key
	category := models.Category{Id: categoryId}
	var categoryKey string
	err := o.Read(&category)
	if err != nil {
		categoryKey = productKey
	} else {
		categoryKey = category.CategoryKey
	}

	// 批量处理设备绑定
	errChan := make(chan error, len(deviceNames))
	semaphore := make(chan struct{}, 1) // 限制并发数
	var wg sync.WaitGroup

	for _, deviceName := range deviceNames {
		wg.Add(1)
		semaphore <- struct{}{} // 占用一个并发槽
		go func(deviceName string) {
			defer wg.Done()
			defer func() { <-semaphore }() // 释放并发槽

			// 创建 / 更新 TDengine 子表
			if err := BindTDDevice(t, o, tenantId, deviceName, productID, productKey, categoryKey, tags); err != nil {
				errChan <- fmt.Errorf("设备 %s 标签绑定失败: %v", deviceName, err)
				return
			}

			// 绑定 IOTP 设备标签
			//productIDStr := strconv.FormatInt(productID, 10)
			//nowStr := strconv.FormatInt(time.Now().Unix(), 10)
			//if err := bindIOTPDevice(s, deviceName, productIDStr, productName, nowStr, tags); err != nil {
			//	errChan <- fmt.Errorf("设备 %s 标签绑定失败: %v", deviceName, err)
			//	return
			//}

			// IOTP 标签（异步）
			go func() {
				_ = bindIOTPDevice(
					s,
					deviceName,
					strconv.FormatInt(productID, 10),
					productName,
					strconv.FormatInt(time.Now().Unix(), 10),
					tags,
				)
			}()

		}(deviceName)
	}

	wg.Wait()
	close(errChan)

	if len(errChan) > 0 {
		return <-errChan // 返回第一个错误
	}
	return nil
}
func bindIOTPDevice(s iotp.TagService, deviceName, productIDStr, productName, nowStr string, tags map[string]string) error {
	/*	initTags := map[string]string{
			"productName": productName,
			"productId": productIDStr,
			"created":     nowStr,
			"status":      "0",
			"lastOnline":  "0",
		}
		// 将 tags 合并到 initTags 中
		for key, val := range tags {
			initTags[key] = val
		}
		for key, val := range initTags {
			if err := s.AddTagEdge(deviceName, key, val); err != nil {
				return fmt.Errorf("设备 %s 绑定失败: %v", deviceName, err)
			}
		}*/
	// 只保留 productId 标签
	if err := s.AddTagEdge(deviceName, "productId", productIDStr); err != nil {
		return fmt.Errorf("设备 %s 绑定失败: %v", deviceName, err)
	}
	return nil
}

// 设备绑定时 创建子表
func BindTDDevice(service *TDengineService, o orm.Ormer, tenantId int64, deviceName string, productId int64, productKey string, categoryKey string, tags map[string]string) error {
	var query string
	product := models.Product{Id: productId}
	positionId, _ := strconv.ParseInt(tags["positionId"], 10, 64)
	groupId, _ := strconv.ParseInt(tags["groupId"], 10, 64)
	description, _ := tags["description"]

	// 开始事务
	tx, err := o.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %v", err)
	}

	// 使用 defer 确保事务被正确处理
	var txCommitted bool
	defer func() {
		if !txCommitted {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				logs.Error("事务回滚失败:", rollbackErr)
			}
		}
	}()

	device := models.Device{Name: deviceName, Product: &product, CategoryKey: categoryKey, Position: &models.Position{Id: positionId}, Group: &models.Group{Id: groupId}, Description: description, Tenant: tenantId}
	if err := tx.Read(&device, "name"); err == nil {
		// 设备存在，检查是否需要更换超级表
		if device.CategoryKey != categoryKey {
			// 超级表发生变化，需要重建子表
			// 1. 删除原子表
			dropQuery := fmt.Sprintf("DROP TABLE IF EXISTS %s.`%s`", DBName, device.Name)
			if _, execErr := service.db.Exec(dropQuery); execErr != nil {
				return fmt.Errorf("删除原子表失败: %v", execErr)
			}

			// 2. 更新设备信息
			device.Product = &product
			device.CategoryKey = categoryKey
			device.Position = &models.Position{Id: positionId}
			device.Group = &models.Group{Id: groupId}
			device.Description = description
			device.BeforeUpdate()
			if _, updateErr := tx.Update(&device); updateErr != nil {
				return fmt.Errorf("更新设备信息失败: %v", updateErr)
			}

			// 3. 创建新的子表
			query = fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.`%s` USING %s.`%s` (%s) TAGS (%d)",
				DBName, device.Name, DBName, categoryKey, SubLabel, productId)
			if _, execErr := service.db.Exec(query); execErr != nil {
				logs.Error("创建新子表失败:", execErr)
				return fmt.Errorf("创建新子表失败: %v", execErr)
			}
		} else {
			// 超级表未变，只更新设备信息和标签
			device.Product = &product
			device.CategoryKey = categoryKey
			device.Position = &models.Position{Id: positionId}
			device.Group = &models.Group{Id: groupId}
			device.Description = description
			device.BeforeUpdate()
			if _, updateErr := tx.Update(&device); updateErr != nil {
				return fmt.Errorf("更新设备信息失败: %v", updateErr)
			}

			// 更新子表的 TAG
			query = fmt.Sprintf("ALTER TABLE %s.`%s` SET TAG productid=%d", DBName, device.Name, productId)
			if _, execErr := service.db.Exec(query); execErr != nil {
				logs.Error("更新子表TAG失败:", execErr)
				return fmt.Errorf("更新子表TAG失败: %v", execErr)
			}
		}
	} else {
		// 设备不存在，创建新设备
		device.BeforeInsert()
		if _, insertErr := tx.Insert(&device); insertErr != nil {
			return fmt.Errorf("插入设备信息失败: %v", insertErr)
		}

		query = fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.`%s` USING %s.`%s` (%s) TAGS (%d)",
			DBName, device.Name, DBName, categoryKey, SubLabel, productId)
		if _, execErr := service.db.Exec(query); execErr != nil {
			logs.Error("创建子表TAG失败:", execErr)
			return fmt.Errorf("请重新发布产品")
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %v", err)
	}
	txCommitted = true

	return nil
}
