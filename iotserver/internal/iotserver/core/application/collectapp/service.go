// Package collectapp 采集配置。
//
// 新版采集引擎（IOTP Gateway API）的数据模型：
//   - 通道：/api/v1/channels
//   - IOTP 设备绑定：/api/v1/iotp_devices，只存 channelId + iotpId
//   - 点位：/api/v1/tags，挂在通道下（channelId）
//
// 因此本包直接代理引擎接口；系统设备表只用于「选设备」和把 iotpId 映射回设备/产品。
package collectapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	pkgContainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/collectengine"
	"iotserver/internal/tools/iotp"
)

var channelTypes = map[string]struct{}{
	"serialport": {},
	"network":    {},
}

type service struct {
	dic        *di.Container
	lc         logger.LoggingClient
	dbClient   interfaces.DBClient
	engine     *collectengine.Client
	iotpClient *iotp.Client
}

func NewCollectApp(ctx context.Context, dic *di.Container) *service {
	configuration := container.ConfigurationFrom(dic.Get)
	var engine *collectengine.Client
	if configuration.Integrations.CollectEngine.Enabled && strings.TrimSpace(configuration.Integrations.CollectEngine.BaseURL) != "" {
		engine = collectengine.New(configuration.Integrations.CollectEngine.BaseURL,
			time.Duration(configuration.Integrations.CollectEngine.TimeoutSec)*time.Second)
	}
	var iotpClient *iotp.Client
	if configuration.Integrations.Iotp.Enabled && strings.TrimSpace(configuration.Integrations.Iotp.BaseURL) != "" {
		iotpClient = iotp.New(configuration.Integrations.Iotp.BaseURL, configuration.Integrations.Iotp.Token,
			configuration.Integrations.Iotp.Database, time.Duration(configuration.Integrations.Iotp.TimeoutSec)*time.Second)
	}
	return &service{
		dic:        dic,
		lc:         pkgContainer.LoggingClientFrom(dic.Get),
		dbClient:   container.DBClientFrom(dic.Get),
		engine:     engine,
		iotpClient: iotpClient,
	}
}

func (s *service) engineClient() (*collectengine.Client, error) {
	if s.engine == nil {
		return nil, errort.NewCommonErr(errort.DefaultSystemError,
			fmt.Errorf("未启用采集引擎，请在 configuration 中配置 [Integrations.CollectEngine]"))
	}
	return s.engine, nil
}

// ownedChannelIds 当前租户在本地登记过的采集通道（键为采集引擎通道 ID）。
func (s *service) ownedChannelIds(ctx context.Context) (map[string]struct{}, error) {
	channels, err := s.dbClient.WithCtx(ctx).CollectChannels()
	if err != nil {
		return nil, err
	}
	owned := make(map[string]struct{}, len(channels))
	for _, channel := range channels {
		owned[channel.Id] = struct{}{}
	}
	return owned, nil
}

// canAccessChannel 采集引擎本身没有租户字段，用本地的通道登记表做归属判断：
// 平台作用域可访问全部通道，租户用户只能访问自己创建（登记过）的通道。
func (s *service) canAccessChannel(ctx context.Context, channelId int64) (bool, error) {
	if tenant.FromContext(ctx).Platform {
		return true, nil
	}
	owned, err := s.ownedChannelIds(ctx)
	if err != nil {
		return false, err
	}
	_, ok := owned[strconv.FormatInt(channelId, 10)]
	return ok, nil
}

// saveChannelOwnership 创建/更新通道时登记归属，供租户隔离与删除清理使用。
func (s *service) saveChannelOwnership(ctx context.Context, channel collectengine.Channel, channelType string) error {
	if channel.Id <= 0 {
		return nil
	}
	_, err := s.dbClient.WithCtx(ctx).SaveCollectChannel(models.CollectChannel{
		Id:             strconv.FormatInt(channel.Id, 10),
		RemoteId:       channel.Id,
		Name:           channel.ChannelCode,
		ChannelType:    channelType,
		Protocol:       channel.Protocol,
		Config:         channel.Advance,
		PollIntervalMs: int(channel.PollInterval),
		TimeoutMs:      int(channel.Timeout),
		Enabled:        !channel.Disable,
	})
	return err
}

// filterAccessibleBindings 只保留当前租户可访问通道下的引擎设备绑定。
func (s *service) filterAccessibleBindings(ctx context.Context, bindings []collectengine.Device) ([]collectengine.Device, error) {
	if tenant.FromContext(ctx).Platform {
		return bindings, nil
	}
	owned, err := s.ownedChannelIds(ctx)
	if err != nil {
		return nil, err
	}
	filtered := make([]collectengine.Device, 0, len(bindings))
	for _, binding := range bindings {
		if _, ok := owned[strconv.FormatInt(binding.ChannelId, 10)]; ok {
			filtered = append(filtered, binding)
		}
	}
	return filtered, nil
}

/******************************* 通道 *******************************/

