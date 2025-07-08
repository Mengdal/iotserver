package iotp

import (
	"encoding/json"
	"fmt"
	"time"
)

func GetRealData(device2tag map[string][]string) ([]Record, error) {
	var records []Record
	url := "http://" + IoTPServer + "/TSLASTQUERY"
	tagMsg := ""
	for deviceCode, tags := range device2tag {
		if tagMsg != "" {
			tagMsg += "/DEVICE"
		}
		tagMsg += "/" + deviceCode
		for _, tagCode := range tags {
			tagMsg += "/" + tagCode
		}
	}
	url += tagMsg

	fmt.Println(url)
	data, err := HttpGet(url)
	if err != nil {
		return nil, err
	}
	var realData map[string]map[string]interface{}
	err = json.Unmarshal([]byte(data), &realData)
	if err != nil {
		return nil, err
	}

	timestamp := time.Now().Unix()
	for deviceCode, tagData := range realData {
		for tagCode, val := range tagData {
			records = append(records, Record{
				Id:        deviceCode + "." + tagCode,
				Status:    "Good",
				Val:       val,
				Timestamp: timestamp,
			})
		}
	}
	return records, nil
}
