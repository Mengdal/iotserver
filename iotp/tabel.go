package iotp

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

type TagService struct{}

func NewTagService() *TagService {
	return &TagService{}
}

// 获取拥有指定标签值的所有设备
func (s *TagService) ListDevicesByTag(tagName, tagValue string) ([]string, error) {
	url := fmt.Sprintf("http://%s/tag/listDevices/%s/%s",
		IoTPServer,
		s.escapePathSegment(tagName),
		s.escapePathSegment(tagValue))

	data, err := HttpGet(url)
	if err != nil {
		log.Printf("查询设备列表失败: %v", err)
		return nil, fmt.Errorf("API请求失败: %v", err)
	}

	// 解析新的响应结构
	var response struct {
		Devices []string `json:"devices"`
		Info    string   `json:"info"`
	}
	if err := json.Unmarshal([]byte(data), &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	// 检查是否有错误信息
	//if response.Info != "" && response.Info != "OK" {
	//	return nil, fmt.Errorf("API返回错误: %s", response.Info)
	//}

	return response.Devices, nil
}

// 获取设备的所有标签
func (s *TagService) ListTagsByDevice(deviceName string) (map[string]string, error) {
	url := fmt.Sprintf("http://%s/tag/listTags/%s",
		IoTPServer,
		s.escapePathSegment(deviceName))

	data, err := HttpGet(url)
	if err != nil {
		log.Printf("查询设备标签失败: %v", err)
		return nil, fmt.Errorf("API请求失败: %v", err)
	}

	tags := make(map[string]string)
	if err := json.Unmarshal([]byte(data), &tags); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	return tags, nil
}

// 获取设备指定标签的值
func (s *TagService) GetTagValue(deviceName, tagName string) (string, error) {
	url := fmt.Sprintf("http://%s/tag/getTag/%s/%s",
		IoTPServer,
		s.escapePathSegment(deviceName),
		s.escapePathSegment(tagName))

	data, err := HttpGet(url)
	if err != nil {
		log.Printf("查询标签值失败: %v", err)
		return "", fmt.Errorf("API请求失败: %v", err)
	}

	var result struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	return result.Value, nil
}

// 为设备添加标签
func (s *TagService) AddTag(deviceName, tagName, tagValue string) error {
	url := fmt.Sprintf("http://%s/tag/addTag/%s/%s/%s",
		IoTPServer,
		s.escapePathSegment(deviceName),
		s.escapePathSegment(tagName),
		s.escapePathSegment(tagValue))

	_, err := HttpGet(url)
	if err != nil {
		log.Printf("添加标签失败: %v", err)
		return fmt.Errorf("API请求失败: %v", err)
	}

	return nil
}

// 删除设备标签
func (s *TagService) RemoveTag(deviceName, tagName string) error {
	url := fmt.Sprintf("http://%s/tag/removeTag/%s/%s",
		IoTPServer,
		s.escapePathSegment(deviceName),
		s.escapePathSegment(tagName))

	_, err := HttpGet(url)
	if err != nil {
		log.Printf("删除标签失败: %v", err)
		return fmt.Errorf("API请求失败: %v", err)
	}

	return nil
}

// 处理URL路径中的特殊字符
func (s *TagService) escapePathSegment(segment string) string {
	// 替换可能影响URL路径的特殊字符
	return strings.ReplaceAll(segment, "/", "_")
}
