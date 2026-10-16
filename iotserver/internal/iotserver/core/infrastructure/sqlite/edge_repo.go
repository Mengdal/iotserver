package sqlite

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"iotserver/internal/models"
	"iotserver/internal/pkg/errort"
)

/*********************** 参考系统（/edge 协议）接入 ***********************/

// gatewayByPkSn 按「外部协议产品标识 + 序列号」定位网关。
//
// 参考系统的网关身份是 /edge/{kind}/{pk}/{sn}/...，实测 pk 与 sn 取值可相同，
// 但协议上是两段，两段都参与匹配。空 pk 视为「任意」——这样在平台界面里手工建的
// 网关（pk 未填）也能被 /edge 报文命中，便于把存量网关逐步接过来。
//
// 上行报文此刻还没有租户上下文，调用方必须显式使用平台作用域。
func gatewayByPkSn(c *Client, pk, sn string) (models.Gateway, error) {
	sn = strings.TrimSpace(sn)
	if sn == "" {
		return models.Gateway{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("gateway sn is required"))
	}
	var g models.Gateway
	q := c.Pool.Table(models.TableGateway).Where("sn = ?", sn)
	if pk = strings.TrimSpace(pk); pk != "" {
		q = q.Where("pk = ? OR pk = ''", pk)
	}
	if err := q.First(&g).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return g, errort.NewCommonErr(errort.DefaultResourcesNotFound,
				fmt.Errorf("gateway pk=%s sn=%s not found", pk, sn))
		}
		return g, errort.NewCommonEdgeX(errort.DefaultSystemError, "gateway query failed", err)
	}
	return g, nil
}

// upsertGatewayDevice 按 (gateway_id, device_key) 插入或更新设备，返回是否新建。
//
// 幂等键就是这一对列：网关重连后重发全量清单、或清单与数据报文乱序到达，
// 都不会产生重复设备。调用方需使用该网关所属租户的作用域。
//
// 注意主键用 uuid 而不是 utils.RandomNum()：后者是 math/rand + 时间种子 + 8 位数字
// （1e8 空间，见 internal/pkg/utils/uuid.go:17），一个网关一包就可能报几百台设备，
// 用它当主键会有实际碰撞风险。
func upsertGatewayDevice(c *Client, d models.Device) (models.Device, bool, error) {
	d.GatewayId = strings.TrimSpace(d.GatewayId)
	d.DeviceKey = strings.TrimSpace(d.DeviceKey)
	if d.GatewayId == "" || d.DeviceKey == "" {
		return d, false, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("gateway id and device key are required"))
	}

	// 用 Find 而非 First：未注册是正常分支（首次上报/首次接入），
	// First 会返回 ErrRecordNotFound 并被 GORM 默认 logger 打成 error 级日志。
	var found []models.Device
	if err := c.Pool.Table(models.TableDevice).
		Where("gateway_id = ? AND device_key = ?", d.GatewayId, d.DeviceKey).
		Limit(1).Find(&found).Error; err != nil {
		return d, false, errort.NewCommonEdgeX(errort.DefaultSystemError, "query device by gateway key failed", err)
	}

	now := time.Now().UnixMilli()
	if len(found) > 0 {
		existing := found[0]
		updates := map[string]interface{}{
			"modified": now,
			// 重新出现在清单/数据里 → 恢复正常状态（此前可能被 full 清单标记为摘除）
			"provision_state": models.DeviceStateActive,
		}
		if d.Name != "" {
			updates["name"] = d.Name
			existing.Name = d.Name
		}
		if d.DeviceSn != "" {
			updates["device_sn"] = d.DeviceSn
			existing.DeviceSn = d.DeviceSn
		}
		if d.Description != "" {
			updates["description"] = d.Description
			existing.Description = d.Description
		}
		if err := c.Pool.Table(models.TableDevice).Where("id = ?", existing.Id).Updates(updates).Error; err != nil {
			return d, false, errort.NewCommonEdgeX(errort.DefaultSystemError, "update gateway device failed", err)
		}
		existing.ProvisionState = models.DeviceStateActive
		return existing, false, nil
	}

	if d.Id == "" {
		d.Id = uuid.NewString()
	}
	if d.ProvisionSource == "" {
		d.ProvisionSource = models.DeviceSourceGateway
	}
	if d.ProvisionState == "" {
		d.ProvisionState = models.DeviceStateActive
	}
	d.Timestamps = models.Timestamps{Created: now, Modified: now}
	if err := c.Pool.Table(models.TableDevice).Create(&d).Error; err != nil {
		return d, false, errort.NewCommonEdgeX(errort.DefaultSystemError, "create gateway device failed", err)
	}
	return d, true, nil
}

