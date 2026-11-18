package driverapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"

	"iotserver/internal/iotserver/core/config"
	"iotserver/internal/pkg/seed"
)

var errEmptyVersionURL = errors.New("empty version url")

const (
	// 版本探测不能拖慢组件列表：单次最多 1 秒。
	componentVersionTimeout = time.Second
	// 探测成功缓存 5 分钟，失败 30 秒后重试（组件可能刚启动）。
	componentVersionTTL        = 5 * time.Minute
	componentVersionFailureTTL = 30 * time.Second
)

var componentVersionHTTPClient = &http.Client{Timeout: componentVersionTimeout}

type componentVersionCacheEntry struct {
	version string
	at      time.Time
}

type componentVersionCall struct {
	done    chan struct{}
	version string
}

var componentVersionCache = struct {
	sync.Mutex
	items    map[string]componentVersionCacheEntry
	inflight map[string]*componentVersionCall
}{items: map[string]componentVersionCacheEntry{}, inflight: map[string]*componentVersionCall{}}

// cachedComponentVersion 缓存探测结果，避免每次打开「我的驱动」都去打外部接口。
// 同一组件的并发探测只放行一个，等待者复用结果，避免慢请求把刚缓存的好版本覆盖成空。
func cachedComponentVersion(ctx context.Context, id string, probe func(context.Context) string) string {
	now := time.Now()
	componentVersionCache.Lock()
	if entry, ok := componentVersionCache.items[id]; ok {
		ttl := componentVersionTTL
		if entry.version == "" {
			ttl = componentVersionFailureTTL
		}
		if now.Sub(entry.at) < ttl {
			componentVersionCache.Unlock()
			return entry.version
		}
	}
	if call := componentVersionCache.inflight[id]; call != nil {
		componentVersionCache.Unlock()
		select {
		case <-call.done:
			return call.version
		case <-ctx.Done():
			return ""
		}
	}
	call := &componentVersionCall{done: make(chan struct{})}
	componentVersionCache.inflight[id] = call
	componentVersionCache.Unlock()

	version := probe(ctx)
	call.version = version
	componentVersionCache.Lock()
	componentVersionCache.items[id] = componentVersionCacheEntry{version: version, at: time.Now()}
	delete(componentVersionCache.inflight, id)
	componentVersionCache.Unlock()
	close(call.done)
	return version
}

// detectComponentVersions 尽量探测各组件的真实版本。
// 探测失败不返回错误，调用方继续使用 device_library 里的占位值（builtin/external）。
func detectComponentVersions(ctx context.Context, cfg *config.ConfigurationStruct, endpoints map[string]string) map[string]string {
	versions := map[string]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	set := func(id, version string) {
		version = strings.TrimSpace(version)
		if version == "" {
			return
		}
		mu.Lock()
		versions[id] = version
		mu.Unlock()
	}

	if endpoint := strings.TrimSpace(endpoints[seed.SystemComponentEKuiperId]); endpoint != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			set(seed.SystemComponentEKuiperId, cachedComponentVersion(ctx, seed.SystemComponentEKuiperId, func(ctx context.Context) string {
				return probeEKuiperVersion(ctx, endpoint)
			}))
		}()
	}

	if cfg != nil {
		db := cfg.GetDataDatabaseInfo()
		base := strings.TrimRight(strings.TrimSpace(db.BaseURL), "/")
		if base != "" {
			wg.Add(1)
			go func() {
				defer wg.Done()
				set(seed.SystemComponentIotEdgeDBId, cachedComponentVersion(ctx, seed.SystemComponentIotEdgeDBId, func(ctx context.Context) string {
					return probeIotEdgeDBVersion(ctx, base, db.Token)
				}))
			}()
		}
	}

	// Mosquitto / iotpGW 没有版本接口，只有 Docker 部署才需要读容器镜像 tag。
	// embedded 模式即使宿主机装了 Docker，也不应该拿本地无关容器覆盖版本。
	if cfg != nil && cfg.DeploymentMode() == config.DeploymentModeServer {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dockerVersions := cachedDockerComponentVersions(endpoints)
			if version := dockerVersions[seed.SystemComponentMosquittoId]; version != "" {
				set(seed.SystemComponentMosquittoId, version)
			}
			if version := dockerVersions[seed.SystemComponentIotpGWId]; version != "" {
				set(seed.SystemComponentIotpGWId, version)
			}
		}()
	}

	wg.Wait()
	return versions
}

// cachedDockerComponentVersions 缓存 Docker 版本探测：ContainerList 可能耗时，
// 不能每次打开组件列表都扫一遍；失败结果只缓存 30 秒，组件刚启动时能尽快恢复。
func cachedDockerComponentVersions(endpoints map[string]string) map[string]string {
	key := endpoints[seed.SystemComponentMosquittoId] + "|" + endpoints[seed.SystemComponentIotpGWId]
	now := time.Now()
	dockerVersionCache.Lock()
	if dockerVersionCache.key == key {
		ttl := componentVersionTTL
		if len(dockerVersionCache.versions) == 0 {
			ttl = componentVersionFailureTTL
		}
		if now.Sub(dockerVersionCache.at) < ttl {
			cached := copyVersions(dockerVersionCache.versions)
			dockerVersionCache.Unlock()
			return cached
		}
	}
	dockerVersionCache.Unlock()

	versions := probeDockerComponentVersions(endpoints)
	if versions == nil {
		versions = map[string]string{}
	}
	dockerVersionCache.Lock()
	dockerVersionCache.key = key
	dockerVersionCache.versions = copyVersions(versions)
	dockerVersionCache.at = time.Now()
	dockerVersionCache.Unlock()
	return versions
}

