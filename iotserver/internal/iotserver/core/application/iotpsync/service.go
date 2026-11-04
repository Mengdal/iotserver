// Package iotpsync 把平台的「产品/物模型/设备」同步到 iotp 的元数据。
//
// 只做元数据：产品 → 库/表，物模型属性 → 字段，设备 → 设备实例。
// 时序数据写入不在本包；同步全部异步执行，iotp 不可用不会阻塞平台的产品/设备创建。
package iotpsync

import (
	"context"
	"strings"
	"time"

	"iotserver/internal/iotserver/core/container"
	"iotserver/internal/models"
	pkgContainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/tools/iotp"
)

const defaultTimeout = 30 * time.Second

// Service iotp 元数据同步服务。
type Service struct {
	dic     *di.Container
	lc      logger.LoggingClient
	enabled bool
	client  *iotp.Client
}

func New(dic *di.Container) *Service {
	cfg := container.ConfigurationFrom(dic.Get)
	lc := pkgContainer.LoggingClientFrom(dic.Get)
	service := &Service{dic: dic, lc: lc}
	if cfg == nil || !cfg.Iotp.Enabled || strings.TrimSpace(cfg.Iotp.BaseURL) == "" {
		lc.Infof("iotp 元数据同步未启用")
		return service
	}
	database := strings.TrimSpace(cfg.Iotp.Database)
	if database == "" {
		database = "iotserver"
	}
	timeout := time.Duration(cfg.Iotp.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	service.enabled = true
	service.client = iotp.New(cfg.Iotp.BaseURL, cfg.Iotp.Token, database, timeout)
	lc.Infof("iotp 元数据同步已启用: %s db=%s", cfg.Iotp.BaseURL, database)
	return service
}

func (s *Service) Enabled() bool {
	return s != nil && s.enabled && s.client != nil
}

// ProductCreated 产品创建/发布后同步整张表（基础标签 + 物模型属性 + 事件/服务字段）。
func (s *Service) ProductCreated(product models.Product, tenantID string) {
	if !s.Enabled() {
		return
	}
	go s.run("product:"+product.Id, func(ctx context.Context) error {
		return s.syncProduct(ctx, product, tenantID)
	})
}

// PropertyChanged 物模型属性新增/修改后同步字段。
func (s *Service) PropertyChanged(productID string, property models.Properties, tenantID string) {
	if !s.Enabled() || strings.TrimSpace(productID) == "" {
		return
	}
	go s.run("property:"+productID+":"+property.Code, func(ctx context.Context) error {
		return s.syncProperty(ctx, productID, property)
	})
}

// EventChanged 物模型事件新增/修改后同步字段。
func (s *Service) EventChanged(productID string, event models.Events, tenantID string) {
	s.changeField(productID, "evt_"+iotp.FieldName(event.Code))
}

// ActionChanged 物模型服务新增/修改后同步字段。
func (s *Service) ActionChanged(productID string, action models.Actions, tenantID string) {
	s.changeField(productID, "act_"+iotp.FieldName(action.Code))
}

func (s *Service) changeField(productID, fieldName string) {
	if !s.Enabled() || strings.TrimSpace(productID) == "" || strings.TrimSpace(fieldName) == "" {
		return
	}
	go s.run("field:"+productID+":"+fieldName, func(ctx context.Context) error {
		table := iotp.TableName(productID)
		if err := s.client.EnsureDatabase(ctx); err != nil {
			return err
		}
		if err := s.client.EnsureTable(ctx, table, iotp.BaseFields()); err != nil {
			return err
		}
		return s.client.AddField(ctx, table, iotp.Field{Name: fieldName, Type: iotp.TypeString})
	})
}

// DeviceCreated 设备创建后注册 iotp 设备。
func (s *Service) DeviceCreated(device models.Device, product models.Product, tenantID string) {
	if !s.Enabled() {
		return
	}
	go s.run("device:"+device.Id, func(ctx context.Context) error {
		return s.syncDevice(ctx, device, product, tenantID)
	})
}

// DeviceDeleted 设备删除后删除 iotp 设备（历史数据保留）。
func (s *Service) DeviceDeleted(device models.Device) {
	if !s.Enabled() {
		return
	}
	go s.run("device-delete:"+device.Id, func(ctx context.Context) error {
		return s.client.DeleteDevice(ctx, iotp.DeviceID(device.Id))
	})
}

func (s *Service) run(what string, fn func(context.Context) error) {
	defer func() {
		if r := recover(); r != nil {
			s.lc.Errorf("iotp 同步 %s panic: %v", what, r)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	if err := fn(ctx); err != nil {
		s.lc.Errorf("iotp 同步 %s 失败: %v", what, err)
		return
	}
	s.lc.Infof("iotp 同步 %s 完成", what)
}

func (s *Service) syncProduct(ctx context.Context, product models.Product, tenantID string) error {
	table := iotp.TableName(product.Id)
	fields := iotp.BaseFields()
	for _, property := range product.Properties {
		fields = append(fields, propertyField(property))
	}
	for _, event := range product.Events {
		fields = append(fields, iotp.Field{Name: "evt_" + iotp.FieldName(event.Code), Type: iotp.TypeString})
	}
	for _, action := range product.Actions {
		fields = append(fields, iotp.Field{Name: "act_" + iotp.FieldName(action.Code), Type: iotp.TypeString})
	}
	if err := s.client.EnsureDatabase(ctx); err != nil {
		return err
	}
	return s.client.EnsureTable(ctx, table, iotp.DedupeFields(fields))
}

func (s *Service) syncProperty(ctx context.Context, productID string, property models.Properties) error {
	table := iotp.TableName(productID)
	if err := s.client.EnsureDatabase(ctx); err != nil {
		return err
	}
	if err := s.client.EnsureTable(ctx, table, iotp.BaseFields()); err != nil {
		return err
	}
	return s.client.AddField(ctx, table, propertyField(property))
}

func (s *Service) syncDevice(ctx context.Context, device models.Device, product models.Product, tenantID string) error {
	table := iotp.TableName(product.Id)
	if err := s.client.EnsureDatabase(ctx); err != nil {
		return err
	}
	if err := s.client.EnsureTable(ctx, table, iotp.BaseFields()); err != nil {
		return err
	}
	deviceKey := strings.TrimSpace(device.DeviceKey)
	if deviceKey == "" {
		deviceKey = strings.TrimSpace(device.DeviceSn)
	}
	if deviceKey == "" {
		deviceKey = device.Id
	}
	tags := map[string]string{
		"tenant_id":  tenantID,
		"product_id": device.ProductId,
		"device_sn":  device.DeviceSn,
		"device_key": deviceKey,
		"dept_id":    device.DeptId,
	}
	attrs := map[string]any{
		"name":      device.Name,
		"productId": device.ProductId,
	}
	return s.client.RegisterDevice(ctx, table, iotp.Device{
		DeviceID: iotp.DeviceID(device.Id),
		Name:     device.Name,
		Tags:     tags,
		Attrs:    attrs,
	})
}

func propertyField(property models.Properties) iotp.Field {
	return iotp.Field{
		Name: iotp.FieldName(property.Code),
		Type: iotp.FieldType(property.TypeSpec.Type),
	}
}
