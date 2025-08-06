package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/core/logs"
	"io"
	"net/http"
	"strconv"
	"time"
)

// InterfaceToString 将任意类型的变量安全地转换为字符串
func InterfaceToString(value interface{}) string {
	var key string
	if value == nil {
		return key
	}

	switch value.(type) {
	case float64:
		ft := value.(float64)
		key = strconv.FormatFloat(ft, 'f', -1, 64)
	case float32:
		ft := value.(float32)
		key = strconv.FormatFloat(float64(ft), 'f', -1, 64)
	case int:
		it := value.(int)
		key = strconv.Itoa(it)
	case uint:
		it := value.(uint)
		key = strconv.Itoa(int(it))
	case int8:
		it := value.(int8)
		key = strconv.Itoa(int(it))
	case uint8:
		it := value.(uint8)
		key = strconv.Itoa(int(it))
	case int16:
		it := value.(int16)
		key = strconv.Itoa(int(it))
	case uint16:
		it := value.(uint16)
		key = strconv.Itoa(int(it))
	case int32:
		it := value.(int32)
		key = strconv.Itoa(int(it))
	case uint32:
		it := value.(uint32)
		key = strconv.Itoa(int(it))
	case int64:
		it := value.(int64)
		key = strconv.FormatInt(it, 10)
	case uint64:
		it := value.(uint64)
		key = strconv.FormatUint(it, 10)
	case string:
		key = value.(string)
	case []byte:
		key = string(value.([]byte))
	default:
		newValue, _ := json.Marshal(value)
		key = string(newValue)
	}

	return key
}

// IsInEffectiveTime 检查当前时间是否在通知有效时间内
func IsInEffectiveTime(notifyConfig map[string]interface{}, currentTime string) bool {
	// 默认有效时间
	startTime := "08:00:00"
	endTime := "17:00:00"

	// 从配置中获取有效时间
	if start, ok := notifyConfig["start_effect_time"].(string); ok && start != "" {
		startTime = start
	}
	if end, ok := notifyConfig["end_effect_time"].(string); ok && end != "" {
		endTime = end
	}

	// 判断当前时间是否在有效范围内
	return currentTime >= startTime && currentTime <= endTime
}

// FormatTimestamp 将 float64 时间戳或时间字符串格式化为本地时间字符串
func FormatTimestamp(timestamp interface{}) string {
	// 设置时区为上海
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.Local // 兜底使用本地默认时区
	}

	// 处理不同的输入类型
	switch v := timestamp.(type) {
	case float64:
		// 防止负数或非时间值
		if v <= 0 {
			return "Invalid timestamp"
		}

		// 自动识别毫秒级别
		if v > 1e12 {
			v = v / 1000
		}

		// 将 float64 转为 int64 秒部分，保留小数部分作为纳秒
		sec := int64(v)
		nsec := int64((v - float64(sec)) * 1e9)
		t := time.Unix(sec, nsec).In(loc)
		return t.Format("2006-01-02 15:04:05")

	case int64:
		if v <= 0 {
			return "Invalid timestamp"
		}

		// 自动识别毫秒级别
		if v > 1e12 {
			v = v / 1000
		}

		t := time.Unix(v, 0).In(loc)
		return t.Format("2006-01-02 15:04:05")

	case string:
		if v == "" {
			return "Invalid timestamp"
		}

		// 尝试解析多种时间格式
		formats := []string{
			"2006-01-02T15:04:05.9999999-07:00",
			"2006-01-02T15:04:05.999999-07:00",
			"2006-01-02T15:04:05.999-07:00",
			"2006-01-02T15:04:05-07:00",
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05Z",
			"2006-01-02T15:04:05.000Z",
			time.RFC3339,
			time.RFC3339Nano,
		}

		for _, format := range formats {
			if t, err := time.Parse(format, v); err == nil {
				return t.In(loc).Format("2006-01-02 15:04:05")
			}
		}

		// 如果标准格式都无法解析，尝试直接解析
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", v, loc); err == nil {
			return t.Format("2006-01-02 15:04:05")
		}

		return "Invalid timestamp format"

	default:
		return "Unsupported timestamp type"
	}
}

// SendHttpPost 发送HTTP POST请求的通用方法
func SendHttpPost(url string, data interface{}) error {
	// 将数据转换为JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		logs.Error("数据序列化失败: %v", err)
		return err
	}

	// 发送POST请求
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		logs.Error("发送HTTP POST请求失败: %v", err)
		return err
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logs.Error("读取响应失败: %v", err)
		return err
	}

	// 检查响应状态
	if resp.StatusCode == 200 {
		logs.Info("HTTP POST请求发送成功: %s", string(body))
	} else {
		return fmt.Errorf("HTTP POST请求发送失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	return nil
}