var dockerVersionCache = struct {
	sync.Mutex
	key      string
	at       time.Time
	versions map[string]string
}{versions: map[string]string{}}

func copyVersions(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// probeEKuiperVersion 读取 eKuiper 根路径的 version 字段（实测 2.4.2）。
func probeEKuiperVersion(ctx context.Context, endpoint string) string {
	body, err := componentVersionGet(ctx, endpoint, "")
	if err != nil {
		return ""
	}
	return versionFromJSON(body)
}

// probeIotEdgeDBVersion 优先读无需鉴权的 /api/v1/version；
// 老版本没有该接口时，再用配置的 token 读 /api/v1/status 兜底。
func probeIotEdgeDBVersion(ctx context.Context, base, token string) string {
	if body, err := componentVersionGet(ctx, base+"/api/v1/version", ""); err == nil {
		if version := versionFromJSON(body); version != "" {
			return version
		}
	}
	body, err := componentVersionGet(ctx, base+"/api/v1/status", token)
	if err != nil {
		return ""
	}
	return versionFromJSON(body)
}

func versionFromJSON(body []byte) string {
	var payload struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Version)
}

// componentVersionGet 带请求超时和响应大小上限，避免错误地址返回大文件时拖垮服务。
func componentVersionGet(ctx context.Context, rawURL, token string) ([]byte, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, errEmptyVersionURL
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = "http://" + rawURL
	}
	ctx, cancel := context.WithTimeout(ctx, componentVersionTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if token = strings.TrimSpace(token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := componentVersionHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("version request failed: status=%d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// probeDockerComponentVersions 从容器镜像 tag 推断版本。
// 容器名默认按 compose：mqtt-broker；iotpGW 容器名不确定，再按引擎端口匹配。
func probeDockerComponentVersions(endpoints map[string]string) map[string]string {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), componentVersionTimeout)
	defer cancel()
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return nil
	}

	// 只有「平台要连的地址确实是本机/本 compose 容器」时，才允许用本机容器推断版本；
	// 否则连的是远程 broker/引擎，本地同名容器只是巧合，不能覆盖版本。
	containerNames := make(map[string]struct{}, len(containers))
	for _, c := range containers {
		for _, name := range c.Names {
			containerNames[strings.TrimPrefix(strings.ToLower(name), "/")] = struct{}{}
		}
	}
	endpointIsLocal := func(endpoint string) bool {
		if isLoopbackEndpoint(endpoint) {
			return true
		}
		host := strings.ToLower(endpointHost(endpoint))
		_, ok := containerNames[host]
		return ok
	}

	versions := map[string]string{}
	mosquittoLocal := endpointIsLocal(endpoints[seed.SystemComponentMosquittoId])
	iotpGWLocal := endpointIsLocal(endpoints[seed.SystemComponentIotpGWId])
	for _, c := range containers {
		name := strings.ToLower(strings.Join(c.Names, " "))
		image := strings.ToLower(c.Image)
		if mosquittoLocal && versions[seed.SystemComponentMosquittoId] == "" &&
			(strings.Contains(name, "mqtt-broker") || strings.Contains(image, "mqtt-broker")) {
			versions[seed.SystemComponentMosquittoId] = tagFromImage(c.Image)
		}
		if iotpGWLocal && versions[seed.SystemComponentIotpGWId] == "" &&
			(strings.Contains(name, "iotpgw") || strings.Contains(name, "iotp-gw") || strings.Contains(image, "iotpgw")) {
			versions[seed.SystemComponentIotpGWId] = tagFromImage(c.Image)
		}
	}

	// iotpGW 容器名可能被改过：仅当引擎地址是本机时，才按宿主机映射端口匹配。
	// 远程引擎（如 192.168.0.74:8081）不能用本机同端口容器顶替版本。
	if versions[seed.SystemComponentIotpGWId] == "" && iotpGWLocal {
		port := endpointPort(endpoints[seed.SystemComponentIotpGWId])
		if port != "" {
			for _, c := range containers {
				for _, p := range c.Ports {
					if p.PublicPort != 0 && strconv.Itoa(int(p.PublicPort)) == port {
						versions[seed.SystemComponentIotpGWId] = tagFromImage(c.Image)
						break
					}
				}
				if versions[seed.SystemComponentIotpGWId] != "" {
					break
				}
			}
		}
	}
	return versions
}

// endpointHost 取地址里的主机名（兼容没有 scheme 的 host:port）。
func endpointHost(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

// isLoopbackEndpoint 判断地址是否指向本机回环。
func isLoopbackEndpoint(endpoint string) bool {
	host := strings.ToLower(endpointHost(endpoint))
	return host == "localhost" || host == "127.0.0.1" || strings.HasPrefix(host, "127.") || host == "::1"
}

func endpointPort(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return parsed.Port()
}

// tagFromImage 从镜像名里取 tag：registry/x/mqtt-broker:2.6 -> 2.6。
// 无 tag、<none>、digest 都返回空字符串。
func tagFromImage(image string) string {
	image = strings.TrimSpace(image)
	// 容器直接从镜像 ID 创建时，c.Image 是 "sha256:<digest>"，不是 name:tag。
	if strings.HasPrefix(strings.ToLower(image), "sha256:") || strings.HasPrefix(strings.ToLower(image), "sha512:") {
		return ""
	}
	if i := strings.Index(image, "@"); i >= 0 {
		image = image[:i]
	}
	if i := strings.LastIndex(image, "/"); i >= 0 {
		image = image[i+1:]
	}
	i := strings.LastIndex(image, ":")
	if i < 0 {
		return ""
	}
	tag := strings.TrimSpace(image[i+1:])
	if tag == "" || tag == "<none>" || strings.HasPrefix(tag, "sha256") {
		return ""
	}
	return tag
}
