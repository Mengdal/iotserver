package iotp

import (
	"encoding/json"
	"fmt"
	"sort"
)

var TagId2Desc = map[string]string{
	"ISE_Act_1":     "VE/SE气源压力",
	"ITV_Act_1":     "VE/SE出气压力",
	"PF_RateFlow_1": "VE/SE瞬时流量",
	"SP_Run_1":      "VE/SE全压压力设定值",
	"SP_Standy_1":   "VE/SE降压压力设定值",
	"ISE_Act_2":     "2#气路气源压力",
	"ITV_Act_2":     "2#气路出气压力",
	"PF_RateFlow_2": "2#气路瞬时流量",
	"SP_Run_2":      "2#气路全压压力设定值",
	"SP_Standy_2":   "2#气路降压压力设定值",
	"ISE_Act_3":     "MAX气源压力",
	"ITV_Act_3":     "MAX出气压力",
	"PF_RateFlow_3": "MAX瞬时流量",
	"SP_Run_3":      "MAX全压压力设定值",
	"SP_Standy_3":   "MAX降压压力设定值",
	"AMS_I_EMG":     "CMS设备急停信号",
	"Run":           "CMS设备全压信号 ",
	"ShutDown":      "CMS设备关断信号",
	"Standby":       "CMS设备降压信号",
	"Manual":        "CMS设备运行模式反馈信号",
	"AccFlow_1":     "VE/SE累计流量",
	"AccFlow_2":     "2#气路通过流量累计值",
	"AccFlow_3":     "MAX累计流量",
	"AccFlow":       "总累计流量",
	"AMS_RateFlow":  "总瞬时流量",
	"STW_Tob":       "烟机运行状态字",
}

func GetTagsTree() ([]DeviceMsg, error) {
	var devices []DeviceMsg
	url := "http://" + IoTPServer + "/ALLDEVICESFIELDS"
	fmt.Println(url)
	data, err := HttpGet(url)
	if err != nil {
		return nil, err
	}
	var device2tag map[string][]TagMsg
	err = json.Unmarshal([]byte(data), &device2tag)
	if err != nil {
		return nil, err
	}

	deviceCodes := make([]string, 0)
	for deviceCode := range device2tag {
		deviceCodes = append(deviceCodes, deviceCode)
	}
	sort.Strings(deviceCodes)

	for _, deviceCode := range deviceCodes {
		tagList := device2tag[deviceCode]
		deviceMsg := DeviceMsg{Name: deviceCode}
		for i, tag := range tagList {
			tagList[i].Id = deviceCode + "." + tag.Name
			if desc, ok := TagId2Desc[tag.Name]; ok {
				tagList[i].Description = desc
			}
		}
		deviceMsg.Children = tagList
		devices = append(devices, deviceMsg)
	}
	return devices, nil
}

func GetDeviceTags(dn string) ([]string, error) {
	url := "http://" + IoTPServer + "/ALLFIELDS/" + dn
	fmt.Println("请求URL:", url)

	data, err := HttpGet(url)
	if err != nil {
		return nil, err
	}

	// 定义结构用于解析JSON
	var tags []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}

	if err := json.Unmarshal([]byte(data), &tags); err != nil {
		return nil, fmt.Errorf("解析设备标签失败: %v", err)
	}

	var result []string
	for _, tag := range tags {
		result = append(result, dn+"."+tag.Name)
	}

	return result, nil
}

func GetRawDeviceTags(dn string) ([]map[string]string, error) {
	url := "http://" + IoTPServer + "/ALLFIELDS/" + dn
	fmt.Println("请求URL:", url)

	data, err := HttpGet(url)
	if err != nil {
		return nil, err
	}

	// 定义结构用于解析JSON
	var tags []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}

	if err := json.Unmarshal([]byte(data), &tags); err != nil {
		return nil, fmt.Errorf("解析设备标签失败: %v", err)
	}

	var result []map[string]string
	for _, t := range tags {
		result = append(result, map[string]string{
			"name": t.Name,
			"type": t.Type,
		})
	}

	return result, nil
}
