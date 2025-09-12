package iotp

import (
	"encoding/json"
	"errors"
	"fmt"
	beego "github.com/beego/beego/v2/server/web"
	"iotServer/models"
	"strings"
)

var IoTPServer = "127.0.0.1"

func init() {
	IoTPServer, _ = beego.AppConfig.String("iotpServer")
}

func HistoryQuery(o models.HistoryObject) (map[string][]interface{}, error) {
	startTimestamp, err1 := GetTimestamp(o.StartTime)
	endTimestamp, err2 := GetTimestamp(o.EndTime)
	if err1 != nil || err2 != nil {
		return nil, errors.New("传输的时间格式出错：yyyy-mm-dd HH:mm:ss")
	}
	tagMsg := ""
	devices := make(map[string][]string)
	for _, tagId := range o.IDs {
		l := strings.Split(tagId, ".")
		if len(l) != 2 {
			fmt.Println("deviceCode或tagCode中不允许有.")
			continue
		}
		deviceCode := l[0]
		tagCode := l[1]
		if _, ok := devices[deviceCode]; !ok {
			devices[deviceCode] = make([]string, 0)
		}
		devices[deviceCode] = append(devices[deviceCode], tagCode)
	}

	for deviceCode, tags := range devices {
		if tagMsg != "" {
			tagMsg += "/DEVICE"
		}
		tagMsg += "/" + deviceCode
		for _, tagCode := range tags {
			tagMsg += "/" + tagCode
		}
	}
	url := fmt.Sprintf("http://%s/TSQUERY/%d/%d/COUNT/%d%s", IoTPServer, startTimestamp, endTimestamp, o.Count, tagMsg)

	fmt.Println(url)
	data, err := HttpGet(url)
	if err != nil {
		return nil, err
	}
	var queryData map[string][]HistoryV2QueryData
	err = json.Unmarshal([]byte(data), &queryData)
	if err != nil {
		return nil, err
	}
	result := make(map[string][]interface{})
	for deviceCode, dData := range queryData {
		for _, v := range dData {
			timestamp := v.Timestamp
			for tagCode, tagVal := range v.Fields {
				tagId := deviceCode + "." + tagCode
				record := GoRecord{
					Status:    "Good",
					Val:       "0",
					Timestamp: timestamp,
				}
				switch tagVal.(type) {
				case string:
					valStr := tagVal.(string)
					if valStr == "none" {
						record.Status = "Error"
					} else {
						record.Val = valStr
					}
				case float64:
					record.Val = fmt.Sprintf("%.3f", tagVal)
				}
				if _, ok := result[tagId]; !ok {
					result[tagId] = make([]interface{}, 0)
				}
				result[tagId] = append(result[tagId], record)
			}
		}
	}
	return result, nil
}

func AggQuery(objects models.AggObject) (map[string][]interface{}, error) {
	startTimestamp, err1 := GetTimestamp(objects.StartTime)
	endTimestamp, err2 := GetTimestamp(objects.EndTime)
	if err1 != nil || err2 != nil {
		return nil, errors.New("传输的时间格式出错：yyyy-mm-dd HH:mm:ss")
	}
	if objects.Count == 0 && objects.Period == 0 {
		return nil, errors.New("period必填")
	}
	var cycle int64
	if objects.Period != 0 {
		if objects.Period < 60 {
			return nil, errors.New("period必须大于60")
		}
		cycle = objects.Period
	} else {
		cycle = (endTimestamp - startTimestamp) / objects.Count
		if cycle < 60 {
			cycle = 60
		}
	}

	tagMsg := ""
	devices := make(map[string][]string)
	for _, tagId := range objects.IDs {
		l := strings.Split(tagId, ".")
		if len(l) != 2 {
			fmt.Println("deviceCode或tagCode中不允许有.")
			continue
		}
		deviceCode := l[0]
		tagCode := l[1]
		if _, ok := devices[deviceCode]; !ok {
			devices[deviceCode] = make([]string, 0)
		}
		devices[deviceCode] = append(devices[deviceCode], tagCode)
	}

	for deviceCode, tags := range devices {
		if tagMsg != "" {
			tagMsg += "/DEVICE"
		}
		tagMsg += "/" + deviceCode
		for _, tagCode := range tags {
			tagMsg += "/" + tagCode
			tagMsg += fmt.Sprintf("/%s(%s,%d,%d)", objects.AggType, tagCode, cycle, cycle)
		}
	}
	url := fmt.Sprintf("http://%s/TSAGGQUERY/%d/%d%s", IoTPServer, startTimestamp, endTimestamp, tagMsg)
	if objects.Desc {
		url = fmt.Sprintf("http://%s/TSREVAGGQUERY/%d/%d%s", IoTPServer, endTimestamp, startTimestamp, tagMsg)

	}
	fmt.Println(url)
	data, err := HttpGet(url)
	if err != nil {
		return nil, err
	}
	//fmt.Println(data)
	var queryData map[string][]HistoryV2QueryData
	err = json.Unmarshal([]byte(data), &queryData)
	if err != nil {
		return nil, err
	}
	aggData, err := parseV2Data(queryData, objects.AggType)
	return aggData, err
}

