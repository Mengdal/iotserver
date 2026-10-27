package seed

import (
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"iotserver/internal/models"
)

// 存量租户升级时要补一块组件演示看板；平台自己的看板由首次访问统一种入，
// 迁移不能提前插入，否则会破坏「空列表种全套」的判断。
func TestEnsureTenantBuiltinBoards(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "seed.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Tenant{}, &models.VizBoard{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	tenant := models.Tenant{Id: "t1", Code: "t1", Name: "Tenant One"}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	if err := EnsureTenantBuiltinBoards(db); err != nil {
		t.Fatalf("ensure boards: %v", err)
	}

	var boards []models.VizBoard
	if err := db.Where("tenant_id = ?", tenant.Id).Find(&boards).Error; err != nil {
		t.Fatalf("query tenant boards: %v", err)
	}
	if len(boards) != 1 || boards[0].Name != "Component Demo" || boards[0].IsDefault != 1 {
		t.Fatalf("租户应补一块默认的组件演示看板，实际 %+v", boards)
	}
	var platform int64
	db.Model(&models.VizBoard{}).Where("tenant_id = ''").Count(&platform)
	if platform != 0 {
		t.Fatalf("迁移不应给平台预插看板，实际 %d 块", platform)
	}

	// 幂等：再跑一次不应重复插入
	if err := EnsureTenantBuiltinBoards(db); err != nil {
		t.Fatalf("ensure boards again: %v", err)
	}
	db.Where("tenant_id = ?", tenant.Id).Find(&boards)
	if len(boards) != 1 {
		t.Fatalf("重复迁移不应重复插入，实际 %d 块", len(boards))
	}

	// 租户已有其它默认看板时，补入的演示看板不再抢默认位
	tenant2 := models.Tenant{Id: "t2", Code: "t2", Name: "Tenant Two"}
	db.Create(&tenant2)
	custom := models.VizBoard{Id: "custom-default", Name: "客户默认看板", IsDefault: 1}
	custom.TenantId = "t2"
	db.Create(&custom)
	if err := EnsureTenantBuiltinBoards(db); err != nil {
		t.Fatalf("ensure boards for t2: %v", err)
	}
	var demo models.VizBoard
	if err := db.Where("tenant_id = ? AND name = ?", "t2", "Component Demo").First(&demo).Error; err != nil {
		t.Fatalf("t2 应补组件演示看板: %v", err)
	}
	if demo.IsDefault != 0 {
		t.Fatal("租户已有默认看板时，补入的演示看板不应再标记默认")
	}
}

// 历史参考系统网关的 protocol 要回填为 edge，自有协议网关保持 gatewaymqtt。
func TestMigrateEdgeGatewayProtocol(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "gw.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Gateway{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db.Create(&models.Gateway{Id: "g1", Sn: "sn1", Protocol: models.GatewayProtocolNative, Vendor: "things", Model: "iotddc-gateway"})
	db.Create(&models.Gateway{Id: "g2", Sn: "sn2", Protocol: "", Vendor: "things", Model: "x"})
	db.Create(&models.Gateway{Id: "g3", Sn: "sn3", Protocol: models.GatewayProtocolNative, Vendor: "huawei", Model: "ar502h"})
	if err := MigrateEdgeGatewayProtocol(db); err != nil {
		t.Fatalf("migrate protocol: %v", err)
	}
	for id, want := range map[string]string{"g1": "edge", "g2": "edge", "g3": "gatewaymqtt"} {
		var g models.Gateway
		if err := db.Where("id = ?", id).First(&g).Error; err != nil {
			t.Fatalf("query %s: %v", id, err)
		}
		if g.Protocol != want {
			t.Fatalf("%s protocol = %q, want %q", id, g.Protocol, want)
		}
	}
}

// 空库启动时要补种内置驱动市场，且重复执行不重复插入。
func TestEnsureBuiltinDriverLibraries(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "drivers.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.DeviceLibrary{}, &models.DriverClassify{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := EnsureBuiltinDriverLibraries(db); err != nil {
			t.Fatalf("seed drivers: %v", err)
		}
	}
	var libs []models.DeviceLibrary
	if err := db.Order("id").Find(&libs).Error; err != nil {
		t.Fatalf("query libraries: %v", err)
	}
	if len(libs) != 3 {
		t.Fatalf("内置驱动应为 3 个，实际 %d", len(libs))
	}
	for _, lib := range libs {
		if !lib.IsInternal {
			t.Fatalf("驱动 %s 应标记为内置", lib.Id)
		}
		if lib.ClassifyId != 1 || lib.Manual == "" || lib.Icon == "" {
			t.Fatalf("驱动 %s 缺少市场信息: %+v", lib.Id, lib)
		}
		if !lib.DefaultVersion().IsDefault {
			t.Fatalf("驱动 %s 的当前版本应为默认版本", lib.Id)
		}
	}
	var classifyCount int64
	db.Model(&models.DriverClassify{}).Count(&classifyCount)
	if classifyCount != 1 {
		t.Fatalf("驱动分类应为 1 个，实际 %d", classifyCount)
	}
}

