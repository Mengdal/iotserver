package tenantapp

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	pkgcontainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
)

// tenantCodePattern 租户编码：小写字母、数字、下划线，2~32 位（用于域名/主题前缀）
var tenantCodePattern = regexp.MustCompile(`^[a-z0-9_]{2,32}$`)

type tenantApp struct {
	dic      *di.Container
	dbClient interfaces.DBClient
	lc       logger.LoggingClient
}

// New 构造租户应用
func New(dic *di.Container) interfaces.TenantItf {
	return &tenantApp{
		dic:      dic,
		dbClient: container.DBClientFrom(dic.Get),
		lc:       pkgcontainer.LoggingClientFrom(dic.Get),
	}
}

func toTenantResponse(t models.Tenant) dtos.TenantResponse {
	return dtos.TenantResponse{
		Id:            t.Id,
		Code:          t.Code,
		Name:          t.Name,
		Status:        t.Status,
		ContactName:   t.ContactName,
		ContactPhone:  t.ContactPhone,
		ContactEmail:  t.ContactEmail,
		ExpireAt:      t.ExpireAt,
		MaxDevices:    t.MaxDevices,
		MaxGateways:   t.MaxGateways,
		MaxUsers:      t.MaxUsers,
		RetentionDays: t.RetentionDays,
		Remark:        t.Remark,
		Created:       t.Created,
		Modified:      t.Modified,
	}
}

// TenantAdd 新建租户
func (app *tenantApp) TenantAdd(ctx context.Context, req dtos.TenantAddRequest) (dtos.TenantResponse, error) {
	var res dtos.TenantResponse
	code := strings.ToLower(strings.TrimSpace(req.Code))
	if !tenantCodePattern.MatchString(code) {
		return res, errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("租户编码只允许小写字母/数字/下划线，长度 2~32", nil))
	}
	tc, err := app.dbClient.WithCtx(ctx).AddTenant(models.Tenant{
		Code:          code,
		Name:          strings.TrimSpace(req.Name),
		ContactName:   req.ContactName,
		ContactPhone:  req.ContactPhone,
		ContactEmail:  req.ContactEmail,
		ExpireAt:      req.ExpireAt,
		MaxDevices:    req.MaxDevices,
		MaxGateways:   req.MaxGateways,
		MaxUsers:      req.MaxUsers,
		RetentionDays: req.RetentionDays,
		Remark:        req.Remark,
	})
	if err != nil {
		return res, err
	}
	// 为新租户准备组织树根节点（公司），便于租户直接往下建工厂/车间/产线
	if _, dErr := app.dbClient.
		WithCtx(tenant.With(ctx, tenant.Context{TenantID: tc.Id})).
		AddSysDept(models.Dept{
			ParentId:  "0",
			Ancestors: "0",
			Name:      tc.Name,
			DeptType:  models.DeptTypeCompany,
			Sort:      1,
			Status:    1,
		}); dErr != nil && app.lc != nil {
		app.lc.Warnf("为租户 %s 创建根组织节点失败: %v", tc.Code, dErr)
	}
	return toTenantResponse(tc), nil
}

// TenantUpdate 修改租户
func (app *tenantApp) TenantUpdate(ctx context.Context, req dtos.TenantUpdateRequest) error {
	_, err := app.dbClient.WithCtx(ctx).TenantById(req.Id)
	if err != nil {
		return err
	}
	return app.dbClient.WithCtx(ctx).UpdateTenant(models.Tenant{
		Id:            req.Id,
		Name:          strings.TrimSpace(req.Name),
		ContactName:   req.ContactName,
		ContactPhone:  req.ContactPhone,
		ContactEmail:  req.ContactEmail,
		ExpireAt:      req.ExpireAt,
		MaxDevices:    req.MaxDevices,
		MaxGateways:   req.MaxGateways,
		MaxUsers:      req.MaxUsers,
		RetentionDays: req.RetentionDays,
		Status:        req.Status,
		Remark:        req.Remark,
	})
}

