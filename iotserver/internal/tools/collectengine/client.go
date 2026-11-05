// Package collectengine 封装采集引擎（IOTP Gateway API）的通道 / IOTP 设备绑定 / 点位接口。
//
// 接口文档见 /swagger/index.html：GET 返回 JSON，POST/PUT/DELETE 使用 multipart/form-data。
package collectengine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Channel 采集通道。
type Channel struct {
	Id           int64  `json:"id"`
	ChannelCode  string `json:"channelCode"`
	Protocol     string `json:"protocol"`
	ComInterval  int64  `json:"comInterval"`
	PollInterval int64  `json:"pollInterval"`
	RetryCount   int64  `json:"retryCount"`
	Timeout      int64  `json:"timeout"`
	Advance      string `json:"advance"`
	Disable      bool   `json:"disable"`
}

// Device 通道下的设备；id 是引擎自增设备 ID，iotpId 对应平台设备。
type Device struct {
	Id        int64 `json:"id"`
	ChannelId int64 `json:"channelId"`
	IotpId    int64 `json:"iotpId"`
}

// Tag 设备点位；deviceId 是引擎自增设备 ID。
type Tag struct {
	Id          int64  `json:"id"`
	DeviceId    int64  `json:"deviceId"`
	TagCode     string `json:"tagCode"`
	Address     string `json:"address"`
	DataType    string `json:"dataType"`
	ReadWrite   string `json:"readWrite"`
	ScaleOffset string `json:"scaleOffset"`
	Storage     bool   `json:"storage"`
}

// TagRealtimeRecord 点位实时值（采集引擎内存中的最新回调记录）。
type TagRealtimeRecord struct {
	// Id 运行点位 ID：iotpId.tagCode
	Id        string `json:"id"`
	Status    any    `json:"status"`
	Value     string `json:"value"`
	Timestamp int64  `json:"timestamp"`
}

// TagCode 去掉 iotpId 前缀后的标签编码。
func (r TagRealtimeRecord) TagCode() string {
	if index := strings.Index(r.Id, "."); index >= 0 {
		return r.Id[index+1:]
	}
	return r.Id
}

// StatusText 统一状态：192/Good、0/Error，其他为 Uncertain。
func (r TagRealtimeRecord) StatusText() string {
	switch value := r.Status.(type) {
	case float64:
		switch int(value) {
		case 192:
			return "Good"
		case 0:
			return "Error"
		}
	case string:
		switch value {
		case "192", "Good":
			return "Good"
		case "0", "Error":
			return "Error"
		}
		if value != "" {
			return value
		}
	}
	return "Uncertain"
}

// TagRealtimePage 点位实时数据分页。
type TagRealtimePage struct {
	Items    []TagRealtimeRecord `json:"items"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"pageSize"`
}

// APIError 采集引擎返回的业务错误。
type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("采集引擎返回 %d: %s", e.Code, e.Message)
}

// Client 采集引擎 HTTP 客户端。
type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		http:    &http.Client{Timeout: timeout},
	}
}

/******************************* 通道 *******************************/

func (c *Client) ListChannels() ([]Channel, error) {
	var result []Channel
	err := c.get("/api/v1/channels", &result)
	return result, err
}

func (c *Client) CreateChannel(channel Channel) (Channel, error) {
	var result Channel
	form := channelForm(channel)
	err := c.postForm("/api/v1/channels", form, &result)
	return result, err
}

func (c *Client) UpdateChannel(id int64, channel Channel) (Channel, error) {
	var result Channel
	form := channelForm(channel)
	err := c.putForm(fmt.Sprintf("/api/v1/channels/%d", id), form, &result)
	return result, err
}

func (c *Client) DeleteChannel(id int64) error {
	return c.delete(fmt.Sprintf("/api/v1/channels/%d", id))
}

func channelForm(channel Channel) map[string]string {
	return map[string]string{
		"channelCode":  channel.ChannelCode,
		"protocol":     channel.Protocol,
		"comInterval":  strconv.FormatInt(channel.ComInterval, 10),
		"pollInterval": strconv.FormatInt(channel.PollInterval, 10),
		"retryCount":   strconv.FormatInt(channel.RetryCount, 10),
		"timeout":      strconv.FormatInt(channel.Timeout, 10),
		"advance":      channel.Advance,
		"disable":      strconv.FormatBool(channel.Disable),
	}
}

/******************************* 设备 *******************************/