// 内置看板名称统一英文；用户自建看板不被动。
func TestMigrateBuiltinBoardNames(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "names.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.VizBoard{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db.Create(&models.VizBoard{Id: "builtin-demo", Name: "组件演示看板"})
	db.Create(&models.VizBoard{Id: "builtin-home-abc", Name: "首页看板"})
	db.Create(&models.VizBoard{Id: "bd-tenant-monitor", Name: "系统监控看板"})
	db.Create(&models.VizBoard{Id: "custom-board", Name: "客户自己的看板"})
	if err := MigrateBuiltinBoardNames(db); err != nil {
		t.Fatalf("rename: %v", err)
	}
	want := map[string]string{
		"builtin-demo": "Component Demo", "builtin-home-abc": "Home Board",
		"bd-tenant-monitor": "System Monitoring", "custom-board": "客户自己的看板",
	}
	for id, name := range want {
		var b models.VizBoard
		if err := db.Where("id = ?", id).First(&b).Error; err != nil {
			t.Fatalf("query %s: %v", id, err)
		}
		if b.Name != name {
			t.Fatalf("%s name = %q, want %q", id, b.Name, name)
		}
	}
}

// 存量演示看板的文本要补英文正文；用户改过的正文不覆盖。
func TestUpgradeBuiltinBoardTexts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "texts.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.VizBoard{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	canvas := `{"cols":12,"rowHeight":40,"widgets":[
		{"id":"demo-text-s","type":"text","text":"文本组件\n标题栏可拖动、右下角可缩放；\n编辑态点正文直接改字，失焦即保存。"},
		{"id":"demo-text-w","type":"text","text":"用户改过的正文"}]}`
	db.Create(&models.VizBoard{Id: "builtin-demo-abc", Name: "Component Demo", Canvas: canvas})
	if err := UpgradeBuiltinBoardTexts(db); err != nil {
		t.Fatalf("upgrade texts: %v", err)
	}
	var b models.VizBoard
	if err := db.Where("id = ?", "builtin-demo-abc").First(&b).Error; err != nil {
		t.Fatalf("query board: %v", err)
	}
	if !strings.Contains(b.Canvas, `"textEn"`) {
		t.Fatalf("应补上 textEn: %s", b.Canvas)
	}
	if strings.Contains(b.Canvas, `"text":"用户改过的正文","textEn"`) {
		t.Fatalf("用户改过的正文不应补 textEn: %s", b.Canvas)
	}
}

// 系统监控看板只保留平台作用域：租户侧的旧命名/旧 id 都要清掉，用户看板保留。
func TestMigrateSystemBoardPlatformOnly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "ops.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.VizBoard{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	mk := func(id, name, tenantId string) {
		b := models.VizBoard{Id: id, Name: name, Canvas: `{"widgets":[]}`}
		b.TenantId = tenantId
		if err := db.Create(&b).Error; err != nil {
			t.Fatalf("create board %s: %v", id, err)
		}
	}
	mk("bd-tenant-monitor-1", "System Monitoring", "t1")
	mk("legacy-ops", "System Monitoring", "t2")
	mk("legacy-cn", "系统监控看板", "t3")
	mk("custom-board", "客户看板", "t1")
	mk("builtin-ops", "System Monitoring", "")
	if err := MigrateSystemBoardPlatformOnly(db); err != nil {
		t.Fatalf("migrate system boards: %v", err)
	}
	var left []models.VizBoard
	if err := db.Order("id").Find(&left).Error; err != nil {
		t.Fatalf("query boards: %v", err)
	}
	got := map[string]bool{}
	for _, b := range left {
		got[b.Id] = true
	}
	for _, id := range []string{"bd-tenant-monitor-1", "legacy-ops", "legacy-cn"} {
		if got[id] {
			t.Fatalf("租户侧系统监控看板 %s 应被删除", id)
		}
	}
	for _, id := range []string{"custom-board", "builtin-ops"} {
		if !got[id] {
			t.Fatalf("看板 %s 不应被删除", id)
		}
	}
}

func TestMigrateAlertNotifyWays(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "notify.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AlertRule{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	rule := models.AlertRule{Id: "r1", Name: "rule"}
	rule.Notify = models.Notify{
		{Name: "企业微信机器人", Option: models.MapStringString{"webhook": "old"}},
		{Name: "sms", Option: models.MapStringString{"phoneNumber": "+966500000000"}},
		{Name: "whatsapp", Option: models.MapStringString{"phoneNumber": "+966500000000"}},
		{Name: "twilio_email", Option: models.MapStringString{"email": "ops@example.com"}},
	}
	if err := db.Create(&rule).Error; err != nil {
		t.Fatalf("create rule: %v", err)
	}

	if err := MigrateAlertNotifyWays(db); err != nil {
		t.Fatalf("migrate notify ways: %v", err)
	}

	var got models.AlertRule
	if err := db.First(&got, "id = ?", "r1").Error; err != nil {
		t.Fatalf("query rule: %v", err)
	}
	if len(got.Notify) != 3 {
		t.Fatalf("expected 3 remaining channels, got %d: %+v", len(got.Notify), got.Notify)
	}
	foundTwilioEmail := false
	for _, item := range got.Notify {
		if item.Name == "twilio_email" {
			t.Fatalf("twilio_email should be normalized to email")
		}
		if item.Name == "email" {
			foundTwilioEmail = item.Option["provider"] == "twilio"
		}
		if item.Name != "sms" && item.Name != "whatsapp" && item.Name != "email" {
			t.Fatalf("unexpected channel kept: %s", item.Name)
		}
	}
	if !foundTwilioEmail {
		t.Fatal("twilio_email should become email with provider=twilio")
	}
}