func parseV2Data(queryData map[string][]HistoryV2QueryData, aggType string) (map[string][]interface{}, error) {
	result := make(map[string][]interface{})
	for deviceCode, deviceData := range queryData {
		for _, v := range deviceData {
			timestamp := v.Timestamp
			for tagAggType, tagVal := range v.Fields {
				end := strings.Index(tagAggType, ",")
				tagID := tagAggType //普通查询
				if aggType != "" {
					tagID = deviceCode + "." + tagAggType[len(aggType)+1:end]
				}
				record := GoRecord{
					Status:    "Good",
					Val:       "0",
					Timestamp: timestamp,
				}
				switch tagVal.(type) {
				case string:
					valStr := tagVal.(string)
					if valStr == "none" {
						record.Val = "Error"
					} else {
						record.Val = valStr
					}
				case float64:
					record.Val = fmt.Sprintf("%.3f", tagVal)
				}
				result[tagID] = append(result[tagID], record)
			}
		}
	}
	return result, nil
}

func DiffQuery(o models.DiffObject) (map[string][]interface{}, error) {
	startTimestamp, err1 := GetTimestamp(o.StartTime)
	endTimestamp, err2 := GetTimestamp(o.EndTime)
	if err1 != nil || err2 != nil {
		return nil, errors.New("传输的时间格式出错：yyyy-mm-dd HH:mm:ss")
	}
	if o.Period == 0 {
		return nil, errors.New("周期不能为0")
	}
	tagMsg := ""
	devices := make(map[string][]string)
	for _, tagId := range o.IDs {
		l := strings.Split(tagId, ".")
		if len(l) != 2 {
			fmt.Println("deviceCode或tagCode中不允许有.")
			continue
		}
		deviceCode := l[0]
		tagCode := l[1]
		if _, ok := devices[deviceCode]; !ok {
			devices[deviceCode] = make([]string, 0)
		}
		devices[deviceCode] = append(devices[deviceCode], tagCode)
	}

	for deviceCode, tags := range devices {
		if tagMsg != "" {
			tagMsg += "/DEVICE"
		}
		tagMsg += "/" + deviceCode
		for _, tagCode := range tags {
			tagMsg += "/" + tagCode
		}
	}
	url := fmt.Sprintf("http://%s/TSDIFFQUERY/%d/%d/%d%s", IoTPServer, o.Period, startTimestamp, endTimestamp, tagMsg)

	fmt.Println(url)
	data, err := HttpGet(url)
	if err != nil {
		return nil, err
	}
	var queryData map[string][]map[string]interface{}
	err = json.Unmarshal([]byte(data), &queryData)
	if err != nil {
		return nil, err
	}
	result := make(map[string][]interface{})
	for deviceCode, dData := range queryData {
		for _, v := range dData {
			timestamp := v["timestamp"].(float64)
			for tagCode, tagVal := range v {
				if tagCode == "timestamp" {
					continue
				}
				tagId := deviceCode + "." + tagCode
				record := GoRecord{
					Status:    "Good",
					Val:       "0",
					Timestamp: timestamp,
				}
				switch tagVal.(type) {
				case string:
					valStr := tagVal.(string)
					if valStr == "none" {
						record.Status = "Error"
					} else {
						record.Val = valStr
					}
				case float64:
					record.Val = fmt.Sprintf("%.3f", tagVal)
				}
				if _, ok := result[tagId]; !ok {
					result[tagId] = make([]interface{}, 0)
				}
				result[tagId] = append(result[tagId], record)
			}
		}
	}
	return result, nil
}

func BoolQuery(o models.BoolObject) (map[string]interface{}, error) {
	startTimestamp, err1 := GetTimestamp(o.StartTime)
	endTimestamp, err2 := GetTimestamp(o.EndTime)
	if err1 != nil || err2 != nil {
		return nil, errors.New("传输的时间格式出错：yyyy-mm-dd HH:mm:ss")
	}
	tagMsg := ""
	devices := make(map[string][]string)
	for _, tagId := range o.IDs {
		l := strings.Split(tagId, ".")
		if len(l) != 2 {
			fmt.Println("deviceCode或tagCode中不允许有.")
			continue
		}
		deviceCode := l[0]
		tagCode := l[1]
		if _, ok := devices[deviceCode]; !ok {
			devices[deviceCode] = make([]string, 0)
		}
		devices[deviceCode] = append(devices[deviceCode], tagCode)
	}

	for deviceCode, tags := range devices {
		if tagMsg != "" {
			tagMsg += "/DEVICE"
		}
		tagMsg += "/" + deviceCode
		for _, tagCode := range tags {
			tagMsg += "/" + tagCode
		}
	}
	url := fmt.Sprintf("http://%s/TSBOOLQUERY/%d/%d%s", IoTPServer, startTimestamp, endTimestamp, tagMsg)

	fmt.Println(url)
	data, err := HttpGet(url)
	if err != nil {
		return nil, err
	}
	var queryData map[string]map[string]interface{}
	err = json.Unmarshal([]byte(data), &queryData)
	if err != nil {
		return nil, err
	}
	result := make(map[string]interface{})
	for deviceCode, dData := range queryData {
		for tagCode, v := range dData {
			tagId := deviceCode + "." + tagCode
			result[tagId] = v
		}
	}
	return result, nil
}