func (s *service) CollectChannels(ctx context.Context) ([]dtos.CollectChannelDTO, error) {
	client, err := s.engineClient()
	if err != nil {
		return nil, err
	}
	channels, err := client.ListChannels()
	if err != nil {
		return nil, err
	}
	result := make([]dtos.CollectChannelDTO, 0, len(channels))
	for _, channel := range channels {
		result = append(result, channelDTO(channel))
	}
	// 租户隔离：租户用户只看到自己创建（本地登记）的通道；平台视图看全部。
	if !tenant.FromContext(ctx).Platform {
		owned, err := s.ownedChannelIds(ctx)
		if err != nil {
			return nil, err
		}
		filtered := make([]dtos.CollectChannelDTO, 0, len(result))
		for _, channel := range result {
			if _, ok := owned[strconv.FormatInt(channel.Id, 10)]; ok {
				filtered = append(filtered, channel)
			}
		}
		result = filtered
	}
	return result, nil
}

func (s *service) CollectChannelSave(ctx context.Context, req dtos.CollectChannelSaveRequest) (dtos.CollectChannelDTO, error) {
	client, err := s.engineClient()
	if err != nil {
		return dtos.CollectChannelDTO{}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return dtos.CollectChannelDTO{}, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("通道名称不能为空"))
	}
	protocol := strings.TrimSpace(req.Protocol)
	if protocol == "" {
		return dtos.CollectChannelDTO{}, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("协议不能为空"))
	}
	channelType := strings.TrimSpace(req.ChannelType)
	if channelType == "" {
		channelType = "network"
	}
	if _, ok := channelTypes[channelType]; !ok {
		return dtos.CollectChannelDTO{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("通道类型 %s 不支持", channelType))
	}
	if err := validateChannelConfig(channelType, req.Config); err != nil {
		return dtos.CollectChannelDTO{}, errort.NewCommonErr(errort.DefaultReqParamsError, err)
	}

	if req.Id > 0 {
		ok, err := s.canAccessChannel(ctx, req.Id)
		if err != nil {
			return dtos.CollectChannelDTO{}, err
		}
		if !ok {
			return dtos.CollectChannelDTO{}, errort.NewCommonErr(errort.DefaultResourcesNotFound,
				fmt.Errorf("通道不存在或无权访问"))
		}
	}
	channels, err := client.ListChannels()
	if err != nil {
		return dtos.CollectChannelDTO{}, err
	}
	var current *collectengine.Channel
	for index := range channels {
		if channels[index].Id == req.Id {
			current = &channels[index]
		}
		if channels[index].ChannelCode == name && channels[index].Id != req.Id {
			return dtos.CollectChannelDTO{}, errort.NewCommonErr(errort.DefaultReqParamsError,
				fmt.Errorf("通道名称 %s 已存在", name))
		}
	}
	if req.Id > 0 && current == nil {
		return dtos.CollectChannelDTO{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("通道不存在"))
	}

	advance := mergeChannelAdvance("", req.Config, channelType)
	if current != nil {
		advance = mergeChannelAdvance(current.Advance, req.Config, channelType)
	}
	comInterval := req.PollIntervalMs
	if comInterval < 1 {
		comInterval = 1000
	}
	retryCount := int64(3)
	if current != nil && current.RetryCount > 0 {
		retryCount = current.RetryCount
	}
	desired := collectengine.Channel{
		ChannelCode:  name,
		Protocol:     protocol,
		ComInterval:  int64(comInterval),
		PollInterval: int64(req.PollIntervalMs),
		RetryCount:   retryCount,
		Timeout:      int64(req.TimeoutMs),
		Advance:      advance,
		Disable:      !req.Enabled,
	}
	if current != nil {
		updated, err := client.UpdateChannel(current.Id, desired)
		if err != nil {
			return dtos.CollectChannelDTO{}, err
		}
		desired.Id = updated.Id
		if desired.Id <= 0 {
			desired.Id = current.Id
		}
		if err := s.saveChannelOwnership(ctx, desired, channelType); err != nil {
			return dtos.CollectChannelDTO{}, err
		}
		return channelDTO(desired), nil
	}
	created, err := client.CreateChannel(desired)
	if err != nil {
		return dtos.CollectChannelDTO{}, err
	}
	if created.Id == 0 {
		refreshed, err := client.ListChannels()
		if err != nil {
			return dtos.CollectChannelDTO{}, err
		}
		for _, item := range refreshed {
			if item.ChannelCode == name {
				created = item
				break
			}
		}
	}
	if created.Id <= 0 {
		return dtos.CollectChannelDTO{}, errort.NewCommonErr(errort.DefaultSystemError,
			fmt.Errorf("采集引擎未返回通道 ID"))
	}
	desired.Id = created.Id
	if err := s.saveChannelOwnership(ctx, desired, channelType); err != nil {
		return dtos.CollectChannelDTO{}, err
	}
	return channelDTO(desired), nil
}

func (s *service) CollectChannelDelete(ctx context.Context, id string) error {
	client, err := s.engineClient()
	if err != nil {
		return err
	}
	channelId, err := parsePositiveId(id, "channel id")
	if err != nil {
		return err
	}
	if ok, err := s.canAccessChannel(ctx, channelId); err != nil {
		return err
	} else if !ok {
		return errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("通道不存在或无权访问"))
	}
	devices, err := client.ListDevices(channelId)
	if err != nil {
		return err
	}
	if len(devices) > 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("通道下还有 %d 台设备，请先移除设备再删除", len(devices)))
	}
	if err := client.DeleteChannel(channelId); err != nil {
		return err
	}
	// 清理平台本地的设备点表缓存。
	db := s.dbClient.WithCtx(ctx)
	configs, err := db.CollectDeviceConfigsByChannel(strconv.FormatInt(channelId, 10))
	if err != nil {
		return err
	}
	for _, cfg := range configs {
		if err := db.ReplaceCollectPointConfigs(cfg.DeviceId, nil); err != nil {
			return err
		}
		if err := db.DeleteCollectDeviceConfig(cfg.DeviceId); err != nil {
			return err
		}
	}
	return db.DeleteCollectChannel(strconv.FormatInt(channelId, 10))
}

