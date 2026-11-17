package driverapp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/config"
	"iotserver/internal/iotserver/core/container"
	"iotserver/internal/models"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/seed"
	"iotserver/internal/tools/iotp"
	"iotserver/internal/tools/logtail"
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
		// iotpGW 的地址来自软网关记录（每个软网关可以接不同的采集引擎）。
		seed.SystemComponentIotpGWId: app.softGatewayEngineEndpoint(ctx),
	}
	logSources := componentLogSources(cfg)
	result := make([]dtos.SystemComponentDTO, 0, len(libraries))
	for _, library := range libraries {
		endpoint := endpoints[library.Id]
		logSource, hasLog := logSources[library.Id]
		logPath := ""
		if hasLog {
			// 按天轮转的日志（如 iotp.log.*）展示当前实际读取的文件。
			logPath = logtail.Resolve(logSource.Path)
			if logPath == "" {
				logPath = logSource.Path
			}
		}
		// IotEdgeDB 是远程服务，没有本机日志文件；配置了 BaseURL 时
		// 通过它的「变更记录」接口提供日志查看。管理员在 ComponentLogs
		// 里显式配置（或显式置空禁用）时以配置为准。
		if library.Id == seed.SystemComponentIotEdgeDBId {
			if _, explicit := explicitComponentLogSource(cfg, library.Id); !explicit {
				if base := iotEdgeDBBaseURL(cfg); base != "" {
					hasLog = true
					logPath = base + "/api/v1/meta/auth/audit"
				}
			}
		}
		// iotpGW 是 HTTP 采集引擎，直接请求 URL 比只看端口更准确；
		// 其余组件仍用 TCP 探活。只探测一次，避免先 TCP 再 HTTP 白等一轮超时。
		var status string
		if library.Id == seed.SystemComponentIotpGWId {
			status = probeHTTPEndpoint(endpoint)
		} else {
			status = probeComponentEndpoint(endpoint)
		}
		result = append(result, dtos.SystemComponentDTO{
			Id:            library.Id,
			Name:          library.Name,
			Version:       library.Version,
			Description:   library.Description,
			Runtime:       library.Runtime,
			Essential:     library.Essential,
			Endpoint:      endpoint,
			Status:        status,
			LogConfigured: hasLog,
			LogPath:       logPath,
			Updated:       library.Modified,
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

// componentLogSources 返回系统组件日志源：内置默认路径 + 配置覆盖。
// 配置里 Path 为空表示禁用该组件的日志入口。
// explicitComponentLogSource 返回管理员在 [ComponentLogs].Sources 里显式配置的日志源。
// 需要区分「没配置」和「显式置空禁用」时用它，componentLogSources 会丢掉空路径项。
func explicitComponentLogSource(cfg *config.ConfigurationStruct, id string) (config.ComponentLogSource, bool) {
	if cfg == nil {
		return config.ComponentLogSource{}, false
	}
	for _, item := range cfg.ComponentLogs.Sources {
		if strings.TrimSpace(item.Id) == id {
			return item, true
		}
	}
	return config.ComponentLogSource{}, false
}

func componentLogSources(cfg *config.ConfigurationStruct) map[string]config.ComponentLogSource {
	sources := map[string]config.ComponentLogSource{
		seed.SystemComponentMosquittoId: {
			Id:   seed.SystemComponentMosquittoId,
			Name: "Mosquitto",
			Path: "/var/log/iotserver/mosquitto.log",
		},
		seed.SystemComponentEKuiperId: {
			Id:   seed.SystemComponentEKuiperId,
			Name: "eKuiper",
			Path: "/opt/iotserver/ekuiper/log/stream.log",
		},
		seed.SystemComponentIotpGWId: {
			Id:   seed.SystemComponentIotpGWId,
			Name: "iotpGW",
			Path: "/var/log/iotserver/iotpgw.log",
		},
	}
	if cfg == nil {
		return sources
	}
	for _, item := range cfg.ComponentLogs.Sources {
		id := strings.TrimSpace(item.Id)
		if id == "" {
			continue
		}
		if strings.TrimSpace(item.Path) == "" {
			delete(sources, id)
			continue
		}
		if strings.TrimSpace(item.Name) == "" {
			item.Name = id
		}
		sources[id] = item
	}
	return sources
}

// ComponentLogs 读取某个系统组件的日志尾部。
// 文件不存在时返回 Exists=false，由前端提示配置路径；不把路径是否存在当成接口错误。
func (app *driverLibApp) ComponentLogs(ctx context.Context, id string, tail int) (dtos.SystemComponentLogsDTO, error) {
	cfg := container.ConfigurationFrom(app.dic.Get)
	// IotEdgeDB 默认走远程审计接口；显式配置了日志源时按配置读文件
	// （空 Path 表示管理员主动禁用日志入口）。
	if id == seed.SystemComponentIotEdgeDBId {
		if _, explicit := explicitComponentLogSource(cfg, id); !explicit {
			return app.iotEdgeDBAuditLogs(ctx, cfg, tail)
		}
	}
	source, ok := componentLogSources(cfg)[id]
	if !ok {
		return dtos.SystemComponentLogsDTO{}, errort.NewCommonEdgeX(
			errort.DefaultResourcesNotFound, fmt.Sprintf("组件 %s 未配置日志文件", id), nil)
	}
	info := logtail.Stat(source.Path)
	result := dtos.SystemComponentLogsDTO{
		ComponentId: id,
		Path:        info.Path,
		Exists:      info.Exists,
		Size:        info.Size,
		Modified:    info.Modified,
		Lines:       []string{},
	}
	if !info.Exists {
		return result, nil
	}
	// 用 Stat 已解析出的具体文件，避免再次解析通配符时日志轮转导致
	// 返回的路径/大小与行内容来自不同文件。
	lines, err := logtail.TailFile(info.Path, tail)
	if err != nil {
		if errors.Is(err, logtail.ErrNotFound) {
			return result, nil
		}
		return result, errort.NewCommonEdgeX(errort.DefaultSystemError, "read component log failed", err)
	}
	result.Lines = lines
	return result, nil
}

// iotEdgeDBAuditLogs 读取远程 IotEdgeDB 的「变更记录」作为组件日志。
// 该服务没有本机日志文件，控制台 #audit 页与 /api/v1/meta/auth/audit 同源。
func (app *driverLibApp) iotEdgeDBAuditLogs(ctx context.Context, cfg *config.ConfigurationStruct, tail int) (dtos.SystemComponentLogsDTO, error) {
	result := dtos.SystemComponentLogsDTO{
		ComponentId: seed.SystemComponentIotEdgeDBId,
		Lines:       []string{},
	}
	if cfg == nil {
		return result, errort.NewCommonEdgeX(errort.DefaultResourcesNotFound, "配置未加载", nil)
	}
	db := cfg.GetDataDatabaseInfo()
	base := iotEdgeDBBaseURL(cfg)
	if base == "" {
		return result, errort.NewCommonEdgeX(errort.DefaultResourcesNotFound,
			"IotEdgeDB 未配置 [Databases.Data].BaseURL", nil)
	}
	result.Path = base + "/api/v1/meta/auth/audit"
	client := iotp.New(base, db.Token, db.IotpDatabaseName(), db.IotpTimeout())
	entries, err := client.AuditLogs(ctx, tail)
	if err != nil {
		return result, errort.NewCommonEdgeX(errort.DefaultSystemError, "读取 IotEdgeDB 变更记录失败", err)
	}
	result.Exists = true
	// 接口按最新在前返回；日志窗口按时间顺序追加，所以反转。
	for i := len(entries) - 1; i >= 0; i-- {
		result.Lines = append(result.Lines, formatAuditEntry(entries[i]))
	}
	return result, nil
}

func iotEdgeDBBaseURL(cfg *config.ConfigurationStruct) string {
	return strings.TrimRight(strings.TrimSpace(componentEndpoint(cfg, seed.SystemComponentIotEdgeDBId)), "/")
}

// formatAuditEntry 把一条审计记录压成一行文本。
// 字段名在不同 iotp 版本间略有差异，这里按常见别名取值；全都不认识时退回 JSON。
func formatAuditEntry(entry map[string]any) string {
	parts := make([]string, 0, 5)
	for _, keys := range [][]string{
		{"timestamp", "time", "ts", "at", "created_at", "created"},
		{"actor", "user", "subject", "principal", "operator"},
		{"event", "event_type", "action", "type", "eventType"},
		{"target", "resource", "object", "path"},
		{"detail", "message", "msg", "description"},
	} {
		if value := firstAuditValue(entry, keys...); value != "" {
			parts = append(parts, value)
		}
	}
	if len(parts) == 0 {
		if raw, err := json.Marshal(entry); err == nil {
			return string(raw)
		}
		return fmt.Sprint(entry)
	}
	return strings.Join(parts, "  ")
}

func firstAuditValue(entry map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := entry[key]
		if !ok || value == nil {
			continue
		}
		if text := auditValueString(value); text != "" {
			return text
		}
	}
	return ""
}

func auditValueString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		if raw, err := json.Marshal(v); err == nil {
			return string(raw)
		}
		return fmt.Sprint(v)
	}
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

// softGatewayEngineEndpoint 取任一软网关的采集引擎地址。
// 系统组件页只需要一个可展示/探活的地址；没有配置软网关时返回空，状态显示 unknown。
func (app *driverLibApp) softGatewayEngineEndpoint(ctx context.Context) string {
	gateways, _, err := app.dbClient.WithCtx(ctx).GatewaySearch(0, -1, dtos.GatewaySearchQueryRequest{
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{IsAll: true},
		Source:                   models.GatewaySourceIoserver,
	})
	if err != nil {
		return ""
	}
	for _, gateway := range gateways {
		if endpoint := strings.TrimRight(strings.TrimSpace(gateway.EngineBaseURL), "/"); endpoint != "" {
			return endpoint
		}
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

// probeHTTPEndpoint 对 HTTP 服务做一次短超时请求。
// 有响应（任意状态码，含 404）=running，连接失败/超时=stopped，
// 地址为空或无法解析=unknown。
func probeHTTPEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "unknown"
	}
	target := endpoint
	if !strings.Contains(target, "://") {
		target = "http://" + target
	}
	parsed, err := url.Parse(target)
	if err != nil || parsed.Host == "" {
		return "unknown"
	}
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get(target)
	if err != nil {
		// 证书不被信任时连接本身可能是通的（内部 CA / 自签证书），
		// 不能等同于服务已停止；交给运维确认，状态先显示 unknown。
		var unknownAuthority x509.UnknownAuthorityError
		var hostnameErr x509.HostnameError
		var invalidErr x509.CertificateInvalidError
		var verifyErr *tls.CertificateVerificationError
		if errors.As(err, &unknownAuthority) || errors.As(err, &hostnameErr) ||
			errors.As(err, &invalidErr) || errors.As(err, &verifyErr) {
			return "unknown"
		}
		return "stopped"
	}
	_ = resp.Body.Close()
	return "running"
}
