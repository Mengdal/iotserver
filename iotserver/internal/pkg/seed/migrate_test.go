package seed

import (
	"path/filepath"
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
	if len(boards) != 1 || boards[0].Name != "组件演示看板" || boards[0].IsDefault != 1 {
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
	if err := db.Where("tenant_id = ? AND name = ?", "t2", "组件演示看板").First(&demo).Error; err != nil {
		t.Fatalf("t2 应补组件演示看板: %v", err)
	}
	if demo.IsDefault != 0 {
		t.Fatal("租户已有默认看板时，补入的演示看板不应再标记默认")
	}
}