/******************************* IOTP 设备绑定 *******************************/

func (s *service) CollectDevices(ctx context.Context) ([]dtos.CollectDeviceDTO, error) {
	client, err := s.engineClient()
	if err != nil {
		return nil, err
	}
	bindings, err := client.ListDevices(0)
	if err != nil {
		return nil, err
	}
	bindings, err = s.filterAccessibleBindings(ctx, bindings)
	if err != nil {
		return nil, err
	}
	return s.deviceDTOs(ctx, bindings), nil
}

func (s *service) CollectChannelDevices(ctx context.Context, channelId string) ([]dtos.CollectDeviceDTO, error) {
	client, err := s.engineClient()
	if err != nil {
		return nil, err
	}
	id, err := parsePositiveId(channelId, "channel id")
	if err != nil {
		return nil, err
	}
	if ok, err := s.canAccessChannel(ctx, id); err != nil {
		return nil, err
	} else if !ok {
		return nil, errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("通道不存在或无权访问"))
	}
	bindings, err := client.ListDevices(id)
	if err != nil {
		return nil, err
	}
	return s.deviceDTOs(ctx, bindings), nil
}

func (s *service) CollectChannelBindDevices(ctx context.Context, channelId string, deviceIds []string) error {
	client, err := s.engineClient()
	if err != nil {
		return err
	}
	id, err := parsePositiveId(channelId, "channel id")
	if err != nil {
		return err
	}
	if len(deviceIds) == 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("请选择要添加的设备"))
	}
	if ok, err := s.canAccessChannel(ctx, id); err != nil {
		return err
	} else if !ok {
		return errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("通道不存在或无权访问"))
	}
	channels, err := client.ListChannels()
	if err != nil {
		return err
	}
	foundChannel := false
	for _, channel := range channels {
		if channel.Id == id {
			foundChannel = true
			break
		}
	}
	if !foundChannel {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("通道不存在"))
	}
	db := s.dbClient.WithCtx(ctx)
	bindings, err := client.ListDevices(0)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(deviceIds))
	for _, deviceId := range deviceIds {
		deviceId = strings.TrimSpace(deviceId)
		if deviceId == "" {
			continue
		}
		if _, ok := seen[deviceId]; ok {
			continue
		}
		seen[deviceId] = struct{}{}
		device, err := db.DeviceById(deviceId)
		if err != nil {
			return err
		}
		iotpId := iotp.DeviceID(device.Id)
		iotpKey := strconv.FormatInt(iotpId, 10)
		existingCfg, hadCfg, err := db.CollectDeviceConfigByDevice(iotpKey)
		if err != nil {
			return err
		}
		enabled := true
		remark := ""
		if hadCfg {
			enabled = existingCfg.Enabled
			remark = existingCfg.Remark
		}

		currentChannel := int64(0)
		currentDeviceId := int64(0)
		alreadyBound := false
		boundDeviceId := int64(0)
		for _, binding := range bindings {
			if binding.IotpId != iotpId {
				continue
			}
			if binding.ChannelId == id {
				alreadyBound = true
				boundDeviceId = binding.Id
				break
			}
			currentChannel = binding.ChannelId
			currentDeviceId = binding.Id
		}
		if alreadyBound {
			if err := db.SaveCollectDeviceConfig(models.CollectDeviceConfig{
				DeviceId:  iotpKey,
				ChannelId: strconv.FormatInt(id, 10),
				Enabled:   enabled,
				Remark:    remark,
			}); err != nil {
				return err
			}
			// 重新绑定（例如引擎数据被清理）时，把本地点位补回引擎。
			points, err := db.CollectPointConfigsByDevice(iotpKey)
			if err != nil {
				return err
			}
			if len(points) > 0 {
				if _, err := s.syncDeviceTags(client, db, iotpKey, boundDeviceId, points, points); err != nil {
					return err
				}
			}
			continue
		}
		// 同一个 IOTP 设备换通道：先删旧设备（级联删除旧点位），再建新设备。
		if currentDeviceId > 0 {
			if ok, err := s.canAccessChannel(ctx, currentChannel); err != nil {
				return err
			} else if !ok {
				return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("设备已绑定到其他通道"))
			}
			if err := client.DeleteDevice(currentDeviceId); err != nil {
				return err
			}
			bindings = removeDevice(bindings, currentDeviceId)
		}
		created, err := client.CreateDevice(id, iotpId)
		if err != nil {
			return err
		}
		engineDeviceId := created.Id
		if engineDeviceId == 0 {
			refreshed, err := client.ListDevices(id)
			if err != nil {
				return err
			}
			for _, item := range refreshed {
				if item.IotpId == iotpId {
					engineDeviceId = item.Id
					break
				}
			}
		}
		if engineDeviceId == 0 {
			return errort.NewCommonErr(errort.DefaultSystemError, fmt.Errorf("采集引擎未返回设备 ID"))
		}
		bindings = append(bindings, collectengine.Device{Id: engineDeviceId, ChannelId: id, IotpId: iotpId})
		if err := db.SaveCollectDeviceConfig(models.CollectDeviceConfig{
			DeviceId:  iotpKey,
			ChannelId: strconv.FormatInt(id, 10),
			Enabled:   enabled,
			Remark:    remark,
		}); err != nil {
			return err
		}

		points, err := db.CollectPointConfigsByDevice(iotpKey)
		if err != nil {
			return err
		}
		if len(points) == 0 {
			continue
		}
		// 换通道时旧通道的引擎点位要迁走；新建绑定时则把本地点位补到新通道。
		if currentChannel > 0 {
			if err := db.ClearCollectPointRemoteIds(iotpKey); err != nil {
				return err
			}
			points, err = db.CollectPointConfigsByDevice(iotpKey)
			if err != nil {
				return err
			}
		}
		if _, err := s.syncDeviceTags(client, db, iotpKey, engineDeviceId, nil, points); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) CollectChannelUnbindDevice(ctx context.Context, channelId, deviceId string) error {
	client, err := s.engineClient()
	if err != nil {
		return err
	}
	channelIdValue, err := parsePositiveId(channelId, "channel id")
	if err != nil {
		return err
	}
	iotpId, err := parsePositiveId(deviceId, "iotp id")
	if err != nil {
		return err
	}
	if ok, err := s.canAccessChannel(ctx, channelIdValue); err != nil {
		return err
	} else if !ok {
		return errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("通道不存在或无权访问"))
	}
	bindings, err := client.ListDevices(channelIdValue)
	if err != nil {
		return err
	}
	engineDeviceId := int64(0)
	for _, binding := range bindings {
		if binding.IotpId == iotpId {
			engineDeviceId = binding.Id
			break
		}
	}
	if engineDeviceId == 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("设备绑定不存在"))
	}
	// 删除设备会级联删除引擎点位。
	if err := client.DeleteDevice(engineDeviceId); err != nil {
		return err
	}
	iotpKey := strconv.FormatInt(iotpId, 10)
	db := s.dbClient.WithCtx(ctx)
	// 移除设备 = 连带删除本地点位配置；重新添加时重新按物模型生成模板。
	if err := db.ReplaceCollectPointConfigs(iotpKey, nil); err != nil {
		return err
	}
	return db.DeleteCollectDeviceConfig(iotpKey)
}