// ListDevices 查询设备；channelId<=0 时全量查询。
func (c *Client) ListDevices(channelId int64) ([]Device, error) {
	path := "/api/v1/devices"
	if channelId > 0 {
		path += "?channelId=" + strconv.FormatInt(channelId, 10)
	}
	var result []Device
	err := c.get(path, &result)
	return result, err
}

func (c *Client) CreateDevice(channelId, iotpId int64) (Device, error) {
	var result Device
	form := map[string]string{
		"channelId": strconv.FormatInt(channelId, 10),
		"iotpId":    strconv.FormatInt(iotpId, 10),
	}
	err := c.postForm("/api/v1/devices", form, &result)
	return result, err
}

func (c *Client) DeleteDevice(id int64) error {
	return c.delete(fmt.Sprintf("/api/v1/devices/%d", id))
}

/******************************* 点位 *******************************/

// RealtimeTags 查询某台引擎设备下所有点位的实时值。
func (c *Client) RealtimeTags(deviceID int64) (TagRealtimePage, error) {
	return c.RealtimeTagsPage(deviceID, 1, 1000)
}

// WriteTag 向点位写入值；deviceID 是引擎设备自增 ID。
func (c *Client) WriteTag(deviceID int64, tagCode, value string) error {
	return c.postForm("/api/v1/tags/write", map[string]string{
		"deviceId": strconv.FormatInt(deviceID, 10),
		"tagCode":  tagCode,
		"value":    value,
	}, nil)
}

// RealtimeTagsPage 分页查询点位实时值；deviceID<=0 时查询全部。
func (c *Client) RealtimeTagsPage(deviceID int64, page, pageSize int) (TagRealtimePage, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 1000 {
		pageSize = 1000
	}
	path := fmt.Sprintf("/api/v1/tags/realtime?page=%d&pageSize=%d", page, pageSize)
	if deviceID > 0 {
		path += fmt.Sprintf("&deviceId=%d", deviceID)
	}
	var result TagRealtimePage
	err := c.get(path, &result)
	return result, err
}

func (c *Client) ListTags() ([]Tag, error) {
	var result []Tag
	err := c.get("/api/v1/tags", &result)
	return result, err
}

func (c *Client) CreateTag(tag Tag) (Tag, error) {
	var result Tag
	err := c.postForm("/api/v1/tags", tagForm(tag), &result)
	return result, err
}

func (c *Client) UpdateTag(id int64, tag Tag) (Tag, error) {
	var result Tag
	err := c.putForm(fmt.Sprintf("/api/v1/tags/%d", id), tagForm(tag), &result)
	return result, err
}

func (c *Client) DeleteTag(id int64) error {
	return c.delete(fmt.Sprintf("/api/v1/tags/%d", id))
}

func tagForm(tag Tag) map[string]string {
	return map[string]string{
		"deviceId":    strconv.FormatInt(tag.DeviceId, 10),
		"tagCode":     tag.TagCode,
		"address":     tag.Address,
		"dataType":    tag.DataType,
		"readWrite":   tag.ReadWrite,
		"scaleOffset": tag.ScaleOffset,
		"storage":     strconv.FormatBool(tag.Storage),
	}
}

/******************************* HTTP *******************************/

type apiResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (c *Client) get(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, c.url(path), nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) postForm(path string, form map[string]string, out any) error {
	return c.formRequest(http.MethodPost, path, form, out)
}

func (c *Client) putForm(path string, form map[string]string, out any) error {
	return c.formRequest(http.MethodPut, path, form, out)
}

func (c *Client) formRequest(method, path string, form map[string]string, out any) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range form {
		if err := writer.WriteField(key, value); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	req, err := http.NewRequest(method, c.url(path), &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return c.do(req, out)
}

func (c *Client) delete(path string) error {
	req, err := http.NewRequest(http.MethodDelete, c.url(path), nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("请求采集引擎失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	var wrapper apiResponse
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return fmt.Errorf("采集引擎响应不是合法 JSON（http %d）: %s", resp.StatusCode, truncate(string(body), 200))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{Code: resp.StatusCode, Message: firstNonEmpty(wrapper.Message, resp.Status)}
	}
	if wrapper.Code != 0 && wrapper.Code != http.StatusOK {
		return &APIError{Code: wrapper.Code, Message: wrapper.Message}
	}
	if out != nil && len(wrapper.Data) > 0 && string(wrapper.Data) != "null" {
		if err := json.Unmarshal(wrapper.Data, out); err != nil {
			return fmt.Errorf("解析采集引擎响应失败: %w", err)
		}
	}
	return nil
}

func (c *Client) url(path string) string {
	return c.baseURL + path
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
