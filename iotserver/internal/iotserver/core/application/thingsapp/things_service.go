package thingsapp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"iotserver/internal/dtos"
	coreContainer "iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	pkgcontainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/things"
)

const (
	configEnabled             = "things.enabled"
	configBaseURL             = "things.baseUrl"
	configUsername            = "things.username"
	configPassword            = "things.password"
	configTimeoutSeconds      = "things.timeoutSeconds"
	configReconcileInterval   = "things.reconcileIntervalSeconds"
	configTokenRefreshMinutes = "things.tokenRefreshMinutes"

	defaultReconcileInterval = 5 * time.Minute
)

// service things 集成应用服务。
//
// 设计边界：这里是“薄适配 + 本地镜像”，不实现 things 的 secret/license 规则。
type service struct {
	dic      *di.Container
	dbClient interfaces.DBClient
	lc       logger.LoggingClient
	client   *things.Client

	reconcileMu sync.Mutex
}

// New 构造 things 应用服务。
func New(dic *di.Container) interfaces.ThingsItf {
	s := &service{
		dic:      dic,
		dbClient: coreContainer.DBClientFrom(dic.Get),
		lc:       pkgcontainer.LoggingClientFrom(dic.Get),
	}
	s.client = things.NewClient(s.rawConfig)
	return s
}

func (s *service) Enabled(ctx context.Context) (bool, error) {
	cfg, err := s.rawConfig(ctx)
	if err != nil {
		return false, err
	}
	return cfg.Enabled, nil
}

func (s *service) TestConnection(ctx context.Context) (dtos.ThingsConnectionStatus, error) {
	cfg, err := s.requireConfig(ctx)
	if err != nil {
		return dtos.ThingsConnectionStatus{}, err
	}
	if _, err = s.client.Login(ctx); err != nil {
		return dtos.ThingsConnectionStatus{}, s.wrapErr("登录 things", err)
	}
	count, err := s.client.Ping(ctx)
	if err != nil {
		return dtos.ThingsConnectionStatus{}, s.wrapErr("读取 things 项目列表", err)
	}
	return dtos.ThingsConnectionStatus{
		BaseUrl:      cfg.BaseURL,
		Username:     cfg.Username,
		ProjectCount: count,
	}, nil
}

func (s *service) ListProjects(ctx context.Context, keyword string) ([]dtos.ThingsProject, error) {
	if _, err := s.requireConfig(ctx); err != nil {
		return nil, err
	}
	list, _, err := s.client.ListProjects(ctx, 1, 100000, keyword)
	if err != nil {
		return nil, s.wrapErr("读取 things 项目列表", err)
	}
	out := make([]dtos.ThingsProject, 0, len(list))
	for _, p := range list {
		out = append(out, dtos.ThingsProject{
			Id:          p.ID,
			Name:        p.Name,
			Description: p.Description,
		})
	}
	return out, nil
}

func (s *service) CreateProject(ctx context.Context, name, description string) (dtos.ThingsProject, error) {
	if _, err := s.requireConfig(ctx); err != nil {
		return dtos.ThingsProject{}, err
	}
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 4 || n > 30 {
		return dtos.ThingsProject{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("things 项目名称长度必须为 4~30 个字符"))
	}
	// 创建接口没有幂等键：先按名称查一次，若已存在且未绑定，则接管它。
	// 这样网络超时后用户重试不会产生重复项目。
	if existing, found, err := s.findProjectByName(ctx, name); err != nil {
		return dtos.ThingsProject{}, err
	} else if found {
		if bound, err := s.projectIsBound(ctx, existing.Id); err != nil {
			return dtos.ThingsProject{}, err
		} else if bound {
			return dtos.ThingsProject{}, errort.NewCommonErr(errort.DefaultNameRepeat,
				fmt.Errorf("things 中已存在同名项目且已绑定租户"))
		}
		return existing, nil
	}

	p, err := s.client.CreateProject(ctx, name, description)
	if err != nil {
		// 请求可能已在 things 侧成功但响应超时；再查一次，查到未绑定项目就接管。
		if existing, found, findErr := s.findProjectByName(ctx, name); findErr == nil && found {
			if bound, bErr := s.projectIsBound(ctx, existing.Id); bErr == nil && !bound {
				return existing, nil
			}
		}
		return dtos.ThingsProject{}, s.wrapErr("创建 things 项目", err)
	}
	return dtos.ThingsProject{
		Id:          p.ID,
		Name:        p.Name,
		Description: p.Description,
	}, nil
}

