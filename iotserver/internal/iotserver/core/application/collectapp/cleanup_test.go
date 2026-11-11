package collectapp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/infrastructure/sqlite"
	"iotserver/internal/models"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/tools/collectengine"
)

func newCleanupTestDB(t *testing.T) *sqlite.Client {
	t.Helper()
	lc := logger.NewMockClient()
	c, err := sqlite.NewClient(dtos.Configuration{DataSource: filepath.Join(t.TempDir(), "core.db")}, lc)
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	t.Cleanup(c.CloseSession)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return c
}

func TestScheduleGatewayCleanup(t *testing.T) {
	c := newCleanupTestDB(t)
	if err := c.Pool.Table(models.TableGateway).Create(&models.Gateway{
		Id: "g1", Sn: "1234567890", Source: models.GatewaySourceIoserver,
		EngineBaseURL: "http://engine", TenantField: models.TenantField{TenantId: "t1"},
	}).Error; err != nil {
		t.Fatalf("create gateway: %v", err)
	}
	for _, ch := range []models.CollectChannel{
		{Id: "g1:6", GatewayId: "g1", RemoteId: 6, Name: "C1", TenantField: models.TenantField{TenantId: "t1"}},
		{Id: "g1:7", GatewayId: "g1", RemoteId: 7, Name: "C2", TenantField: models.TenantField{TenantId: "t1"}},
	} {
		if err := c.Pool.Table(models.TableCollectChannel).Create(&ch).Error; err != nil {
			t.Fatalf("create channel: %v", err)
		}
	}
	service := &service{lc: logger.NewMockClient(), dbClient: c, engines: make(map[string]*collectengine.Client)}
	if err := service.ScheduleGatewayCleanup(context.Background(), "g1"); err != nil {
		t.Fatalf("ScheduleGatewayCleanup: %v", err)
	}
	tasks, err := c.CollectCleanupTasksDue(time.Now().UnixMilli(), 10)
	if err != nil {
		t.Fatalf("tasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].DeleteAll || tasks[0].ChannelIDs != "[6,7]" || tasks[0].GatewayId != "g1" {
		t.Fatalf("清理任务 = %+v", tasks)
	}
}

func TestScheduleGatewayCleanupDeleteAll(t *testing.T) {
	c := newCleanupTestDB(t)
	if err := c.Pool.Table(models.TableGateway).Create(&models.Gateway{
		Id: "g1", Sn: "1234567890", Source: models.GatewaySourceIoserver,
		EngineBaseURL: "http://engine", TenantField: models.TenantField{TenantId: "t1"},
	}).Error; err != nil {
		t.Fatalf("create gateway: %v", err)
	}
	service := &service{lc: logger.NewMockClient(), dbClient: c, engines: make(map[string]*collectengine.Client)}
	if err := service.ScheduleGatewayCleanup(context.Background(), "g1"); err != nil {
		t.Fatalf("ScheduleGatewayCleanup: %v", err)
	}
	tasks, err := c.CollectCleanupTasksDue(time.Now().UnixMilli(), 10)
	if err != nil {
		t.Fatalf("tasks: %v", err)
	}
	if len(tasks) != 1 || !tasks[0].DeleteAll {
		t.Fatalf("清理任务 = %+v, want DeleteAll", tasks)
	}
}

func TestScheduleGatewayCleanupSkipsSharedEngine(t *testing.T) {
	c := newCleanupTestDB(t)
	for _, g := range []models.Gateway{
		{Id: "g1", Sn: "1234567890", Source: models.GatewaySourceIoserver, EngineBaseURL: "http://engine", TenantField: models.TenantField{TenantId: "t1"}},
		{Id: "g2", Sn: "1234567891", Source: models.GatewaySourceIoserver, EngineBaseURL: "http://engine", TenantField: models.TenantField{TenantId: "t1"}},
	} {
		if err := c.Pool.Table(models.TableGateway).Create(&g).Error; err != nil {
			t.Fatalf("create gateway: %v", err)
		}
	}
	service := &service{lc: logger.NewMockClient(), dbClient: c, engines: make(map[string]*collectengine.Client)}
	if err := service.ScheduleGatewayCleanup(context.Background(), "g1"); err != nil {
		t.Fatalf("ScheduleGatewayCleanup: %v", err)
	}
	tasks, err := c.CollectCleanupTasksDue(time.Now().UnixMilli(), 10)
	if err != nil {
		t.Fatalf("tasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("共用引擎不应登记清空任务: %+v", tasks)
	}
}

func TestExecuteCleanupTask(t *testing.T) {
	var (
		mu       sync.Mutex
		deletedD []int64
		deletedC []int64
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/devices":
			_, _ = w.Write([]byte(`{"code":200,"message":"ok","data":[
				{"id":21,"channelId":6,"iotpId":1},
				{"id":22,"channelId":6,"iotpId":2},
				{"id":23,"channelId":7,"iotpId":3},
				{"id":24,"channelId":99,"iotpId":4}]}`))
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/devices/"):
			id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/v1/devices/"), 10, 64)
			deletedD = append(deletedD, id)
			_, _ = w.Write([]byte(`{"code":200,"message":"ok"}`))
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/channels/"):
			id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/v1/channels/"), 10, 64)
			deletedC = append(deletedC, id)
			_, _ = w.Write([]byte(`{"code":200,"message":"ok"}`))
		default:
			_, _ = w.Write([]byte(`{"code":200,"message":"ok"}`))
		}
	}))
	defer srv.Close()

	c := newCleanupTestDB(t)
	if err := c.AddCollectCleanupTask(models.CollectCleanupTask{
		TenantField:   models.TenantField{TenantId: "t1"},
		GatewayId:     "g1",
		EngineBaseURL: srv.URL,
		ChannelIDs:    "[6,7]",
	}); err != nil {
		t.Fatalf("add task: %v", err)
	}
	tasks, err := c.CollectCleanupTasksDue(time.Now().UnixMilli(), 10)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("tasks = %+v err=%v", tasks, err)
	}

	service := &service{lc: logger.NewMockClient(), dbClient: c, engines: make(map[string]*collectengine.Client)}
	service.executeCleanupTask(context.Background(), c, tasks[0])

	mu.Lock()
	defer mu.Unlock()
	wantD := map[int64]bool{21: true, 22: true, 23: true}
	if len(deletedD) != len(wantD) {
		t.Fatalf("删除设备 = %v, want %v", deletedD, wantD)
	}
	for _, id := range deletedD {
		if !wantD[id] {
			t.Fatalf("误删设备 %d", id)
		}
	}
	if len(deletedC) != 2 || !containsInt64(deletedC, 6) || !containsInt64(deletedC, 7) {
		t.Fatalf("删除通道 = %v, want [6 7]", deletedC)
	}
	remaining, err := c.CollectCleanupTasksDue(time.Now().UnixMilli(), 10)
	if err != nil {
		t.Fatalf("tasks after cleanup: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("清理成功后任务应删除: %+v", remaining)
	}
}

