/*******************************************************************************
 * Copyright 2017 Dell Inc.
 * Copyright (c) 2019 Intel Corporation
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *******************************************************************************/
package mysql

import (
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/utils"
	"iotserver/internal/tools/sqldb/sqlite"
	"time"
)

// deviceById 按主键查询设备。
//
// 注意：这里必须写成 `Where("id = ?", id)`，**不能**用 GORM 的内联主键简写
// `First(&device, id)`。GORM v1.23 的 BuildCondition 会先做 `strconv.Atoi(id)`：
// 只有能解析成整数的字符串才被当作主键值参数化；解析失败时它把整个值当**原始 SQL**
// 拼进 WHERE（`clause.Expr{SQL: s}`），于是非纯数字的 ID（如 uuid）会拼出
// `WHERE a604a540-af9b-4a4f-...` 这种语法错误——SQLite 报 unrecognized token。
// 该写法过去"能用"只是因为所有主键恰好都是 8 位数字（utils.RandomNum）。
func deviceById(c *Client, id string) (device models.Device, edgeXErr error) {
	if id == "" {
		return device, errort.NewCommonEdgeX(errort.DefaultIdEmpty, "device id is empty", nil)
	}
	//err := c.client.GetObject(&models.Device{Id: id}, &device)
	err := c.Pool.Table(device.TableName()).Preload("Product").Where("id = ?", id).First(&device).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return device, errort.NewCommonErr(errort.DeviceNotExist, fmt.Errorf("device id(%s) not found", id))
		}
		return device, errort.NewCommonErr(errort.DefaultSystemError, fmt.Errorf("query device fail (Id:%s), %s", device.Id, err))
	}
	return
}

func deviceOnlineById(c *Client, id string) (edgeXErr error) {
	d := models.Device{}
	tx := c.Pool.Table(d.TableName())
	edgeXErr = tx.Where("id = ?", id).Updates(map[string]interface{}{"status": constants.DeviceStatusOnline, "last_online_time": utils.MakeTimestamp()}).Error
	if edgeXErr != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "deviceOnlineById failed", edgeXErr)
	}
	return nil
}

func deviceOfflineById(c *Client, id string) (edgeXErr error) {
	d := models.Device{}
	tx := c.Pool.Table(d.TableName())
	edgeXErr = tx.Where("id = ?", id).Updates(map[string]interface{}{"status": constants.DeviceStatusOffline}).Error
	if edgeXErr != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "deviceOnlineById failed", edgeXErr)
	}
	return nil
}

func deviceOfflineByCloudInstanceId(c *Client, id string) (edgeXErr error) {
	d := models.Device{}
	tx := c.Pool.Table(d.TableName())
	edgeXErr = tx.Where("cloud_instance_id = ?", id).Updates(map[string]interface{}{"status": constants.DeviceStatusOffline}).Error
	if edgeXErr != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "deviceOnlineById failed", edgeXErr)
	}
	return nil
}

func msgReportDeviceById(c *Client, id string) (device models.Device, edgeXErr error) {
	if id == "" {
		return device, errort.NewCommonEdgeX(errort.DefaultIdEmpty, "device id is empty", nil)
	}
	//err := c.client.GetObject(&models.Device{Id: id}, &device)
	err := c.Pool.Table(device.TableName()).Preload("Product").Where("id = ?", id).First(&device).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return device, errort.NewCommonErr(errort.DeviceNotExist, fmt.Errorf("device id(%s) not found", id))
		}
		return device, errort.NewCommonErr(errort.DefaultSystemError, fmt.Errorf("query device fail (Id:%s), %s", device.Id, err))
	}
	return
}

func deviceByCloudId(c *Client, id string) (device models.Device, edgeXErr error) {
	if id == "" {
		return device, errort.NewCommonEdgeX(errort.DefaultIdEmpty, "device cloudId is empty", nil)
	}
	err := c.client.GetObject(&models.Device{CloudDeviceId: id}, &device)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return device, errort.NewCommonErr(errort.DeviceNotExist, fmt.Errorf("device cloudId (%s) not found", id))
		}
		return device, errort.NewCommonErr(errort.DefaultSystemError, fmt.Errorf("query device fail (cloudId:%s), %s", device.Id, err))
	}
	return
}

