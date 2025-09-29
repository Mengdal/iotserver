package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	beego "github.com/beego/beego/v2/server/web"
	"io"
	"io/ioutil"
	"iotServer/models/dtos"
	"log"
	"net/http"
	"time"
)

// EkuiperClient eKuiper客户端
type EkuiperClient struct {
	baseURL string
	client  *http.Client
}

// CreateRule 创建规则
type CreateRule struct {
	Triggered bool      `json:"triggered"`
	Id        string    `json:"id"`
	Name      string    `json:"name"`
	Sql       string    `json:"sql"`
	Actions   []Actions `json:"actions"`
}

// Actions 动作配置
type Actions struct {
	Rest map[string]interface{} `json:"rest,omitempty"`
}

// NewEkuiperClient 创建eKuiper客户端
func NewEkuiperClient(baseURL string) *EkuiperClient {
	return &EkuiperClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func InitEuiper() {
	var mqttURL, _ = beego.AppConfig.String("mqttServer")
	var ClientId, _ = beego.AppConfig.String("ClientId")
	var Username, _ = beego.AppConfig.String("Username")
	var Password, _ = beego.AppConfig.String("Password")
	var stream = "stream"

	// 1. 配置MQTT连接
	connUrl := fmt.Sprintf("%s/metadata/sources/mqtt/confKeys/init", EkuiperServer)
	connReq := map[string]interface{}{
		"qos":                0,
		"server":             "tcp://" + mqttURL,
		"username":           Username,
		"password":           Password,
		"clientid":           ClientId,
		"protocolVersion":    "3.1.1",
		"insecureSkipVerify": false,
	}

	b, _ := json.Marshal(connReq)
	req, err := http.NewRequest("PUT", connUrl, bytes.NewReader(b))
	if err != nil {
		log.Fatalf("创建 MQTT 连接请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("配置 MQTT 连接失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("Ekuiper 配置 MQTT 连接失败: %s", string(body))
	} else {
		log.Println("EKuiper MQTT 连接配置成功")
	}

	// 2. 先GET判断流是否存在 默认创建全局stream流

	url := fmt.Sprintf("%s/streams/%s", EkuiperServer, stream)
	resp, err = http.Get(url)
	if err != nil {
		log.Fatalln("初始化ekuiper失败，查询服务是否启动")
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		log.Println("流文件已存在，无需创建")
		return
	}

	if resp.StatusCode != 400 {
		log.Fatalln("初始化ekuiper失败，查询服务是否正常")
	}

	// 1. 不存在则创建  实时数据主题 edge/property/{SN}/post
	createUrl := fmt.Sprintf("%s/streams", EkuiperServer)
	sql := fmt.Sprintf(`
        CREATE STREAM %s ()
        WITH (
            DATASOURCE="/edge/stream/+/post",
            FORMAT="JSON",
            SHARED="true",
            TYPE="mqtt",
            CONF_KEY="init"
        )
    `, stream)
	reqBody := map[string]string{"sql": sql}
	b, _ = json.Marshal(reqBody)
	resp2, err := http.Post(createUrl, "application/json", bytes.NewReader(b))
	if err != nil {
		log.Fatalf("create stream failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 201 {
		body, _ := ioutil.ReadAll(resp2.Body)
		log.Printf("create stream failed: %s", string(body))
	}
}

// RuleExist 检查规则是否存在，如果不存在或发生错误返回 error，否则返回 nil
func (c *EkuiperClient) RuleExist(ctx context.Context, ruleId string) error {
	url := fmt.Sprintf("%s/rules/%s", c.baseURL, ruleId)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// 尝试解析 JSON
	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err == nil {
		// 如果有 error 字段且是 1002，则认为规则不存在
		if code, ok := result["error"].(float64); ok && code == 1002 {
			return fmt.Errorf("rule not found")
		}
		// 其他错误信息也返回
		if msg, ok := result["message"].(string); ok {
			return fmt.Errorf("server error: %s", msg)
		}
	}

	// 如果状态码是 404 也认为不存在
	if resp.StatusCode == 404 {
		return fmt.Errorf("rule not found")
	}

	// 只有 HTTP 200 才认为规则存在
	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// 成功找到规则，返回 nil
	return nil
}

// CreateRule 创建规则
func (c *EkuiperClient) CreateRule(ctx context.Context, actions []Actions, ruleId string, sql string) error {
	url := fmt.Sprintf("%s/rules", c.baseURL)

	createRule := CreateRule{
		Triggered: false,
		Id:        ruleId,
		Name:      ruleId,
		Sql:       sql,
		Actions:   actions,
	}

	body, err := json.Marshal(createRule)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 201 {
		return nil
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("create rule failed: %s", string(bodyBytes))
}

// UpdateRule 更新规则
func (c *EkuiperClient) UpdateRule(ctx context.Context, actions []Actions, ruleId string, sql string) error {
	url := fmt.Sprintf("%s/rules/%s", c.baseURL, ruleId)

	createRule := CreateRule{
		Triggered: false,
		Id:        ruleId,
		Name:      ruleId,
		Sql:       sql,
		Actions:   actions,
	}

	body, err := json.Marshal(createRule)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return nil
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("update rule failed: %s", string(bodyBytes))
}

// DeleteRule 删除规则
func (c *EkuiperClient) DeleteRule(ctx context.Context, ruleId string) error {
	url := fmt.Sprintf("%s/rules/%s", c.baseURL, ruleId)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return nil
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("delete rule failed: %s", string(bodyBytes))
}

// StartRule 启动规则
func (c *EkuiperClient) StartRule(ctx context.Context, ruleId string) error {
	url := fmt.Sprintf("%s/rules/%s/start", c.baseURL, ruleId)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return nil
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("start rule failed: %s", string(bodyBytes))
}

// StopRule 停止规则
func (c *EkuiperClient) StopRule(ctx context.Context, ruleId string) error {
	url := fmt.Sprintf("%s/rules/%s/stop", c.baseURL, ruleId)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return nil
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("stop rule failed: %s", string(bodyBytes))
}

// RestartRule 重启规则
func (c *EkuiperClient) RestartRule(ctx context.Context, ruleId string) error {
	url := fmt.Sprintf("%s/rules/%s/restart", c.baseURL, ruleId)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return nil
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("restart rule failed: %s", string(bodyBytes))
}

// GetAllRules 获取所有规则摘要信息
func (c *EkuiperClient) GetAllRules(ctx context.Context) ([]dtos.RuleResponse, error) {
	url := fmt.Sprintf("%s/rules", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("获取规则列表失败: %s", string(bodyBytes))
	}

	var ruleList []dtos.RuleResponse
	if err := json.Unmarshal(bodyBytes, &ruleList); err != nil {
		return nil, fmt.Errorf("解析规则列表失败: %v", err)
	}

	return ruleList, nil
}

// GetRule 获取指定规则的详细信息
func (c *EkuiperClient) GetRule(ctx context.Context, ruleId string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/rules/%s", c.baseURL, ruleId)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("描述规则失败: %s", string(bodyBytes))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("解析规则详情失败: %v", err)
	}

	return result, nil
}

// GetRuleStats 获取规则状态
func (c *EkuiperClient) GetRuleStats(ctx context.Context, ruleId string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/rules/%s/status", c.baseURL, ruleId)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var result map[string]interface{}
		bodyBytes, _ := io.ReadAll(resp.Body)
		err = json.Unmarshal(bodyBytes, &result)
		return result, err
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	return nil, fmt.Errorf("get rule stats failed: %s", string(bodyBytes))
}

// GetAllRuleStats 获取所有规则状态
func (c *EkuiperClient) GetAllRuleStats(ctx context.Context) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/rules/status/all", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var result map[string]interface{}
		bodyBytes, _ := io.ReadAll(resp.Body)
		err = json.Unmarshal(bodyBytes, &result)
		return result, err
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	return nil, fmt.Errorf("get rule stats failed: %s", string(bodyBytes))
}

// GetRuleAlertEkuiperActions 获取告警规则动作
func GetRuleAlertEkuiperActions(actionUrl string) []Actions {
	rest := map[string]interface{}{
		"method":      "POST",
		"url":         actionUrl,
		"bodyType":    "json",
		"timeout":     5000,
		"runAsync":    false,
		"omitIfEmpty": false,
		"sendSingle":  true,
		"enableCache": false,
		"format":      "json",
	}

	return []Actions{
		{Rest: rest},
	}
}
