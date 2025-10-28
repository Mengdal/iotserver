package services

import (
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
	"iotServer/iotp"
	"iotServer/models"
	"strconv"
	"sync"
	"time"
)

func BindDeviceTags(s iotp.TagService, deviceNames []string, productID int64, productName string, productKey string, categoryId int64, tags map[string]string) error {
	if len(deviceNames) == 0 {
		return nil
	}
	productIDStr := strconv.FormatInt(productID, 10)
	nowStr := strconv.FormatInt(time.Now().Unix(), 10)
	o := orm.NewOrm()
	t, _ := NewTDengineService()

	// 批量处理设备绑定
	errChan := make(chan error, len(deviceNames))
	semaphore := make(chan struct{}, 10) // 限制并发数
	var wg sync.WaitGroup

	for _, deviceName := range deviceNames {
		wg.Add(1)
		semaphore <- struct{}{} // 占用一个并发槽
		go func(deviceName string) {
			defer wg.Done()
			defer func() { <-semaphore }() // 释放并发槽

			// 绑定 IOTP 设备标签
			if err := bindIOTPDevice(s, deviceName, productIDStr, productName, nowStr, tags); err != nil {
				errChan <- fmt.Errorf("设备 %s 标签绑定失败: %v", deviceName, err)
				return
			}

			// 创建 / 更新 TDengine 子表
			if err := BindTDDevice(t, o, deviceName, productID, productKey, categoryId); err != nil {
				errChan <- fmt.Errorf("设备 %s 标签绑定失败: %v", deviceName, err)
			}
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
	initTags := map[string]string{
		"productName": productName,
		"productId":   productIDStr,
		"created":     nowStr,
		"status":      "0",
		"lastOnline":  "0",
	}
	// 将 tags 合并到 initTags 中
	for key, val := range tags {
		initTags[key] = val
	}
	for key, val := range initTags {
		if err := s.AddTag(deviceName, key, val); err != nil {
			return fmt.Errorf("设备 %s 绑定失败: %v", deviceName, err)
		}
	}
	return nil
}

// 设备绑定时 创建子表
func BindTDDevice(service *TDengineService, o orm.Ormer, deviceName string, productId int64, productKey string, categoryId int64) error {
	category := models.Category{Id: categoryId}
	product := models.Product{Id: productId}
	var query string
	var categoryKey string
	// 如果自定义模型则使用product_key作为超级表Key
	if o.Read(&category) == nil {
		categoryKey = category.CategoryKey
	} else {
		categoryKey = productKey
	}
	device := models.Device{Name: deviceName, Product: &product, CategoryKey: categoryKey}
	if o.Read(&device, "name") == nil {
		// 设备存在，更新信息
		device.Product = &product
		device.CategoryKey = categoryKey
		device.BeforeUpdate()
		o.Update(&device)
		// 更新子表的 TAG
		query = fmt.Sprintf("ALTER TABLE %s.`%s` SET TAG productid=%d", DBName, device.Name, productId)
		if _, err := service.db.Exec(query); err != nil {
			logs.Error("更新子表TAG失败:", err)
			return err
		}
	} else {
		// 设备不存在，创建新设备
		device.BeforeInsert()
		o.Insert(&device)
		query = fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.`%s` USING %s.%s (%s) TAGS (%d)",
			DBName, device.Name, DBName, categoryKey, SubLabel, productId)
		service.db.Exec(query)
		if _, err := service.db.Exec(query); err != nil {
			logs.Error("更新子表TAG失败:", err)
			return err
		}
	}
	return nil
}