func devicesSearch(c *Client, offset int, limit int, req dtos.DeviceSearchQueryRequest) (devices []models.Device, count uint32, edgeXErr error) {
	dp := models.Device{}
	var total int64
	tx := c.Pool.Table(dp.TableName())
	tx = sqlite.BuildCommonCondition(tx, dp, req.BaseSearchConditionQuery)

	if req.Name != "" {
		tx = tx.Where("`name` LIKE ?", sqlite.MakeLikeParams(req.Name))
	}

	if req.Platform != "" {
		tx = tx.Where("`platform` = ?", req.Platform)
	}

	if req.ProductId != "" {
		tx = tx.Where("`product_id` = ?", req.ProductId)
	}

	if req.CloudProductId != "" {
		tx = tx.Where("`cloud_product_id` = ?", req.CloudProductId)

	}
	if req.CloudInstanceId != "" {
		tx = tx.Where("`cloud_instance_id` = ?", req.CloudInstanceId)
	}

	if req.DriveInstanceId != "" {
		tx = tx.Where("`drive_instance_id` = ?", req.DriveInstanceId)
	}
	if req.Status != "" {
		tx = tx.Where("`status` = ?", req.Status)
	}

	err := tx.Count(&total).Error
	if err != nil {
		return []models.Device{}, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "devices failed query from the database", err)
	}

	err = tx.Offset(offset).Preload("Product").Limit(limit).Find(&devices).Error
	if err != nil {
		return []models.Device{}, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "devices failed query from the database", err)
	}

	return devices, uint32(total), nil
}

func batchUpsertDevice(c *Client, d []models.Device) (int64, error) {
	if len(d) <= 0 {
		return 0, nil
	}
	tx := c.Pool.Clauses(
		clause.OnConflict{
			UpdateAll: true,
		}).CreateInBatches(&d, 10000)
	num := tx.RowsAffected
	err := tx.Error
	if err != nil {
		return num, err
	}
	return num, nil
}

func batchDeleteDevice(c *Client, ids []string) error {
	d := models.Device{}
	tx := c.Pool.Table(d.TableName())
	err := tx.Delete(d, ids).Error
	if err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "batchDeleteDevice failed", err)
	}
	return nil
}

func batchUnBindDevice(c *Client, ids []string) error {
	d := models.Device{}
	tx := c.Pool.Table(d.TableName())
	err := tx.Where("id IN ?", ids).Updates(map[string]interface{}{"drive_instance_id": ""}).Error
	if err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "batchDeleteDevice failed", err)
	}
	return nil
}

func batchBindDevice(c *Client, ids []string, driverInstanceId string) error {
	d := models.Device{}
	tx := c.Pool.Table(d.TableName())
	err := tx.Where("id IN ?", ids).Updates(map[string]interface{}{"drive_instance_id": driverInstanceId}).Error
	if err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "batchDeleteDevice failed", err)
	}
	return nil
}

func deleteDeviceById(c *Client, id string) error {
	if id == "" {
		return errort.NewCommonEdgeX(errort.DefaultIdEmpty, "device id is empty", nil)
	}
	err := c.client.DeleteObject(&models.Device{Id: id})
	if err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "device deletion failed", err)
	}
	return nil
}

func deleteDeviceByCloudInstanceId(c *Client, cloudInstanceId string) error {
	if cloudInstanceId == "" {
		return errort.NewCommonEdgeX(errort.DefaultIdEmpty, "cloudInstanceId id is empty", nil)
	}
	err := c.client.DeleteObject(&models.Device{CloudInstanceId: cloudInstanceId})
	if err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "device deletion failed", err)
	}
	return nil
}

func updateDevice(c *Client, dl models.Device) error {
	dl.Modified = utils.MakeTimestamp()
	err := c.client.UpdateObject(&dl)
	if err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "device update failed", err)
	}
	return nil
}

