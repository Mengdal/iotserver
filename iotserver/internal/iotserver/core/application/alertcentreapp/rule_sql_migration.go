package alertcentreapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"iotserver/internal/dtos"
	resourceContainer "iotserver/internal/iotserver/core/container"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/tenant"
)

const alertRuleSqlResyncTimeout = 30 * time.Second

// resyncAlertRuleSql 把已存在的属性告警规则重新下发为新总线形状。
//
// 696d192 之后属性报文在 mqtt_stream 里是 data.<code>=value，上报时间在
// 消息级 time；历史 eKuiper 规则仍是 data.<code>.value/.time，平台启动后
// 会静默失效。这里按数据库里的规则模型重建 SQL，并保留规则原有的运行状态。
func (p alertApp) resyncAlertRuleSql() (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			p.lc.Errorf("resync alert rule sql panic: %v", r)
			ok = false
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), alertRuleSqlResyncTimeout)
	defer cancel()

	rules, _, err := p.dbClient.WithCtx(tenant.Platform(ctx)).AlertRuleSearch(0, -1, dtos.AlertRuleSearchQueryRequest{})
	if err != nil {
		p.lc.Warnf("resync alert rule sql: query failed: %v", err)
		return false
	}
	if len(rules) == 0 {
		return true
	}

	configapp := resourceContainer.ConfigurationFrom(p.dic.Get)
	actions := dtos.GetRuleAlertEkuiperActions(configapp.Service.CallbackBaseUrl())
	ekuiper := resourceContainer.EkuiperAppFrom(p.dic.Get)
	ok = true
	for _, rule := range rules {
		if !rule.EkuiperRule() {
			continue
		}
		sql, err := p.buildAlertRuleSql(ctx, rule)
		if err != nil {
			p.lc.Warnf("resync alert %s sql: %v", rule.Id, err)
			ok = false
			continue
		}
		stats, err := ekuiper.GetRuleStats(ctx, rule.Id)
		if err != nil {
			p.lc.Warnf("resync alert %s sql: get status failed: %v", rule.Id, err)
			ok = false
			continue
		}
		status, _ := stats["status"].(string)
		wasRunning := status == string(constants.RuleStart)
		if err = ekuiper.UpdateRule(ctx, actions, rule.Id, sql); err != nil {
			p.lc.Warnf("resync alert %s sql: update failed: %v", rule.Id, err)
			ok = false
			continue
		}
		if wasRunning {
			if err = ekuiper.StartRule(ctx, rule.Id); err != nil {
				p.lc.Warnf("resync alert %s sql: restart failed: %v", rule.Id, err)
				ok = false
				continue
			}
		}
		p.lc.Infof("alert %s sql resynced (running=%v)", rule.Id, wasRunning)
	}
	return ok
}

// buildAlertRuleSql 用数据库中的规则模型重建属性告警 SQL。
func (p alertApp) buildAlertRuleSql(ctx context.Context, rule models.AlertRule) (string, error) {
	if len(rule.SubRule) == 0 {
		return "", fmt.Errorf("sub_rule is empty")
	}
	sub := rule.SubRule[0]
	deviceID := strings.TrimSpace(sub.DeviceId)
	if deviceID == "" {
		deviceID = strings.TrimSpace(rule.DeviceId)
	}
	if deviceID == "" {
		return "", fmt.Errorf("device_id is empty")
	}
	device, err := p.dbClient.WithCtx(tenant.Platform(ctx)).DeviceById(deviceID)
	if err != nil {
		return "", fmt.Errorf("load device %s: %w", deviceID, err)
	}
	product, err := p.dbClient.WithCtx(tenant.Platform(ctx)).ProductById(device.ProductId)
	if err != nil {
		return "", fmt.Errorf("load product %s: %w", device.ProductId, err)
	}
	code := strings.TrimSpace(sub.Option["code"])
	if code == "" {
		return "", fmt.Errorf("code is empty")
	}
	var property models.Properties
	found := false
	for _, item := range product.Properties {
		if item.Code == code {
			property = item
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("property %s not found in product %s", code, product.Id)
	}
	req := dtos.RuleUpdateRequest{
		SubRule: []dtos.SubRule{{
			Trigger:   sub.Trigger,
			ProductId: sub.ProductId,
			DeviceId:  device.Id,
			Option:    map[string]string(sub.Option),
		}},
	}
	sql := req.BuildEkuiperSql(device.Id, property.TypeSpec.Type)
	if sql == "" {
		return "", fmt.Errorf("build sql failed for code %s", code)
	}
	return sql, nil
}
