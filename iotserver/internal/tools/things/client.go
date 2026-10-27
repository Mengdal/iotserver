package things

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	loginPath         = "/edge-user/user/apiLogin"
	projectListPath   = "/edge-user/project/list"
	projectDetailPath = "/edge-user/project/detail"
	projectCreatePath = "/edge-user/project/create"
	projectUpdatePath = "/edge-user/project/setting"
	projectDeletePath = "/edge-user/project/delete"
	gatewayListPath   = "/edge-mgmt/gateway/list"
	gatewayCreatePath = "/edge-mgmt/gateway/create"
	gatewayUpdatePath = "/edge-mgmt/gateway/modify"
	gatewayDeletePath = "/edge-mgmt/gateway/delete"

	// maxPageSize things 侧的 MAX_SIZE_OF_PAGE。
	maxPageSize = 999999
	// maxResponseBytes 防止异常响应把内存打满。things 网关列表不会接近这个量级。
	maxResponseBytes = 8 << 20
)

// Client things HTTP 客户端。
//
// 一个 Client 应作为进程级单例复用：token、登录并发控制都挂在实例上。
type Client struct {
	config ConfigProvider

	mu          sync.Mutex
	cachedToken string
	tokenAt     time.Time
	tokenKey    string

	loginMu sync.Mutex
}

// NewClient 创建 things 客户端。config 每次调用都会重新读取，支持运行时改配置。
func NewClient(config ConfigProvider) *Client {
	return &Client{config: config}
}

// Login 强制登录并刷新进程内 token，返回非敏感登录信息。
func (c *Client) Login(ctx context.Context) (LoginInfo, error) {
	cfg, err := c.loadConfig(ctx)
	if err != nil {
		return LoginInfo{}, err
	}
	token, data, err := c.login(ctx, cfg)
	if err != nil {
		return LoginInfo{}, err
	}
	var info struct {
		Token    string `json:"token"`
		UserID   int64  `json:"userId"`
		Username string `json:"username"`
		RoleID   int64  `json:"roleId"`
	}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &info)
	}
	return LoginInfo{
		Token:    token,
		UserID:   info.UserID,
		Username: info.Username,
		RoleID:   info.RoleID,
	}, nil
}

// Ping 验证配置与服务可用性，返回项目总数。
func (c *Client) Ping(ctx context.Context) (int64, error) {
	projects, total, err := c.ListProjects(ctx, 1, 1, "")
	if err != nil {
		return 0, err
	}
	if total == 0 && len(projects) > 0 {
		total = int64(len(projects))
	}
	return total, nil
}

// ListProjects 拉取项目列表。
func (c *Client) ListProjects(ctx context.Context, page, size int, name string) ([]Project, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > maxPageSize {
		size = maxPageSize
	}
	form := url.Values{}
	form.Set("page", strconv.Itoa(page))
	form.Set("size", strconv.Itoa(size))
	form.Set("lang", "ZH")
	if strings.TrimSpace(name) != "" {
		form.Set("name", strings.TrimSpace(name))
	}
	raw, err := c.doForm(ctx, projectListPath, form)
	if err != nil {
		return nil, 0, err
	}
	return decodeProjectList(raw)
}

// ProjectByID 查询项目详情。
func (c *Client) ProjectByID(ctx context.Context, projectID int64) (Project, error) {
	if projectID <= 0 {
		return Project{}, fmt.Errorf("project id is required")
	}
	form := url.Values{}
	form.Set("projectId", strconv.FormatInt(projectID, 10))
	raw, err := c.doForm(ctx, projectDetailPath, form)
	if err != nil {
		return Project{}, err
	}
	var p Project
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &p); err != nil {
			return Project{}, fmt.Errorf("decode things project: %w", err)
		}
	}
	return p, nil
}

