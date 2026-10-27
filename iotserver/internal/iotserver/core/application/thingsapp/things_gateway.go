package thingsapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/things"
)

func (s *service) CreateGateway(ctx context.Context, projectID int64, req dtos.ThingsGatewayCreateRequest) (dtos.ThingsGateway, error) {
	if _, err := s.requireConfig(ctx); err != nil {
		return dtos.ThingsGateway{}, err
	}
	if projectID <= 0 {
		return dtos.ThingsGateway{}, errort.NewCommonErr(errort.AppThingsBindingMissing,
			fmt.Errorf("当前租户尚未绑定 things 项目"))
	}
	g, err := s.client.CreateGateway(ctx, things.CreateGatewayRequest{
		ProjectID:   projectID,
		SN:          strings.TrimSpace(req.Sn),
		Secret:      req.Secret,
		Name:        strings.TrimSpace(req.Name),
		Model:       strings.TrimSpace(req.Model),
		LicenseType: strings.TrimSpace(req.LicenseType),
		License:     req.License,
	})
	if err != nil {
		return dtos.ThingsGateway{}, s.wrapErr("创建 things 网关", err)
	}
	return toThingsGateway(g), nil
}

func (s *service) RenameGateway(ctx context.Context, projectID, gatewayID int64, name string) error {
	if _, err := s.requireConfig(ctx); err != nil {
		return err
	}
	if projectID <= 0 || gatewayID <= 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("things project/gateway id is required"))
	}
	if strings.TrimSpace(name) == "" {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("gateway name is required"))
	}
	if _, err := s.client.UpdateGateway(ctx, things.UpdateGatewayRequest{
		ProjectID: projectID,
		GatewayID: gatewayID,
		Name:      name,
	}); err != nil {
		return s.wrapErr("修改 things 网关", err)
	}
	return nil
}

func (s *service) DeleteGateway(ctx context.Context, projectID, gatewayID int64) error {
	cfg, err := s.requireConfig(ctx)
	if err != nil {
		return err
	}
	if projectID <= 0 || gatewayID <= 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("things project/gateway id is required"))
	}
	if err = s.client.DeleteGateway(ctx, projectID, gatewayID, cfg.Password); err != nil {
		if things.IsNotFound(err) {
			return nil
		}
		return s.wrapErr("删除 things 网关", err)
	}
	return nil
}