func addDevice(c *Client, device models.Device) (string, error) {
	exists, edgeXErr := deviceNameExist(c, device.Name)
	if edgeXErr != nil {
		return "", edgeXErr
	} else if exists {
		return "", errort.NewCommonEdgeX(errort.DefaultNameRepeat, fmt.Sprintf("device name %s exists", device.Name), edgeXErr)
	}
	exists, edgeXErr = productIdExist(c, device.ProductId)
	if edgeXErr != nil {
		return "", edgeXErr
	} else if !exists {
		return "", errort.NewCommonEdgeX(errort.DeviceProductIdNotFound, fmt.Sprintf("device product %s not exists", device.ProductId), edgeXErr)
	}
	ts := utils.MakeTimestamp()
	if device.Created == 0 {
		device.Created = ts
	}
	device.Modified = ts

	err := c.client.CreateObject(&device)
	if err != nil {
		edgeXErr = errort.NewCommonEdgeX(errort.DefaultSystemError, "device creation failed", err)
	}

	return device.Id, edgeXErr
}

func addOrUpdateAuth(c *Client, auth models.MqttAuth) error {
	exists, err := c.client.ExistObject(&models.MqttAuth{ClientId: auth.ClientId})
	if err != nil {
		return err
	}
	if !exists {
		ts := utils.MakeTimestamp()
		if auth.Created == 0 {
			auth.Created = ts
		}
		auth.Modified = ts
		err = c.client.CreateObject(&auth)
		if err != nil {
			return errort.NewCommonEdgeX(errort.DefaultSystemError, "mqtt auch creation failed", err)
		}
	}
	return nil
}

func addMqttAuth(c *Client, auth models.MqttAuth) (string, error) {
	var edgeXErr error
	ts := utils.MakeTimestamp()
	if auth.Created == 0 {
		auth.Created = ts
	}
	auth.Modified = ts

	err := c.client.CreateObject(&auth)
	if err != nil {
		edgeXErr = errort.NewCommonEdgeX(errort.DefaultSystemError, "mqtt auch creation failed", err)
	}
	return auth.Id, edgeXErr
}

func deviceNameExist(c *Client, name string) (bool, error) {
	exists, err := c.client.ExistObject(&models.Product{Name: name})
	if err != nil {
		return false, err
	}
	return exists, nil
}

func deviceMqttAuthInfo(c *Client, id string) (mqttAuth models.MqttAuth, edgeXErr error) {
	if id == "" {
		return mqttAuth, errort.NewCommonEdgeX(errort.DefaultIdEmpty, "device id is empty", nil)
	}
	err := c.client.GetObject(&models.MqttAuth{ResourceId: id, ResourceType: constants.DeviceResource}, &mqttAuth)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return mqttAuth, errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("mqtt auth resoure id(%s) not found", id))
		}
		return mqttAuth, errort.NewCommonErr(errort.DefaultSystemError, fmt.Errorf("query mqtt auth fail (resoureId:%s), %s", id, err))
	}
	return
}

// markStaleDevicesOffline 把「最后在线时间早于 beforeMs」的在线设备批量置为离线。
//
// 只动「当前在线」且「有最后在线时间」的行：从未上过线的设备保持原状，
// 已离线的行不重复写（避免每分钟无谓地刷新 modified）。
func markStaleDevicesOffline(c *Client, beforeMs int64) (int64, error) {
	if beforeMs <= 0 {
		return 0, nil
	}
	d := models.Device{}
	tx := c.Pool.Table(d.TableName()).
		Where("status = ? AND last_online_time > 0 AND last_online_time < ?",
			constants.DeviceStatusOnline, beforeMs).
		Updates(map[string]interface{}{
			"status":   constants.DeviceStatusOffline,
			"modified": time.Now().UnixMilli(),
		})
	if tx.Error != nil {
		return 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "mark stale devices offline failed", tx.Error)
	}
	return tx.RowsAffected, nil
}