/******************************* 设备点表 *******************************/

func (s *service) CollectConfigGet(ctx context.Context, deviceId string) (dtos.CollectConfigResponse, error) {
	response := dtos.CollectConfigResponse{Points: []dtos.CollectPointDTO{}}
	client, err := s.engineClient()
	if err != nil {
		return response, err
	}
	iotpId, err := parsePositiveId(deviceId, "device id")
	if err != nil {
		return response, err
	}
	binding, channel, found, err := s.bindingForIotp(client, iotpId)
	if err != nil {
		return response, err
	}
	if !found {
		return response, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("设备未绑定通道"))
	}
	if ok, err := s.canAccessChannel(ctx, binding.ChannelId); err != nil {
		return response, err
	} else if !ok {
		return response, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("设备未绑定通道"))
	}
	local, product := s.resolveIotpDevice(ctx, iotpId)
	name := local.Name
	if strings.TrimSpace(name) == "" {
		name = fmt.Sprintf("#%d", iotpId)
	}
	response.DeviceId = iotpId
	response.DeviceName = name
	response.ProductId = local.ProductId
	response.ProductName = product.Name
	response.ChannelId = binding.ChannelId
	response.ChannelName = channel.ChannelCode
	response.Enabled = true

	iotpKey := strconv.FormatInt(iotpId, 10)
	db := s.dbClient.WithCtx(ctx)
	if cfg, cfgFound, err := db.CollectDeviceConfigByDevice(iotpKey); err != nil {
		return response, err
	} else if cfgFound {
		response.Enabled = cfg.Enabled
	}
	points, err := db.CollectPointConfigsByDevice(iotpKey)
	if err != nil {
		return response, err
	}
	if len(points) > 0 {
		response.Points = make([]dtos.CollectPointDTO, 0, len(points))
		for index, point := range points {
			response.Points = append(response.Points, localPointDTO(point, index))
		}
		return response, nil
	}

	// 首次打开：按系统设备的物模型属性生成模板。
	response.Points = make([]dtos.CollectPointDTO, 0, len(product.Properties))
	for index, property := range product.Properties {
		response.Points = append(response.Points, dtos.CollectPointDTO{
			TagId:      property.Code,
			Address:    "",
			DataType:   templateDataType(property.TypeSpec.Type),
			AccessMode: firstNonEmpty(property.AccessMode, "R"),
			Multiplier: 1,
			Offset:     0,
			Storage:    true,
			Enabled:    true,
			Sort:       index,
		})
	}
	return response, nil
}

