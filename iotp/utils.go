package iotp

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

func HttpGet(url string) (string, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	body, _ := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	return string(body), nil
}

func getTimestamp(timeStr string) (int64, error) {
	// 加载时区
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		// 时区加载失败时使用固定时区（东八区）
		loc = time.FixedZone("CST", 8*60*60)
	}

	cstTime, err := time.ParseInLocation("2006-01-02 15:04:05", timeStr, loc)
	if err != nil {
		fmt.Println("解析CST时间字符串出错：", err)
		return 0, err
	}

	// 将UTC时间转换为时间戳（毫秒）
	timestamp := cstTime.Unix()
	return timestamp, nil
}

func timestampFormat(timestamp float64) string {
	// 将毫秒时间戳转换为时间对象
	t := time.Unix(int64(timestamp), 0)
	// 格式化时间对象为指定的时间字符串
	timeStr := t.Format("2006-01-02 15:04:05")
	return timeStr
}