func (s *service) findProjectByName(ctx context.Context, name string) (dtos.ThingsProject, bool, error) {
	list, _, err := s.client.ListProjects(ctx, 1, 1000, name)
	if err != nil {
		return dtos.ThingsProject{}, false, s.wrapErr("读取 things 项目列表", err)
	}
	for _, p := range list {
		if p.Name == name {
			return dtos.ThingsProject{
				Id:          p.ID,
				Name:        p.Name,
				Description: p.Description,
			}, true, nil
		}
	}
	return dtos.ThingsProject{}, false, nil
}

func (s *service) projectIsBound(ctx context.Context, projectID int64) (bool, error) {
	_, err := s.dbClient.WithCtx(tenant.Platform(ctx)).TenantThingsBindingByProject(projectID)
	if err == nil {
		return true, nil
	}
	if errort.Is(errort.DefaultResourcesNotFound, err) {
		return false, nil
	}
	return false, err
}

func (s *service) RenameProject(ctx context.Context, projectID int64, name string) error {
	if _, err := s.requireConfig(ctx); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 4 || n > 30 {
		return errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("things 项目名称长度必须为 4~30 个字符"))
	}
	if _, err := s.client.UpdateProject(ctx, projectID, name, ""); err != nil {
		return s.wrapErr("修改 things 项目", err)
	}
	return nil
}

func (s *service) DeleteProject(ctx context.Context, projectID int64) error {
	cfg, err := s.requireConfig(ctx)
	if err != nil {
		return err
	}
	if err = s.client.DeleteProject(ctx, projectID, cfg.Password); err != nil {
		if things.IsNotFound(err) {
			return nil
		}
		return s.wrapErr("删除 things 项目", err)
	}
	return nil
}

func (s *service) GetTenantBinding(ctx context.Context, tenantID string) (models.TenantThingsBinding, bool, error) {
	if strings.TrimSpace(tenantID) == "" {
		return models.TenantThingsBinding{}, false, nil
	}
	db := s.dbClient.WithCtx(tenant.Platform(ctx))
	b, err := db.TenantThingsBindingByTenant(tenantID)
	if err != nil {
		if errort.Is(errort.DefaultResourcesNotFound, err) {
			return models.TenantThingsBinding{}, false, nil
		}
		return models.TenantThingsBinding{}, false, err
	}
	return b, true, nil
}

func (s *service) BindTenantProject(ctx context.Context, tenantID string, projectID int64, projectName string) error {
	if strings.TrimSpace(tenantID) == "" {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("tenant id is required"))
	}
	if projectID <= 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("things project id is required"))
	}
	b, err := s.dbClient.WithCtx(tenant.Platform(ctx)).AddOrUpdateTenantThingsBinding(models.TenantThingsBinding{
		TenantField:       models.TenantField{TenantId: tenantID},
		ThingsProjectId:   projectID,
		ThingsProjectName: projectName,
		Status:            models.TenantThingsStatusBound,
		LastSyncAt:        time.Now().UnixMilli(),
	})
	if err != nil {
		return err
	}
	_ = b
	return nil
}

func (s *service) UnbindTenant(ctx context.Context, tenantID string) error {
	return s.dbClient.WithCtx(tenant.Platform(ctx)).DeleteTenantThingsBinding(tenantID)
}

func (s *service) UpdateBindingStatus(ctx context.Context, tenantID, status, lastError string) error {
	return s.dbClient.WithCtx(tenant.Platform(ctx)).
		UpdateTenantThingsBindingStatus(tenantID, status, lastError, time.Now().UnixMilli())
}

/******************************* 配置与错误 *******************************/

// rawConfig 读取原始配置。即使 things 未启用也返回配置，供 Enabled 判断。
func (s *service) rawConfig(ctx context.Context) (things.Config, error) {
	list, _, err := s.dbClient.WithCtx(tenant.Platform(ctx)).
		SysConfigSearch(0, -1, dtos.SysConfigSearchRequest{IsAll: true})
	if err != nil {
		return things.Config{}, err
	}
	values := make(map[string]string, len(list))
	for _, item := range list {
		values[item.ConfigKey] = item.ConfigValue
	}
	cfg := things.Config{
		Enabled:       parseBool(values[configEnabled]),
		BaseURL:       strings.TrimSpace(values[configBaseURL]),
		Username:      strings.TrimSpace(values[configUsername]),
		Timeout:       time.Duration(parseInt(values[configTimeoutSeconds], 10)) * time.Second,
		RefreshBefore: time.Duration(parseInt(values[configTokenRefreshMinutes], 90)) * time.Minute,
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = things.DefaultBaseURL
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = things.DefaultTimeout
	}
	if cfg.RefreshBefore <= 0 {
		cfg.RefreshBefore = things.DefaultRefreshBefore
	}
	encrypted := strings.TrimSpace(values[configPassword])
	if cfg.Enabled && encrypted != "" {
		plain, dErr := things.DecryptPassword(encrypted)
		if dErr != nil {
			return cfg, errort.NewCommonErr(errort.AppThingsNotConfigured,
				fmt.Errorf("things 服务账号密码无法解密，请重新保存配置"))
		}
		cfg.Password = plain
	}
	return cfg, nil
}

