package mysql

import (
	"fmt"

	"gorm.io/gorm"

	"iotserver/internal/models"
	"iotserver/internal/pkg/errort"
)

// 网关接入相关：按「网关 + 网关内设备标识」定位设备。
// 网关上报时只带自己的 deviceKey（如 M-001），平台据此映射到平台侧 device.id。

func deviceByGatewayAndKey(c *Client, gatewayId, deviceKey string) (models.Device, error) {
	var device models.Device
	if gatewayId == "" || deviceKey == "" {
		return device, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("gateway id and device key are required"))
	}
	// 用 Find + Limit(1) 而不是 First：网关上报是**先按网关绑定查找、未绑定时再按设备 SN 兜底**
	// 的两步流程（见 gatewayingest.resolveDevice），所以「这里查不到」是正常分支而非异常。
	// First 会返回 gorm.ErrRecordNotFound，GORM 默认 logger 会把它打成 error 级日志，
	// 结果是未绑定网关的设备每上报一次就刷一条看起来像报错的日志。Find 查空不报错，行为一致但不再刷日志。
	var found []models.Device
	if err := c.Pool.Table("device").
		Where("gateway_id = ? AND device_key = ?", gatewayId, deviceKey).
		Limit(1).
		Find(&found).Error; err != nil {
		return device, errort.NewCommonEdgeX(errort.DefaultSystemError, "query device by gateway key failed", err)
	}
	if len(found) == 0 {
		return device, errort.NewCommonErr(errort.DeviceNotExist,
			fmt.Errorf("device key(%s) not found in gateway(%s)", deviceKey, gatewayId))
	}
	return found[0], nil
}

func deviceBySn(c *Client, deviceSn string) (models.Device, error) {
	var device models.Device
	if deviceSn == "" {
		return device, errort.NewCommonErr(errort.DefaultIdEmpty, fmt.Errorf("device sn is empty"))
	}
	err := c.Pool.Table("device").Where("device_sn = ?", deviceSn).First(&device).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return device, errort.NewCommonErr(errort.DeviceNotExist, fmt.Errorf("device sn(%s) not found", deviceSn))
		}
		return device, errort.NewCommonEdgeX(errort.DefaultSystemError, "query device by sn failed", err)
	}
	return device, nil
}

func (c *Client) DeviceByGatewayAndKey(gatewayId, deviceKey string) (models.Device, error) {
	return deviceByGatewayAndKey(c, gatewayId, deviceKey)
}

func (c *Client) DeviceBySn(deviceSn string) (models.Device, error) {
	return deviceBySn(c, deviceSn)
}