// listGatewayDevices 取网关下已注册设备，按 device_key 索引（清单全量比对用）。
func listGatewayDevices(c *Client, gatewayId string) (map[string]models.Device, error) {
	out := map[string]models.Device{}
	if gatewayId == "" {
		return out, nil
	}
	var list []models.Device
	if err := c.Pool.Table(models.TableDevice).
		Where("gateway_id = ?", gatewayId).Find(&list).Error; err != nil {
		return nil, errort.NewCommonEdgeX(errort.DefaultSystemError, "list gateway devices failed", err)
	}
	for _, d := range list {
		if d.DeviceKey != "" {
			out[d.DeviceKey] = d
		}
	}
	return out, nil
}

// markDevicesAbsent 把「全量清单里已消失」的设备标记为摘除。
//
// 只改状态不删行：时序历史在时序库里，而删除元数据会断掉告警规则/组态的引用。
func markDevicesAbsent(c *Client, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := c.Pool.Table(models.TableDevice).
		Where("id IN ?", ids).
		Where("provision_state <> ?", models.DeviceStateAbsent).
		Updates(map[string]interface{}{
			"provision_state": models.DeviceStateAbsent,
			"modified":        time.Now().UnixMilli(),
		})
	if res.Error != nil {
		return 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "mark devices absent failed", res.Error)
	}
	return res.RowsAffected, nil
}

// upsertDeviceTags 写入/更新设备级测点定义。
//
// 只增改、**不删**：人工校准过的测点（单位/类型）不该因为某次清单里没带上就被抹掉；
// 多余的测点定义不会造成错误数据（不像多余的设备会污染列表），保守处理更安全。
func upsertDeviceTags(c *Client, deviceId string, tags []models.DeviceTag) (int, error) {
	if deviceId == "" {
		return 0, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("device id is required"))
	}
	if len(tags) == 0 {
		return 0, nil
	}

	var existing []models.DeviceTag
	if err := c.Pool.Table(models.TableDeviceTag).
		Where("device_id = ?", deviceId).Find(&existing).Error; err != nil {
		return 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "list device tags failed", err)
	}
	byCode := make(map[string]models.DeviceTag, len(existing))
	for _, t := range existing {
		byCode[t.Code] = t
	}

	now := time.Now().UnixMilli()
	created := 0
	for _, t := range tags {
		t.Code = strings.TrimSpace(t.Code)
		if t.Code == "" {
			continue
		}
		old, ok := byCode[t.Code]
		if !ok {
			t.Id = uuid.NewString()
			t.DeviceId = deviceId
			if t.Source == "" {
				t.Source = models.DeviceTagSourceAuto
			}
			if t.Type == "" {
				t.Type = "unknown"
			}
			if t.DiscoveredAt == 0 {
				t.DiscoveredAt = now
			}
			t.Timestamps = models.Timestamps{Created: now, Modified: now}
			if err := c.Pool.Table(models.TableDeviceTag).Create(&t).Error; err != nil {
				return created, errort.NewCommonEdgeX(errort.DefaultSystemError, "create device tag failed", err)
			}
			created++
			continue
		}
		// 已存在：只补自动可得的字段，**不覆盖人工校准过的单位与类型**
		updates := map[string]interface{}{"modified": now}
		if t.Name != "" {
			updates["name"] = t.Name
		}
		if t.AccessMode != "" {
			updates["access_mode"] = t.AccessMode
		}
		if old.Unit == "" && t.Unit != "" {
			updates["unit"] = t.Unit
		}
		if old.Type == "unknown" && t.Type != "" && t.Type != "unknown" {
			updates["type"] = t.Type
		}
		if err := c.Pool.Table(models.TableDeviceTag).Where("id = ?", old.Id).Updates(updates).Error; err != nil {
			return created, errort.NewCommonEdgeX(errort.DefaultSystemError, "update device tag failed", err)
		}
	}
	return created, nil
}

// listDeviceTags 取设备级测点定义（读路径在无产品物模型时回落用）。
func listDeviceTags(c *Client, deviceId string) ([]models.DeviceTag, error) {
	if deviceId == "" {
		return nil, nil
	}
	var list []models.DeviceTag
	if err := c.Pool.Table(models.TableDeviceTag).
		Where("device_id = ?", deviceId).Find(&list).Error; err != nil {
		return nil, errort.NewCommonEdgeX(errort.DefaultSystemError, "list device tags failed", err)
	}
	return list, nil
}

/*********************** 对外方法 ***********************/

func (c *Client) GatewayByPkSn(pk, sn string) (models.Gateway, error) {
	return gatewayByPkSn(c, pk, sn)
}

func (c *Client) UpsertGatewayDevice(d models.Device) (models.Device, bool, error) {
	return upsertGatewayDevice(c, d)
}

func (c *Client) ListGatewayDevices(gatewayId string) (map[string]models.Device, error) {
	return listGatewayDevices(c, gatewayId)
}

func (c *Client) MarkDevicesAbsent(ids []string) (int64, error) {
	return markDevicesAbsent(c, ids)
}

func (c *Client) UpsertDeviceTags(deviceId string, tags []models.DeviceTag) (int, error) {
	return upsertDeviceTags(c, deviceId, tags)
}

func (c *Client) ListDeviceTags(deviceId string) ([]models.DeviceTag, error) {
	return listDeviceTags(c, deviceId)
}