// CreateProject 创建项目。
func (c *Client) CreateProject(ctx context.Context, name, description string) (Project, error) {
	form := url.Values{}
	form.Set("name", strings.TrimSpace(name))
	if strings.TrimSpace(description) != "" {
		form.Set("description", description)
	}
	raw, err := c.doForm(ctx, projectCreatePath, form)
	if err != nil {
		return Project{}, err
	}
	var p Project
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &p); err != nil {
			return Project{}, fmt.Errorf("decode things project: %w", err)
		}
	}
	if p.ID == 0 {
		return p, fmt.Errorf("things returned an empty project id")
	}
	return p, nil
}

// UpdateProject 修改项目名称/备注。
func (c *Client) UpdateProject(ctx context.Context, projectID int64, name, description string) (Project, error) {
	if projectID <= 0 {
		return Project{}, fmt.Errorf("project id is required")
	}
	form := url.Values{}
	form.Set("projectId", strconv.FormatInt(projectID, 10))
	if strings.TrimSpace(name) != "" {
		form.Set("name", strings.TrimSpace(name))
	}
	if strings.TrimSpace(description) != "" {
		form.Set("description", description)
	}
	raw, err := c.doForm(ctx, projectUpdatePath, form)
	if err != nil {
		return Project{}, err
	}
	var p Project
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &p); err != nil {
			return Project{}, fmt.Errorf("decode things project: %w", err)
		}
	}
	return p, nil
}

// DeleteProject 删除项目。pwd 是 things 服务账号密码。
func (c *Client) DeleteProject(ctx context.Context, projectID int64, password string) error {
	if projectID <= 0 {
		return fmt.Errorf("project id is required")
	}
	form := url.Values{}
	form.Set("projectId", strconv.FormatInt(projectID, 10))
	form.Set("pwd", password)
	_, err := c.doForm(ctx, projectDeletePath, form)
	return err
}

// ListGateways 拉取项目下全部网关。
//
// 返回的 Gateway 不包含 secret/license，即使 things 响应里带了也不会进入本地模型。
func (c *Client) ListGateways(ctx context.Context, projectID int64) ([]Gateway, error) {
	if projectID <= 0 {
		return nil, fmt.Errorf("project id is required")
	}
	form := url.Values{}
	form.Set("projectId", strconv.FormatInt(projectID, 10))
	form.Set("page", "1")
	form.Set("size", strconv.Itoa(maxPageSize))
	raw, err := c.doForm(ctx, gatewayListPath, form)
	if err != nil {
		return nil, err
	}
	return decodeGatewayList(raw)
}

// CreateGateway 创建网关。
func (c *Client) CreateGateway(ctx context.Context, req CreateGatewayRequest) (Gateway, error) {
	if req.ProjectID <= 0 {
		return Gateway{}, fmt.Errorf("project id is required")
	}
	form := url.Values{}
	form.Set("projectId", strconv.FormatInt(req.ProjectID, 10))
	form.Set("sn", strings.TrimSpace(req.SN))
	form.Set("secret", req.Secret)
	form.Set("name", strings.TrimSpace(req.Name))
	form.Set("model", strings.TrimSpace(req.Model))
	if strings.TrimSpace(req.LicenseType) != "" {
		form.Set("licenseType", strings.TrimSpace(req.LicenseType))
	}
	if strings.TrimSpace(req.License) != "" {
		form.Set("license", req.License)
	}
	raw, err := c.doForm(ctx, gatewayCreatePath, form)
	if err != nil {
		return Gateway{}, err
	}
	var g Gateway
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &g); err != nil {
			return Gateway{}, fmt.Errorf("decode things gateway: %w", err)
		}
	}
	if g.ID == 0 {
		return g, fmt.Errorf("things returned an empty gateway id")
	}
	if g.ProjectID == 0 {
		g.ProjectID = req.ProjectID
	}
	if g.SN == "" {
		g.SN = req.SN
	}
	if g.PK == "" {
		g.PK = g.SN
	}
	return g, nil
}