func (s *service) CollectConfigTemplate(ctx context.Context, deviceId string) ([]dtos.CollectPointDTO, error) {
	result := make([]dtos.CollectPointDTO, 0)
	iotpId, err := parsePositiveId(deviceId, "device id")
	if err != nil {
		return result, err
	}
	// 模板读取同样要校验通道归属，避免跨租户按 iotpId 读取产品物模型。
	client, err := s.engineClient()
	if err != nil {
		return result, err
	}
	binding, _, found, err := s.bindingForIotp(client, iotpId)
	if err != nil {
		return result, err
	}
	if !found {
		return result, nil
	}
	if ok, err := s.canAccessChannel(ctx, binding.ChannelId); err != nil {
		return result, err
	} else if !ok {
		return result, nil
	}
	local, product := s.resolveIotpDevice(ctx, iotpId)
	if local.Id == "" || product.Id == "" {
		return result, nil
	}
	result = make([]dtos.CollectPointDTO, 0, len(product.Properties))
	for index, property := range product.Properties {
		result = append(result, dtos.CollectPointDTO{
			TagId:      property.Code,
			Address:    "",
			DataType:   templateDataType(property.TypeSpec.Type),
			AccessMode: firstNonEmpty(property.AccessMode, "R"),
			Multiplier: 1,
			Offset:     0,
			Storage:    true,
			Enabled:    true,
			Sort:       index,
		})
	}
	return result, nil
}

// CollectConfigLatest 读取设备在时序库中的最新值，供采集页面监控模式使用。
func (s *service) CollectConfigLatest(ctx context.Context, deviceId string) (dtos.CollectMonitorResponse, error) {
	response := dtos.CollectMonitorResponse{Items: []dtos.CollectMonitorValueDTO{}}
	iotpId, err := parsePositiveId(deviceId, "device id")
	if err != nil {
		return response, err
	}
	db := s.dbClient.WithCtx(ctx)
	iotpKey := strconv.FormatInt(iotpId, 10)
	points, err := db.CollectPointConfigsByDevice(iotpKey)
	if err != nil {
		return response, err
	}
	if len(points) == 0 {
		response.Message = "还没有保存点位"
		return response, nil
	}
	cfg, found, err := db.CollectDeviceConfigByDevice(iotpKey)
	if err != nil {
		return response, err
	}
	if !found || strings.TrimSpace(cfg.ChannelId) == "" {
		return response, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("设备未绑定通道"))
	}
	channelId, err := parsePositiveId(cfg.ChannelId, "channel id")
	if err != nil {
		return response, err
	}
	if ok, err := s.canAccessChannel(ctx, channelId); err != nil {
		return response, err
	} else if !ok {
		return response, errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("通道不存在或无权访问"))
	}
	if s.iotpClient == nil {
		response.Message = "未启用时序库，无法查看实时数据"
		return response, nil
	}
	realtime, err := s.iotpClient.Realtime(ctx, iotpId)
	if err != nil {
		var apiErr *iotp.APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			response.Message = "时序库中还没有该设备，尚未上报数据"
			return response, nil
		}
		return response, err
	}
	response.Available = true
	if len(realtime.Fields) == 0 {
		response.Message = "该设备还没有上报过数据"
	}
	fields := make(map[string]iotp.RealtimeField, len(realtime.Fields))
	for _, field := range realtime.Fields {
		fields[field.Name] = field
	}
	updatedAt := int64(0)
	if realtime.UpdatedAt != nil {
		updatedAt = *realtime.UpdatedAt
	}
	staleAfter := 3 * s.channelPollInterval(ctx, channelId)
	if staleAfter < 15000 {
		staleAfter = 15000
	}
	now := time.Now().UnixMilli()
	for _, point := range points {
		item := dtos.CollectMonitorValueDTO{TagId: point.TagId, DataType: point.DataType, Status: "no_data"}
		if !point.Enabled {
			item.Status = "disabled"
			response.Items = append(response.Items, item)
			continue
		}
		if field, ok := fields[point.TagId]; ok {
			ts := field.Time
			if ts == 0 {
				ts = updatedAt
			}
			item.Value = field.Value
			item.Time = ts
			if ts > 0 && now-ts > staleAfter {
				item.Status = "stale"
			} else {
				item.Status = "normal"
			}
		}
		response.Items = append(response.Items, item)
	}
	return response, nil
}

// channelPollInterval 读取通道轮询周期（毫秒）。
// 用本地通道登记表而不是引擎接口：采集引擎不可达时不能把实时监控一起拖住。
func (s *service) channelPollInterval(ctx context.Context, channelId int64) int64 {
	channels, err := s.dbClient.WithCtx(ctx).CollectChannels()
	if err != nil {
		return 0
	}
	key := strconv.FormatInt(channelId, 10)
	for _, channel := range channels {
		if channel.Id == key && channel.PollIntervalMs > 0 {
			return int64(channel.PollIntervalMs)
		}
	}
	return 0
}