// TenantDelete 删除租户
//
// 保护规则（设计文档 §3.1）：删除前统计该租户下的
// 用户 / 设备 / 网关 / 产品 / 看板数量，只要有任何一项 > 0 就拒绝删除，
// 并在错误信息里带上各项数量；全部为 0 才允许删除。
//
// 计数必须按**被删租户**的作用域进行：平台管理员可能正代管着别的租户
// （x-tenant-id），若沿用请求上下文，GORM 租户插件会叠加出互斥条件导致误判为 0。
//
// 注意：删除成功后会顺带清理该租户的 tenant_menu / tenant_menu_pref
// （菜单定制表若尚未建出则自动跳过，见 CleanupTenantMenuRecords）。
// 业务数据不级联删除，避免误删。
func (app *tenantApp) TenantDelete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return errort.NewCommonErr(errort.DefaultIdEmpty, fmt.Errorf("tenant id is required"))
	}
	// 先确认租户存在（不存在时直接返回 404，不进入计数）
	if _, err := app.dbClient.WithCtx(ctx).TenantById(id); err != nil {
		return err
	}
	db := app.dbClient.WithCtx(tenant.With(ctx, tenant.Context{TenantID: id}))
	users, err := db.TenantUserCount(id)
	if err != nil {
		return err
	}
	devices, err := db.TenantDeviceCount(id)
	if err != nil {
		return err
	}
	gateways, err := db.TenantGatewayCount(id)
	if err != nil {
		return err
	}
	products, err := db.TenantProductCount(id)
	if err != nil {
		return err
	}
	// 看板只统计「用户自建」的：内置看板（首页/组件演示等）是系统自动种入的脚手架，
	// 不应该挡住租户删除；租户删除后会连同看板一起清理。
	boards, err := db.VizBoards()
	if err != nil {
		return err
	}
	customBoards := int64(0)
	for _, b := range boards {
		if !isBuiltinVizBoard(b) {
			customBoards++
		}
	}
	if users+devices+gateways+products+customBoards > 0 {
		return errort.NewCommonErr(errort.AppTenantNotEmpty, errort.NewMessageErr("该租户下仍有 {{.p1}} 个用户、{{.p2}} 台设备、{{.p3}} 台网关、{{.p4}} 个产品、{{.p5}} 个自定义看板，请先清理", map[string]interface{}{"p1": users, "p2": devices, "p3": gateways, "p4": products, "p5": customBoards}))
	}
	if err = app.dbClient.WithCtx(ctx).DeleteTenant(id); err != nil {
		return err
	}
	// 看板随租户一起清理；失败不回滚删除（与菜单定制数据一致）
	if err = db.VizBoardsReplace(nil); err != nil && app.lc != nil {
		app.lc.Warnf("删除租户 %s 后清理看板失败: %v", id, err)
	}
	// 菜单定制数据清理失败不回滚删除：菜单定制属于展示层数据，缺失不影响主体删除
	if err = app.dbClient.WithCtx(ctx).CleanupTenantMenuRecords(id); err != nil && app.lc != nil {
		app.lc.Warnf("删除租户 %s 后清理菜单定制数据失败: %v", id, err)
	}
	return nil
}

// isBuiltinVizBoard 是否为系统内置看板。
// id 前缀 builtin-/bd- 覆盖现有与历史命名，名称兜底覆盖手工导入的旧数据。
func isBuiltinVizBoard(b models.VizBoard) bool {
	if strings.HasPrefix(b.Id, "builtin-") || strings.HasPrefix(b.Id, "bd-") {
		return true
	}
	switch b.Name {
	case "首页看板", "组件演示看板", "系统监控看板", "产品介绍看板":
		return true
	}
	return false
}

// TenantById 租户详情
func (app *tenantApp) TenantById(ctx context.Context, id string) (dtos.TenantResponse, error) {
	tenant, err := app.dbClient.WithCtx(ctx).TenantById(id)
	if err != nil {
		return dtos.TenantResponse{}, err
	}
	return toTenantResponse(tenant), nil
}

// TenantSearch 租户列表
func (app *tenantApp) TenantSearch(ctx context.Context, req dtos.TenantSearchQueryRequest) ([]dtos.TenantResponse, uint32, error) {
	offset, limit := req.GetPage()
	list, total, err := app.dbClient.WithCtx(ctx).TenantSearch(offset, limit, req)
	if err != nil {
		return nil, 0, err
	}
	out := make([]dtos.TenantResponse, len(list))
	for i, t := range list {
		out[i] = toTenantResponse(t)
	}
	return out, total, nil
}