// UpdateGateway 修改网关名称/授权。只发送非空字段，未传的字段由 things 保持不变。
func (c *Client) UpdateGateway(ctx context.Context, req UpdateGatewayRequest) (Gateway, error) {
	if req.ProjectID <= 0 || req.GatewayID <= 0 {
		return Gateway{}, fmt.Errorf("project id and gateway id are required")
	}
	form := url.Values{}
	form.Set("projectId", strconv.FormatInt(req.ProjectID, 10))
	form.Set("gatewayId", strconv.FormatInt(req.GatewayID, 10))
	if strings.TrimSpace(req.Name) != "" {
		form.Set("name", strings.TrimSpace(req.Name))
	}
	if strings.TrimSpace(req.LicenseType) != "" {
		form.Set("licenseType", strings.TrimSpace(req.LicenseType))
	}
	if strings.TrimSpace(req.License) != "" {
		form.Set("license", req.License)
	}
	raw, err := c.doForm(ctx, gatewayUpdatePath, form)
	if err != nil {
		return Gateway{}, err
	}
	var g Gateway
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &g); err != nil {
			return Gateway{}, fmt.Errorf("decode things gateway: %w", err)
		}
	}
	if g.ID == 0 {
		g.ID = req.GatewayID
	}
	if g.ProjectID == 0 {
		g.ProjectID = req.ProjectID
	}
	return g, nil
}

// DeleteGateway 删除网关。pwd 是 things 服务账号密码。
func (c *Client) DeleteGateway(ctx context.Context, projectID, gatewayID int64, password string) error {
	if projectID <= 0 || gatewayID <= 0 {
		return fmt.Errorf("project id and gateway id are required")
	}
	form := url.Values{}
	form.Set("projectId", strconv.FormatInt(projectID, 10))
	form.Set("gatewayId", strconv.FormatInt(gatewayID, 10))
	form.Set("pwd", password)
	_, err := c.doForm(ctx, gatewayDeletePath, form)
	return err
}

/******************************* 内部实现 *******************************/

func (c *Client) loadConfig(ctx context.Context) (Config, error) {
	if c.config == nil {
		return Config{}, fmt.Errorf("things config provider is not configured")
	}
	cfg, err := c.config(ctx)
	if err != nil {
		return Config{}, err
	}
	cfg = cfg.Normalize()
	if !cfg.Enabled {
		return Config{}, ErrDisabled
	}
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.Username) == "" || cfg.Password == "" {
		return Config{}, fmt.Errorf("things config is incomplete")
	}
	return cfg, nil
}

// token 返回可用 token。并发请求只允许一个登录请求，避免 apiLogin 互相踢下线。
func (c *Client) token(ctx context.Context) (string, Config, error) {
	cfg, err := c.loadConfig(ctx)
	if err != nil {
		return "", cfg, err
	}
	key := cfg.BaseURL + "\x00" + cfg.Username + "\x00" + cfg.Password

	c.mu.Lock()
	if c.cachedToken != "" && c.tokenKey == key && time.Since(c.tokenAt) < cfg.RefreshBefore {
		token := c.cachedToken
		c.mu.Unlock()
		return token, cfg, nil
	}
	c.mu.Unlock()

	c.loginMu.Lock()
	defer c.loginMu.Unlock()

	// 双重检查：等锁期间可能已有其他 goroutine 完成登录。
	c.mu.Lock()
	if c.cachedToken != "" && c.tokenKey == key && time.Since(c.tokenAt) < cfg.RefreshBefore {
		token := c.cachedToken
		c.mu.Unlock()
		return token, cfg, nil
	}
	c.mu.Unlock()

	token, _, err := c.login(ctx, cfg)
	if err != nil {
		return "", cfg, err
	}
	return token, cfg, nil
}

// login 强制登录，成功后写入进程缓存。
func (c *Client) login(ctx context.Context, cfg Config) (string, json.RawMessage, error) {
	form := url.Values{}
	form.Set("username", cfg.Username)
	form.Set("password", cfg.Password)

	raw, err := c.doFormRaw(ctx, cfg, loginPath, form, "")
	if err != nil {
		return "", nil, fmt.Errorf("things login failed: %w", err)
	}
	var info struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		return "", nil, fmt.Errorf("decode things login response: %w", err)
	}
	if strings.TrimSpace(info.Token) == "" {
		return "", nil, fmt.Errorf("things login returned an empty token")
	}

	key := cfg.BaseURL + "\x00" + cfg.Username + "\x00" + cfg.Password
	c.mu.Lock()
	c.cachedToken = info.Token
	c.tokenAt = time.Now()
	c.tokenKey = key
	c.mu.Unlock()
	return info.Token, raw, nil
}