func (s *service) CollectConfigSave(ctx context.Context, req dtos.CollectConfigSaveRequest) error {
	client, err := s.engineClient()
	if err != nil {
		return err
	}
	if req.DeviceId <= 0 {
		return errort.NewCommonErr(errort.DefaultIdEmpty, fmt.Errorf("device id is required"))
	}
	binding, _, found, err := s.bindingForIotp(client, req.DeviceId)
	if err != nil {
		return err
	}
	if !found {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("设备未绑定通道"))
	}
	if ok, err := s.canAccessChannel(ctx, binding.ChannelId); err != nil {
		return err
	} else if !ok {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("设备未绑定通道"))
	}
	iotpKey := strconv.FormatInt(req.DeviceId, 10)
	db := s.dbClient.WithCtx(ctx)
	existing, err := db.CollectPointConfigsByDevice(iotpKey)
	if err != nil {
		return err
	}
	existingByTag := make(map[string]models.CollectPointConfig, len(existing))
	for _, point := range existing {
		existingByTag[point.TagId] = point
	}

	seenTags := make(map[string]struct{}, len(req.Points))
	points := make([]models.CollectPointConfig, 0, len(req.Points))
	for index, point := range req.Points {
		tagId := strings.TrimSpace(point.TagId)
		if tagId == "" {
			return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("第 %d 行标签ID不能为空", index+1))
		}
		if _, ok := seenTags[tagId]; ok {
			return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("标签ID %s 重复", tagId))
		}
		seenTags[tagId] = struct{}{}
		address := strings.TrimSpace(point.Address)
		if address == "" {
			return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("第 %d 行（%s）地址不能为空", index+1, tagId))
		}
		dataType := normalizeDataType(point.DataType)
		if dataType == "" {
			return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("第 %d 行（%s）数据类型不能为空", index+1, tagId))
		}
		accessMode := firstNonEmpty(strings.ToUpper(strings.TrimSpace(point.AccessMode)), "R")
		switch accessMode {
		case "R", "W", "RW":
		default:
			return errort.NewCommonErr(errort.DefaultReqParamsError,
				fmt.Errorf("第 %d 行（%s）读写类型 %s 不支持", index+1, tagId, accessMode))
		}
		localId := ""
		remoteId := int64(0)
		if previous, ok := existingByTag[tagId]; ok {
			localId = previous.Id
			// 未使能的点位不下发引擎；清掉远端 ID，让同步逻辑删除旧 tag。
			if point.Enabled {
				remoteId = previous.RemoteId
			}
		}
		sortIndex := point.Sort
		if sortIndex == 0 {
			sortIndex = index
		}
		points = append(points, models.CollectPointConfig{
			Id:         localId,
			DeviceId:   iotpKey,
			TagId:      tagId,
			RemoteId:   remoteId,
			Address:    address,
			DataType:   dataType,
			AccessMode: accessMode,
			Multiplier: point.Multiplier,
			Offset:     point.Offset,
			Storage:    point.Storage,
			Enabled:    point.Enabled,
			Sort:       sortIndex,
		})
	}

	remark := ""
	if cfg, cfgFound, err := db.CollectDeviceConfigByDevice(iotpKey); err != nil {
		return err
	} else if cfgFound {
		remark = cfg.Remark
	}
	if err := db.SaveCollectDeviceConfig(models.CollectDeviceConfig{
		DeviceId:  iotpKey,
		ChannelId: strconv.FormatInt(binding.ChannelId, 10),
		Enabled:   req.Enabled,
		Remark:    remark,
	}); err != nil {
		return err
	}
	if err := db.ReplaceCollectPointConfigs(iotpKey, points); err != nil {
		return err
	}
	// 设备禁用时把点位从引擎移除，重新启用保存后再补回去；本地点位保留。
	if !req.Enabled {
		if err := s.deleteDeviceTags(client, binding.Id, existing); err != nil {
			return err
		}
		return db.ClearCollectPointRemoteIds(iotpKey)
	}
	_, err = s.syncDeviceTags(client, db, iotpKey, binding.Id, existing, points)
	return err
}

func removeDevice(devices []collectengine.Device, id int64) []collectengine.Device {
	for index := range devices {
		if devices[index].Id == id {
			return append(devices[:index], devices[index+1:]...)
		}
	}
	return devices
}

// bindingForIotp 找到 IOTP 设备当前绑定的通道。
func (s *service) bindingForIotp(client *collectengine.Client, iotpId int64) (collectengine.Device, collectengine.Channel, bool, error) {
	bindings, err := client.ListDevices(0)
	if err != nil {
		return collectengine.Device{}, collectengine.Channel{}, false, err
	}
	var binding *collectengine.Device
	for index := range bindings {
		if bindings[index].IotpId == iotpId {
			binding = &bindings[index]
			break
		}
	}
	if binding == nil {
		return collectengine.Device{}, collectengine.Channel{}, false, nil
	}
	channels, err := client.ListChannels()
	if err != nil {
		return collectengine.Device{}, collectengine.Channel{}, false, err
	}
	for _, channel := range channels {
		if channel.Id == binding.ChannelId {
			return *binding, channel, true, nil
		}
	}
	return collectengine.Device{}, collectengine.Channel{}, false, nil
}

// pointRemoteUpdater 只依赖回写 remote_id 的能力，便于同步逻辑单测。
type pointRemoteUpdater interface {
	UpdateCollectPointRemoteIds(deviceId string, remoteIds map[string]int64) error
}

