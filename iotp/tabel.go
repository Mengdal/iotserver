package iotp

import (
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"iotServer/models"
	"iotServer/utils"
	"log"
	"strconv"
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
	url := fmt.Sprintf("http://%s/tag/listDevicesByName/productId/%d/%d",
		IoTPServer, size, (page-1)*size)

	data, err := HttpGet(url)
	if err != nil {
		log.Printf("获取设备列表失败: %v", err)
		return nil, fmt.Errorf(data)
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

	// 返回设备所有标签
	var deviceInfoList []map[string]string
	if len(response.Devices) > 0 {
		path := s.joinPathSegments(response.Devices)

		// ListTagsByDevice 需返回 map[string]map[string]string
		wrapper, err := s.ListTagsByDevice(path)
		if err != nil {
			log.Printf("批量查询设备标签失败: %v", err)
			return nil, fmt.Errorf("API请求失败: %v", err)
		} else {
			// 按 devices 原顺序组织结果
			for _, dn := range response.Devices {
				if info, ok := wrapper[dn]; ok && info != nil {
					// 复制一份，避免直接修改底层 map
					m := make(map[string]string, len(info)+1)
					for k, v := range info {
						m[k] = v
					}
					m["dn"] = dn
					deviceInfoList = append(deviceInfoList, m)
				} else {
					log.Printf("设备 %s 在返回结果中未找到或为空", dn)
					return nil, fmt.Errorf("API请求失败: %v", err)
				}
			}
		}
	}

	// 直接返回分页结果
	return &utils.PageResult{
		List:       deviceInfoList,
		PageNumber: page,
		PageSize:   size,
		TotalPage:  totalPage,
		TotalRow:   response.Total,
		FirstPage:  isFirstPage,
		LastPage:   isLastPage,
	}, nil
}

// 路径拼接（逐段 escape 再用 / 连接）
func (s *TagService) joinPathSegments(segments []string) string {
	esc := make([]string, 0, len(segments))
	for _, seg := range segments {
		esc = append(esc, s.escapePathSegment(seg))
	}
	return strings.Join(esc, "/")
}

// GetNoBindDevices 查询未绑定的设备
func (s *TagService) GetNoBindDevices() ([]map[string]string, error) {

	// 调用原始接口获取数据
	url := fmt.Sprintf("http://%s/tag/listDevicesByName/GWSN",
		IoTPServer)

	data, err := HttpGet(url)
	if err != nil {
		log.Printf("获取设备列表失败: %v", err)
		return nil, fmt.Errorf(data)
	}

	// 解析响应
	var response struct {
		Devices []string `json:"devices"`
		Total   int64    `json:"total"`
	}
	if err := json.Unmarshal([]byte(data), &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	var deviceInfoList []map[string]string
	if len(response.Devices) > 0 {
		// 拼接成批量 path
		path := s.joinPathSegments(response.Devices)

		// ListTagsByDevice 改成返回 map[string]map[string]string
		wrapper, err := s.ListTagsByDevice(path)
		if err != nil {
			log.Printf("批量查询设备标签失败: %v", err)
		} else {
			// 按 response.Devices 原始顺序遍历
			for _, dn := range response.Devices {
				if info, ok := wrapper[dn]; ok && info != nil {
					// 复制一份 map，避免修改底层
					m := make(map[string]string, len(info)+1)
					for k, v := range info {
						m[k] = v
					}
					m["dn"] = dn

					// 按 productId 判定是否加入
					if m["productId"] == "" {
						deviceInfoList = append(deviceInfoList, m)
					}
				} else {
					log.Printf("设备 %s 在返回结果中未找到或为空", dn)
				}
			}
		}
	}
	return deviceInfoList, nil
}

// DeleteDevices 删除指定设备
func (s *TagService) DeleteDevices(deviceNames ...string) error {
	if len(deviceNames) == 0 {
		return fmt.Errorf("未指定要删除的设备")
	}

	url := fmt.Sprintf("http://%s/DELDEVICE/%s",
		IoTPServer, strings.Join(deviceNames, ","))

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

// DevicesTagsTree 获取拥有指定标签值的设备树 V1
func (s *TagService) DevicesTagsTree(tagName, tagValue string) ([]map[string]interface{}, error) {
	// 根据标签筛选设备
	devices, err := s.ListDevicesByTag(tagName, tagValue)
	if err != nil {
		return nil, err
	}

	// 返回设备详细信息（带标签，树形结构）
	var deviceTree []map[string]interface{}

	for _, device := range devices {
		// 查询设备的所有标签
		tags, err := GetRawDeviceTags(device)
		if err != nil {
			log.Printf("获取设备 %s 标签失败: %v", device, err)
			continue
		}

		// 子节点（tags）
		var children []map[string]interface{}
		for _, v := range tags {
			children = append(children, map[string]interface{}{
				"name":  v["name"], // 标签名
				"type":  v["type"], // 标签类型
				"tag":   device + "." + v["name"],
				"isTag": true,
			})
		}

		// 根节点（设备）
		deviceNode := map[string]interface{}{
			"name":     device,   // 设备名
			"isDevice": true,     // 标识这是设备节点
			"children": children, // 子节点为标签
		}

		deviceTree = append(deviceTree, deviceNode)
	}

	return deviceTree, nil
}

// DevicesTagsTree2 获取拥有指定标签值的设备树 V2
func (s *TagService) DevicesTagsTree2(tagName, tagValue string) ([]map[string]interface{}, error) {
	// 根据标签筛选设备
	devices, err := s.ListDevicesByTag(tagName, tagValue)
	if err != nil {
		return nil, err
	}

	// 返回设备详细信息（带标签，树形结构）
	var deviceTree []map[string]interface{}

	// 根据产品返回统一模型
	o := orm.NewOrm()
	var tags []*models.Properties
	productIdInt, _ := strconv.ParseInt(tagValue, 10, 64)
	_, err = o.QueryTable(new(models.Properties)).
		Filter("Product__Id", productIdInt).
		All(&tags)

	for _, device := range devices {
		// 子节点（tags）
		var children []map[string]interface{}
		for _, v := range tags {
			var spec map[string]string
			err = json.Unmarshal([]byte(v.TypeSpec), &spec)
			if err != nil {
				v.Type = ""
			}
			children = append(children, map[string]interface{}{
				"name":  v.Name,       // 标签名
				"type":  spec["type"], // 标签类型
				"tag":   device + "." + v.Code,
				"isTag": true,
			})
		}

		// 根节点（设备）
		deviceNode := map[string]interface{}{
			"name":     device,   // 设备名
			"isDevice": true,     // 标识这是设备节点
			"children": children, // 子节点为标签
		}

		deviceTree = append(deviceTree, deviceNode)
	}

	return deviceTree, nil
}

// 获取设备的所有标签
func (s *TagService) ListTagsByDevice(path string) (map[string]map[string]string, error) {
	// 将路径分割为设备列表
	devices := strings.Split(path, "/")

	// 设置每个批次的最大设备数
	batchSize := 150

	// 初始化结果map
	result := make(map[string]map[string]string)

	// 分批处理设备列表
	for i := 0; i < len(devices); i += batchSize {
		// 计算当前批次的结束索引
		end := i + batchSize
		if end > len(devices) {
			end = len(devices)
		}

		// 获取当前批次的设备
		batchDevices := devices[i:end]

		// 构造当前批次的路径
		batchPath := strings.Join(batchDevices, "/")

		// 查询当前批次的设备标签
		batchResult, err := s.listTagsByDeviceBatch(batchPath)
		if err != nil {
			log.Printf("查询设备标签失败: %v", err)
			return nil, fmt.Errorf("API请求失败: %v", err)
		}

		// 合并结果
		for device, tags := range batchResult {
			result[device] = tags
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("未查询到设备标签")
	}

	return result, nil
}

// 批量查询设备标签的辅助方法
func (s *TagService) listTagsByDeviceBatch(path string) (map[string]map[string]string, error) {
	url := fmt.Sprintf("http://%s/tag/listTags/%s", IoTPServer, path)

	data, err := HttpGet(url)
	if err != nil {
		return nil, err
	}

	// 通用结构：map[设备名]map[string]string
	var wrapper map[string]map[string]string
	if err := json.Unmarshal([]byte(data), &wrapper); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	return wrapper, nil
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