// 任务排队期间引擎被新网关接管时，DeleteAll 不能清空整个引擎。
func TestExecuteCleanupTaskDeleteAllSkipsSharedEngine(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"code":200,"message":"ok"}`))
	}))
	defer srv.Close()

	c := newCleanupTestDB(t)
	// 新网关 g2 已占用同一个采集引擎。
	if err := c.Pool.Table(models.TableGateway).Create(&models.Gateway{
		Id: "g2", Sn: "1234567891", Source: models.GatewaySourceIoserver,
		EngineBaseURL: srv.URL, TenantField: models.TenantField{TenantId: "t1"},
	}).Error; err != nil {
		t.Fatalf("create gateway: %v", err)
	}
	if err := c.AddCollectCleanupTask(models.CollectCleanupTask{
		TenantField:   models.TenantField{TenantId: "t1"},
		GatewayId:     "g1",
		EngineBaseURL: srv.URL,
		DeleteAll:     true,
	}); err != nil {
		t.Fatalf("add task: %v", err)
	}
	tasks, err := c.CollectCleanupTasksDue(time.Now().UnixMilli(), 10)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("tasks = %+v err=%v", tasks, err)
	}

	service := &service{lc: logger.NewMockClient(), dbClient: c, engines: make(map[string]*collectengine.Client)}
	service.executeCleanupTask(context.Background(), c, tasks[0])

	if calls != 0 {
		t.Fatalf("引擎已被接管时不应调用引擎，实际 %d 次", calls)
	}
	remaining, err := c.CollectCleanupTasksDue(time.Now().UnixMilli(), 10)
	if err != nil || len(remaining) != 0 {
		t.Fatalf("任务应被放弃: %+v err=%v", remaining, err)
	}
}

func TestExecuteCleanupTaskDeleteAll(t *testing.T) {
	var (
		mu       sync.Mutex
		cleared  bool
		deletedD []int64
		deletedC []int64
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tags":
			cleared = true
			_, _ = w.Write([]byte(`{"code":200,"message":"清空所有点位成功"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/devices":
			_, _ = w.Write([]byte(`{"code":200,"message":"ok","data":[{"id":21,"channelId":6,"iotpId":1}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/channels":
			_, _ = w.Write([]byte(`{"code":200,"message":"ok","data":[{"id":6,"channelCode":"C1"}]}`))
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/devices/"):
			id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/v1/devices/"), 10, 64)
			deletedD = append(deletedD, id)
			_, _ = w.Write([]byte(`{"code":200,"message":"ok"}`))
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/channels/"):
			id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/v1/channels/"), 10, 64)
			deletedC = append(deletedC, id)
			_, _ = w.Write([]byte(`{"code":200,"message":"ok"}`))
		default:
			_, _ = w.Write([]byte(`{"code":200,"message":"ok"}`))
		}
	}))
	defer srv.Close()

	c := newCleanupTestDB(t)
	if err := c.AddCollectCleanupTask(models.CollectCleanupTask{
		TenantField:   models.TenantField{TenantId: "t1"},
		GatewayId:     "g1",
		EngineBaseURL: srv.URL,
		DeleteAll:     true,
	}); err != nil {
		t.Fatalf("add task: %v", err)
	}
	tasks, err := c.CollectCleanupTasksDue(time.Now().UnixMilli(), 10)
	if err != nil || len(tasks) != 1 {
		t.Fatalf("tasks = %+v err=%v", tasks, err)
	}
	service := &service{lc: logger.NewMockClient(), dbClient: c, engines: make(map[string]*collectengine.Client)}
	service.executeCleanupTask(context.Background(), c, tasks[0])

	mu.Lock()
	defer mu.Unlock()
	if !cleared {
		t.Fatal("delete_all 任务应先调用清空点位接口")
	}
	if len(deletedD) != 1 || deletedD[0] != 21 {
		t.Fatalf("删除设备 = %v", deletedD)
	}
	if len(deletedC) != 1 || deletedC[0] != 6 {
		t.Fatalf("删除通道 = %v", deletedC)
	}
	remaining, err := c.CollectCleanupTasksDue(time.Now().UnixMilli(), 10)
	if err != nil || len(remaining) != 0 {
		t.Fatalf("清理成功后任务应删除: %+v err=%v", remaining, err)
	}
}