// syncDeviceTags 把某台设备的本地点位同步到通道的引擎点位：
// 有远端 ID 的更新，没有的创建，旧点位删除，只动这台设备自己的点位。
func (s *service) syncDeviceTags(client *collectengine.Client, db pointRemoteUpdater, localDeviceId string, deviceId int64, oldPoints, newPoints []models.CollectPointConfig) (map[string]int64, error) {
	tags, err := client.ListTags()
	if err != nil {
		return nil, err
	}
	remoteById := make(map[int64]collectengine.Tag)
	for _, tag := range tags {
		if tag.DeviceId == deviceId {
			remoteById[tag.Id] = tag
		}
	}
	oldRemoteIds := make(map[int64]struct{}, len(oldPoints))
	for _, point := range oldPoints {
		if point.RemoteId > 0 {
			oldRemoteIds[point.RemoteId] = struct{}{}
		}
	}
	keptIds := make(map[int64]struct{}, len(newPoints))
	mapping := make(map[string]int64, len(newPoints))
	missing := make(map[string]string)
	for _, point := range newPoints {
		// 未使能的点位保留在平台本地，但不创建/更新引擎 tag。
		if !point.Enabled {
			continue
		}
		desired := collectengine.Tag{
			DeviceId:    deviceId,
			TagCode:     point.TagId,
			Address:     point.Address,
			DataType:    point.DataType,
			ReadWrite:   firstNonEmpty(point.AccessMode, "R"),
			ScaleOffset: scaleOffset(point.Multiplier, point.Offset),
			Storage:     point.Storage,
		}
		remoteId := point.RemoteId
		if remoteId > 0 {
			if remote, ok := remoteById[remoteId]; ok {
				// 内容没变就不下发，避免点表有多少行就调用引擎多少次。
				if !tagMatches(remote, desired) {
					updated, err := client.UpdateTag(remoteId, desired)
					if err != nil {
						return nil, err
					}
					if updated.Id > 0 {
						remoteId = updated.Id
					}
				}
			} else {
				remoteId = 0
			}
		}
		if remoteId == 0 {
			created, err := client.CreateTag(desired)
			if err != nil {
				return nil, err
			}
			remoteId = created.Id
		}
		if remoteId == 0 {
			// 新建接口没回传 ID 时立刻重查一次，避免后续失败时丢失已创建点位的 ID。
			remoteId = s.findTagIdByCode(client, deviceId, point.TagId, keptIds)
		}
		if remoteId > 0 {
			keptIds[remoteId] = struct{}{}
			// 未变化的点位本地点表里已有 remote_id，不必重复写库。
			if point.Id != "" && remoteId != point.RemoteId {
				mapping[point.Id] = remoteId
				if err := db.UpdateCollectPointRemoteIds(localDeviceId, map[string]int64{point.Id: remoteId}); err != nil {
					return nil, err
				}
			}
		} else if point.Id != "" {
			missing[point.Id] = point.TagId
		}
	}
	for id := range oldRemoteIds {
		if _, ok := keptIds[id]; ok {
			continue
		}
		if _, ok := remoteById[id]; !ok {
			continue
		}
		if err := client.DeleteTag(id); err != nil {
			return nil, err
		}
	}
	if len(missing) > 0 {
		refreshed, err := client.ListTags()
		if err != nil {
			return nil, err
		}
		byCode := make(map[string][]collectengine.Tag)
		for _, tag := range refreshed {
			if tag.DeviceId == deviceId {
				byCode[tag.TagCode] = append(byCode[tag.TagCode], tag)
			}
		}
		for localId, code := range missing {
			for _, tag := range byCode[code] {
				if _, used := keptIds[tag.Id]; used {
					continue
				}
				if tag.Id > 0 {
					keptIds[tag.Id] = struct{}{}
					mapping[localId] = tag.Id
					if err := db.UpdateCollectPointRemoteIds(localDeviceId, map[string]int64{localId: tag.Id}); err != nil {
						return nil, err
					}
					break
				}
			}
		}
	}
	return mapping, nil
}

func tagMatches(remote, desired collectengine.Tag) bool {
	return remote.DeviceId == desired.DeviceId &&
		remote.TagCode == desired.TagCode &&
		remote.Address == desired.Address &&
		remote.DataType == desired.DataType &&
		remote.ReadWrite == desired.ReadWrite &&
		remote.ScaleOffset == desired.ScaleOffset &&
		remote.Storage == desired.Storage
}

func (s *service) findTagIdByCode(client *collectengine.Client, deviceId int64, tagCode string, used map[int64]struct{}) int64 {
	tags, err := client.ListTags()
	if err != nil {
		return 0
	}
	for _, tag := range tags {
		if tag.DeviceId != deviceId || tag.TagCode != tagCode {
			continue
		}
		if _, ok := used[tag.Id]; ok {
			continue
		}
		return tag.Id
	}
	return 0
}

