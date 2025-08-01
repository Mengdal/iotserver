package services

import (
	"fmt"
	"iotServer/iotp"
	"strconv"
	"time"
)

func BindDeviceTags(s iotp.TagService, deviceNames []string, productID int64, productName string) error {
	productIDStr := strconv.FormatInt(productID, 10)
	nowStr := strconv.FormatInt(time.Now().Unix(), 10)

	for _, deviceName := range deviceNames {
		if deviceName == "" {
			continue
		}
		tags := map[string]string{
			"productName": productName,
			"productId":   productIDStr,
			"created":     nowStr,
			"status":      "0",
			"lastOnline":  "0",
		}

		for key, val := range tags {
			if err := s.AddTag(deviceName, key, val); err != nil {
				return fmt.Errorf("设备 %s 绑定失败: %v", deviceName, err)
			}
		}
	}

	return nil
}