func (s *service) requireConfig(ctx context.Context) (things.Config, error) {
	cfg, err := s.rawConfig(ctx)
	if err != nil {
		return cfg, err
	}
	if !cfg.Enabled {
		return cfg, errort.NewCommonErr(errort.AppThingsDisabled, fmt.Errorf("things 集成未启用"))
	}
	if cfg.Username == "" || cfg.Password == "" {
		return cfg, errort.NewCommonErr(errort.AppThingsNotConfigured,
			fmt.Errorf("things 服务账号或密码未配置"))
	}
	return cfg, nil
}

func (s *service) wrapErr(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, things.ErrDisabled) {
		return errort.NewCommonErr(errort.AppThingsDisabled, fmt.Errorf("things 集成未启用"))
	}
	var apiErr *things.APIError
	if errors.As(err, &apiErr) {
		msg := strings.TrimSpace(apiErr.Message)
		lower := strings.ToLower(msg)
		switch {
		case things.IsAuthError(err):
			return errort.NewCommonErr(errort.AppThingsAuthFailed,
				fmt.Errorf("things 登录已失效，请检查服务账号"))
		case strings.Contains(lower, "secret error"):
			return errort.NewCommonErr(errort.AppThingsSecretRejected,
				fmt.Errorf("things 拒绝该网关密钥（Secret），请核对公司提供的认证信息"))
		case strings.Contains(lower, "license error"):
			return errort.NewCommonErr(errort.AppThingsLicenseRejected,
				fmt.Errorf("things 拒绝该 License，请核对授权信息"))
		case strings.Contains(lower, "already exist") ||
			strings.Contains(lower, "already bind") ||
			strings.Contains(msg, "同名") ||
			strings.Contains(msg, "已存在"):
			return errort.NewCommonErr(errort.DefaultNameRepeat,
				fmt.Errorf("things: %s", msg))
		case things.IsNotFound(err):
			return errort.NewCommonErr(errort.AppThingsProjectNotFound,
				fmt.Errorf("things 资源不存在，可能已在 things 中删除"))
		case apiErr.Code == 403:
			return errort.NewCommonErr(errort.DefaultTokenPermission,
				fmt.Errorf("things 服务账号权限不足"))
		default:
			return errort.NewCommonErr(errort.DefaultSystemError,
				fmt.Errorf("%s失败: %s", op, msg))
		}
	}
	if strings.Contains(err.Error(), "config is incomplete") {
		return errort.NewCommonErr(errort.AppThingsNotConfigured,
			fmt.Errorf("things 配置不完整"))
	}
	return errort.NewCommonErr(errort.AppThingsUnavailable,
		fmt.Errorf("%s失败，things 平台不可用: %v", op, err))
}

func (s *service) Start(ctx context.Context) {
	go func() {
		// 启动后先给系统初始化留一点时间，再进入周期对账。
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}

			interval := defaultReconcileInterval
			if cfg, err := s.rawConfig(ctx); err == nil {
				if !cfg.Enabled {
					interval = time.Minute
				} else if sec := s.reconcileInterval(ctx); sec > 0 {
					interval = time.Duration(sec) * time.Second
				}
			}
			if _, err := s.ReconcileAll(ctx); err != nil {
				if s.lc != nil {
					s.lc.Warnf("things 对账失败: %v", err)
				}
			}
			timer.Reset(interval)
		}
	}()
}

func (s *service) reconcileInterval(ctx context.Context) int {
	list, _, err := s.dbClient.WithCtx(tenant.Platform(ctx)).
		SysConfigSearch(0, -1, dtos.SysConfigSearchRequest{IsAll: true})
	if err != nil {
		return 0
	}
	for _, item := range list {
		if item.ConfigKey == configReconcileInterval {
			return parseInt(item.ConfigValue, 300)
		}
	}
	return 300
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

func parseInt(value string, fallback int) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}