func (c *Client) invalidate(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cachedToken == token {
		c.cachedToken = ""
		c.tokenAt = time.Time{}
	}
}

func (c *Client) markUsed(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cachedToken == token {
		c.tokenAt = time.Now()
	}
}

// doForm 带鉴权执行请求。遇到 code=401 时刷新 token 并重试一次。
func (c *Client) doForm(ctx context.Context, path string, form url.Values) (json.RawMessage, error) {
	for attempt := 0; attempt < 2; attempt++ {
		token, tokenCfg, err := c.token(ctx)
		if err != nil {
			return nil, err
		}
		raw, err := c.doFormRaw(ctx, tokenCfg, path, form, token)
		if err == nil {
			c.markUsed(token)
			return raw, nil
		}
		if IsAuthError(err) && attempt == 0 {
			c.invalidate(token)
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("things api %s failed after token refresh", path)
}

// doFormRaw 发请求并解析 envelope。token 为空时表示登录接口。
func (c *Client) doFormRaw(ctx context.Context, cfg Config, path string, form url.Values, token string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+path, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if token != "" {
		req.Header.Set("Token", token)
	}

	client := &http.Client{Timeout: cfg.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("things request %s failed: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read things response %s failed: %w", path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{
			Code:       resp.StatusCode,
			Message:    fmt.Sprintf("http %d", resp.StatusCode),
			HTTPStatus: resp.StatusCode,
			Path:       path,
		}
	}

	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decode things response %s failed: %w", path, err)
	}
	if env.Code == 0 && env.Message == "" && len(env.Data) == 0 {
		return nil, fmt.Errorf("things response %s is empty or not a json envelope", path)
	}
	if env.Code != http.StatusOK {
		// 兼容只返回 message=UNAUTHORIZED 的网关实现。
		if env.Code == 0 && IsUnauthorizedMessage(env.Message) {
			env.Code = http.StatusUnauthorized
		}
		return nil, &APIError{
			Code:       env.Code,
			Message:    env.Message,
			HTTPStatus: resp.StatusCode,
			Path:       path,
		}
	}
	if len(env.Data) == 0 {
		return nil, nil
	}
	return env.Data, nil
}

func decodeProjectList(raw json.RawMessage) ([]Project, int64, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, 0, nil
	}
	if trimmed[0] == '[' {
		var list []Project
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return nil, 0, fmt.Errorf("decode things project list: %w", err)
		}
		return list, int64(len(list)), nil
	}
	var wrapper struct {
		List          []Project `json:"list"`
		TableData     []Project `json:"tableData"`
		TotalElements int64     `json:"totalElements"`
		Total         int64     `json:"total"`
	}
	if err := json.Unmarshal(trimmed, &wrapper); err != nil {
		return nil, 0, fmt.Errorf("decode things project list: %w", err)
	}
	list := wrapper.List
	if len(list) == 0 {
		list = wrapper.TableData
	}
	total := wrapper.TotalElements
	if total == 0 {
		total = wrapper.Total
	}
	if total == 0 {
		total = int64(len(list))
	}
	return list, total, nil
}

func decodeGatewayList(raw json.RawMessage) ([]Gateway, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var list []Gateway
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return nil, fmt.Errorf("decode things gateway list: %w", err)
		}
		return list, nil
	}
	var wrapper struct {
		List          []Gateway `json:"list"`
		TableData     []Gateway `json:"tableData"`
		TotalRow      int64     `json:"totalRow"`
		TotalElements int64     `json:"totalElements"`
		Total         int64     `json:"total"`
	}
	if err := json.Unmarshal(trimmed, &wrapper); err != nil {
		return nil, fmt.Errorf("decode things gateway list: %w", err)
	}
	if len(wrapper.List) > 0 {
		return wrapper.List, nil
	}
	return wrapper.TableData, nil
}