// ReconcileTenant 把 things 项目下的网关同步为本地镜像。
//
// 只更新 things 拥有的展示字段；不覆盖本地在线状态、配置、设备数据。
func (s *service) ReconcileTenant(ctx context.Context, tenantID string) (dtos.ThingsReconcileResult, error) {
	var result dtos.ThingsReconcileResult
	if strings.TrimSpace(tenantID) == "" {
		return result, nil
	}
	if _, err := s.requireConfig(ctx); err != nil {
		return result, err
	}
	binding, found, err := s.GetTenantBinding(ctx, tenantID)
	if err != nil {
		return result, err
	}
	if !found {
		return result, errort.NewCommonErr(errort.AppThingsBindingMissing,
			fmt.Errorf("租户 %s 尚未绑定 things 项目", tenantID))
	}
	remote, err := s.client.ListGateways(ctx, binding.ThingsProjectId)
	if err != nil {
		_ = s.UpdateBindingStatus(ctx, tenantID, models.TenantThingsStatusError, err.Error())
		return result, s.wrapErr("读取 things 网关列表", err)
	}
	result.Tenants = 1
	result.Gateways = len(remote)

	tenantCtx := tenant.With(ctx, tenant.Context{TenantID: tenantID})
	local, _, err := s.dbClient.WithCtx(tenantCtx).
		GatewaySearch(0, -1, dtos.GatewaySearchQueryRequest{})
	if err != nil {
		return result, err
	}

	localByThingsID := make(map[int64]models.Gateway, len(local))
	localBySN := make(map[string]models.Gateway, len(local))
	for _, g := range local {
		if g.ThingsGatewayId > 0 {
			localByThingsID[g.ThingsGatewayId] = g
		}
		localBySN[g.Sn] = g
	}

	seen := make(map[string]struct{}, len(remote))
	for _, rg := range remote {
		pk := strings.TrimSpace(rg.PK)
		if pk == "" {
			pk = rg.SN
		}
		localGateway, byID := localByThingsID[rg.ID]
		if !byID {
			if candidate, ok := localBySN[rg.SN]; ok {
				// 存量 edge 网关允许接管；自有 MQTT 网关不静默改成 things 网关。
				if sourceOf(candidate) == models.GatewaySourceThings ||
					(strings.EqualFold(candidate.Protocol, models.GatewayProtocolEdge)) {
					localGateway = candidate
					byID = true
				} else {
					result.Failed++
					_ = s.dbClient.WithCtx(tenantCtx).UpdateGatewayThingsStatus(candidate.Id,
						models.GatewayThingsSyncConflict,
						fmt.Sprintf("SN %s 同时属于本地网关和 things 网关", rg.SN), time.Now().UnixMilli())
					continue
				}
			}
		}

		if byID {
			if localGateway.ThingsGatewayId > 0 && localGateway.ThingsGatewayId != rg.ID {
				result.Failed++
				_ = s.dbClient.WithCtx(tenantCtx).UpdateGatewayThingsStatus(localGateway.Id,
					models.GatewayThingsSyncConflict,
					fmt.Sprintf("things 网关 ID 不一致：本地 %d，远端 %d", localGateway.ThingsGatewayId, rg.ID),
					time.Now().UnixMilli())
				continue
			}
			localGateway.Name = rg.Name
			localGateway.Model = rg.Model
			localGateway.Pk = pk
			localGateway.Protocol = models.GatewayProtocolEdge
			localGateway.Source = models.GatewaySourceThings
			localGateway.ThingsGatewayId = rg.ID
			localGateway.ThingsProjectId = binding.ThingsProjectId
			localGateway.ThingsSyncStatus = models.GatewayThingsSyncSynced
			localGateway.ThingsLastError = ""
			if err := s.dbClient.WithCtx(tenantCtx).UpdateGatewayThingsSync(localGateway); err != nil {
				result.Failed++
				continue
			}
		} else {
			_, err := s.dbClient.WithCtx(tenantCtx).AddGateway(models.Gateway{
				TenantField:      models.TenantField{TenantId: tenantID},
				Name:             rg.Name,
				Sn:               rg.SN,
				Pk:               pk,
				Protocol:         models.GatewayProtocolEdge,
				Source:           models.GatewaySourceThings,
				Model:            rg.Model,
				ThingsGatewayId:  rg.ID,
				ThingsProjectId:  binding.ThingsProjectId,
				ThingsSyncStatus: models.GatewayThingsSyncSynced,
				ThingsLastSyncAt: time.Now().UnixMilli(),
				Status:           models.GatewayStatusNever,
			})
			if err != nil {
				result.Failed++
				if s.lc != nil {
					s.lc.Warnf("things 对账：导入网关 %s 失败: %v", rg.SN, err)
				}
				continue
			}
		}
		seen[rg.SN] = struct{}{}
	}

	// 本地有、things 无：只标记，不删本地设备/历史数据。
	now := time.Now().UnixMilli()
	for _, g := range local {
		if _, ok := seen[g.Sn]; ok {
			continue
		}
		if sourceOf(g) != models.GatewaySourceThings && g.ThingsGatewayId == 0 {
			continue
		}
		result.Missing++
		_ = s.dbClient.WithCtx(tenantCtx).UpdateGatewayThingsStatus(g.Id,
			models.GatewayThingsSyncMissing,
			"things 中已不存在该网关", now)
	}

	lastErr := ""
	if result.Failed > 0 {
		lastErr = fmt.Sprintf("%d 个网关存在同步冲突", result.Failed)
	}
	_ = s.UpdateBindingStatus(ctx, tenantID, models.TenantThingsStatusBound, lastErr)
	return result, nil
}

func (s *service) ReconcileAll(ctx context.Context) (dtos.ThingsReconcileResult, error) {
	var total dtos.ThingsReconcileResult
	enabled, err := s.Enabled(ctx)
	if err != nil {
		return total, err
	}
	if !enabled {
		return total, nil
	}
	bindings, err := s.dbClient.WithCtx(tenant.Platform(ctx)).TenantThingsBindings()
	if err != nil {
		return total, err
	}
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	for _, b := range bindings {
		res, err := s.ReconcileTenant(ctx, b.TenantId)
		if err != nil {
			total.Failed++
			if s.lc != nil {
				s.lc.Warnf("things 对账：租户 %s 失败: %v", b.TenantId, err)
			}
			continue
		}
		total.Tenants += res.Tenants
		total.Gateways += res.Gateways
		total.Missing += res.Missing
		total.Failed += res.Failed
	}
	return total, nil
}

func toThingsGateway(g things.Gateway) dtos.ThingsGateway {
	pk := strings.TrimSpace(g.PK)
	if pk == "" {
		pk = g.SN
	}
	return dtos.ThingsGateway{
		Id:        g.ID,
		ProjectId: g.ProjectID,
		Sn:        g.SN,
		Pk:        pk,
		Name:      g.Name,
		Model:     g.Model,
	}
}

func sourceOf(g models.Gateway) string {
	if g.Source == "" {
		return models.GatewaySourceLocal
	}
	return g.Source
}
