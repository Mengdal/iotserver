package driverapp

import (
	"context"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/config"
	"iotserver/internal/iotserver/core/container"
	"iotserver/internal/models"
	"iotserver/internal/pkg/seed"
)

// SystemComponents 返回内置系统组件及其运行状态。
// Phase 1 只做展示与探活，启停/配置由后续的组件运行时接管。
func (app *driverLibApp) SystemComponents(ctx context.Context) ([]dtos.SystemComponentDTO, error) {
	libraries, _, err := app.dbClient.WithCtx(ctx).DeviceLibrariesSearch(0, -1, dtos.DeviceLibrarySearchQueryRequest{
		Category: models.PluginCategorySystem,
	})
	if err != nil {
		return nil, err
	}
	cfg := container.ConfigurationFrom(app.dic.Get)
	endpoints := map[string]string{
		seed.SystemComponentMosquittoId: componentEndpoint(cfg, seed.SystemComponentMosquittoId),
		seed.SystemComponentEKuiperId:   componentEndpoint(cfg, seed.SystemComponentEKuiperId),
		seed.SystemComponentIotEdgeDBId: componentEndpoint(cfg, seed.SystemComponentIotEdgeDBId),
	}
	result := make([]dtos.SystemComponentDTO, 0, len(libraries))
	for _, library := range libraries {
		endpoint := endpoints[library.Id]
		result = append(result, dtos.SystemComponentDTO{
			Id:          library.Id,
			Name:        library.Name,
			Version:     library.Version,
			Description: library.Description,
			Runtime:     library.Runtime,
			Essential:   library.Essential,
			Endpoint:    endpoint,
			Status:      probeComponentEndpoint(endpoint),
			Updated:     library.Modified,
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Essential != result[j].Essential {
			return result[i].Essential
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// componentEndpoint 从平台配置读取组件实际监听地址。
// Mosquitto 使用内部消息总线配置；eKuiper 使用 Clients["Ekuiper"]；
// IotEdgeDB 是外部时序库，使用 [Databases.Data].BaseURL。
func componentEndpoint(cfg *config.ConfigurationStruct, componentId string) string {
	if cfg == nil {
		return ""
	}
	switch componentId {
	case seed.SystemComponentMosquittoId:
		if strings.TrimSpace(cfg.MessageQueue.Host) == "" {
			return ""
		}
		return cfg.MessageQueue.URL()
	case seed.SystemComponentEKuiperId:
		client, ok := cfg.Clients["Ekuiper"]
		if !ok || strings.TrimSpace(client.Host) == "" {
			return ""
		}
		if address := client.Address(); address != "" {
			return address
		}
		return client.Url()
	case seed.SystemComponentIotEdgeDBId:
		return strings.TrimSpace(cfg.GetDataDatabaseInfo().BaseURL)
	}
	return ""
}

// probeComponentEndpoint 只做一次短超时 TCP 探活，避免系统组件页面被远端拖住。
func probeComponentEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "unknown"
	}
	target := endpoint
	if !strings.Contains(target, "://") {
		target = "tcp://" + target
	}
	parsed, err := url.Parse(target)
	if err != nil || parsed.Host == "" {
		return "unknown"
	}
	conn, err := net.DialTimeout("tcp", parsed.Host, 500*time.Millisecond)
	if err != nil {
		return "stopped"
	}
	_ = conn.Close()
	return "running"
}
