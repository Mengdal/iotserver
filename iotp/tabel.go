package iotp

import (
	"encoding/json"
	"fmt"
	"iotServer/utils"
	"log"
	"strings"
)

type TagService struct{}

func NewTagService() *TagService {
	return &TagService{}
}

// GetAllDevices 获取所有设备列表
func (s *TagService) GetAllDevices(page, size int) (*utils.PageResult, error) {
	// 参数校验
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}

	// 调用原始接口获取数据
	url := fmt.Sprintf("http://%s/tag/listDevicesByName/GWSN/%d/%d",
		IoTPServer, size, (page-1)*size)

	data, err := HttpGet(url)
	if err != nil {
		log.Printf("获取设备列表失败: %v", err)
		return nil, fmt.Errorf("API请求失败: %v", err)
	}

	// 解析响应
	var response struct {
		Devices []string `json:"devices"`
		Total   int64    `json:"total"`
	}
	if err := json.Unmarshal([]byte(data), &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	// 手动计算分页参数
	totalPage := int((response.Total + int64(size) - 1) / int64(size))
	isFirstPage := page == 1
	isLastPage := page >= totalPage

	// 直接返回分页结果
	return &utils.PageResult{
		List:       response.Devices,
		PageNumber: page,
		PageSize:   size,
		TotalPage:  totalPage,
		TotalRow:   response.Total,
		FirstPage:  isFirstPage,
		LastPage:   isLastPage,
	}, nil
}

// DeleteDevices 删除指定设备
func (s *TagService) DeleteDevices(deviceNames ...string) error {
	if len(deviceNames) == 0 {
		return fmt.Errorf("未指定要删除的设备")
	}

	url := fmt.Sprintf("http://%s/TSDELDEVICE/%s",
		IoTPServer, deviceNames)

	_, err := HttpGet(url)
	if err != nil {
		log.Printf("删除设备失败: %v", err)
		return fmt.Errorf("删除设备失败: %v", err)
	}

	return nil
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
	if tags["info"] != "" {
		return nil, fmt.Errorf("未查询到该设备")
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