// deleteDeviceTags 删除某台设备在通道里的全部引擎点位，只按本地记录的远端 ID 删除。
func (s *service) deleteDeviceTags(client *collectengine.Client, deviceId int64, points []models.CollectPointConfig) error {
	ids := make(map[int64]struct{}, len(points))
	for _, point := range points {
		if point.RemoteId > 0 {
			ids[point.RemoteId] = struct{}{}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	tags, err := client.ListTags()
	if err != nil {
		return err
	}
	for _, tag := range tags {
		if tag.DeviceId != deviceId {
			continue
		}
		if _, ok := ids[tag.Id]; !ok {
			continue
		}
		if err := client.DeleteTag(tag.Id); err != nil {
			return err
		}
	}
	return nil
}

/******************************* 映射与工具 *******************************/

func (s *service) deviceDTOs(ctx context.Context, bindings []collectengine.Device) []dtos.CollectDeviceDTO {
	result := make([]dtos.CollectDeviceDTO, 0, len(bindings))
	for _, binding := range bindings {
		local, product := s.resolveIotpDevice(ctx, binding.IotpId)
		name := local.Name
		if strings.TrimSpace(name) == "" {
			name = fmt.Sprintf("#%d", binding.IotpId)
		}
		result = append(result, dtos.CollectDeviceDTO{
			IotpId:      binding.IotpId,
			ChannelId:   binding.ChannelId,
			DeviceId:    local.Id,
			DeviceName:  name,
			ProductId:   local.ProductId,
			ProductName: product.Name,
		})
	}
	return result
}

func (s *service) resolveIotpDevice(ctx context.Context, iotpId int64) (models.Device, models.Product) {
	if iotpId <= 0 {
		return models.Device{}, models.Product{}
	}
	// 平台设备 ID 是 %08d；转成 int64 后可能丢掉前导零，反查时两种形式都要试。
	db := s.dbClient.WithCtx(ctx)
	candidates := []string{strconv.FormatInt(iotpId, 10), fmt.Sprintf("%08d", iotpId)}
	var local models.Device
	found := false
	for _, id := range candidates {
		device, err := db.DeviceById(id)
		if err == nil {
			local = device
			found = true
			break
		}
	}
	if !found {
		return models.Device{}, models.Product{}
	}
	product, err := db.ProductById(local.ProductId)
	if err != nil {
		return local, models.Product{}
	}
	return local, product
}

func channelDTO(channel collectengine.Channel) dtos.CollectChannelDTO {
	config := map[string]any{}
	if strings.TrimSpace(channel.Advance) != "" {
		_ = json.Unmarshal([]byte(channel.Advance), &config)
	}
	channelType, _ := config["channelType"].(string)
	delete(config, "channelType")
	if channelType == "" {
		channelType = inferChannelType(channel.Protocol)
	}
	poll := channel.PollInterval
	if poll <= 0 {
		poll = channel.ComInterval
	}
	return dtos.CollectChannelDTO{
		Id:             channel.Id,
		Name:           channel.ChannelCode,
		ChannelType:    channelType,
		Protocol:       channel.Protocol,
		Config:         config,
		PollIntervalMs: int(poll),
		TimeoutMs:      int(channel.Timeout),
		Enabled:        !channel.Disable,
	}
}

func localPointDTO(point models.CollectPointConfig, index int) dtos.CollectPointDTO {
	return dtos.CollectPointDTO{
		Id:         point.RemoteId,
		TagId:      point.TagId,
		Address:    point.Address,
		DataType:   normalizeDataType(point.DataType),
		AccessMode: firstNonEmpty(point.AccessMode, "R"),
		Multiplier: point.Multiplier,
		Offset:     point.Offset,
		Storage:    point.Storage,
		Enabled:    point.Enabled,
		Sort:       index,
	}
}

func mergeChannelAdvance(existing string, config map[string]any, channelType string) string {
	advance := map[string]any{}
	if strings.TrimSpace(existing) != "" {
		_ = json.Unmarshal([]byte(existing), &advance)
	}
	for key, value := range config {
		advance[key] = value
	}
	advance["channelType"] = channelType
	raw, err := json.Marshal(advance)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func inferChannelType(protocol string) string {
	value := strings.ToLower(protocol)
	switch {
	case strings.Contains(value, "rtu"), strings.Contains(value, "serial"), strings.Contains(value, "com"):
		return "serialport"
	default:
		return "network"
	}
}

func scaleOffset(multiplier, offset float64) string {
	return strconv.FormatFloat(multiplier, 'f', -1, 64) + "/" + strconv.FormatFloat(offset, 'f', -1, 64)
}

func parsePositiveId(value, name string) (int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, errort.NewCommonErr(errort.DefaultIdEmpty, fmt.Errorf("%s is required", name))
	}
	id, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || id <= 0 {
		return 0, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("%s 不合法", name))
	}
	return id, nil
}

func validateChannelConfig(channelType string, config map[string]any) error {
	switch channelType {
	case "network":
		if configString(config, "ip") == "" {
			return fmt.Errorf("网口通道需要填写 IP 地址")
		}
		if configInt(config, "port") <= 0 {
			return fmt.Errorf("网口通道需要填写端口")
		}
	case "serialport":
		if configString(config, "com") == "" {
			return fmt.Errorf("串口通道需要填写串口")
		}
	}
	return nil
}

func configString(config map[string]any, key string) string {
	if config == nil {
		return ""
	}
	value, _ := config[key].(string)
	return strings.TrimSpace(value)
}

func configInt(config map[string]any, key string) int {
	if config == nil {
		return 0
	}
	switch value := config[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case int64:
		return int(value)
	case json.Number:
		n, _ := value.Int64()
		return int(n)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(value))
		return n
	default:
		return 0
	}
}

func templateDataType(spec constants.SpecsType) string {
	switch spec {
	case constants.SpecsTypeInt, constants.SpecsTypeEnum:
		return "int16"
	case constants.SpecsTypeFloat:
		return "float"
	case constants.SpecsTypeBool:
		return "bool"
	case constants.SpecsTypeDate:
		return "int64"
	case constants.SpecsTypeText, constants.SpecsTypeStruct, constants.SpecsTypeArray:
		return "string"
	default:
		return "int16"
	}
}

// normalizeDataType 统一采集数据类型的命名：float32 → float，float64 → double。
func normalizeDataType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "float32":
		return "float"
	case "float64":
		return "double"
	}
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
