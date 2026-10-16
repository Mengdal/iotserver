package sqlite

import (
	"testing"

	"github.com/google/uuid"

	"iotserver/internal/models"
	"iotserver/internal/pkg/errort"
)

// TestDeviceByIdWithNonNumericId 锁定「主键不是纯数字时也能按 ID 查设备」。
//
// 背景：原先用 GORM 的内联主键简写 `First(&device, id)`。GORM v1.23 的
// BuildCondition（statement.go:275 起）先做 `strconv.Atoi(id)`：
// **只有能解析成整数的字符串**才被当作主键值参数化；解析失败时它把整个值当
// **原始 SQL** 拼进 WHERE（`clause.Expr{SQL: s}`），于是 uuid 这种含连字符的 ID
// 会拼出 `WHERE a604a540-af9b-4a4f-...`，SQLite 报 `unrecognized token: "4a4f"`。
//
// 这个坑过去不暴露，只因为所有主键恰好都是 utils.RandomNum 生成的 8 位数字。
// 网关自动接入的设备改用 uuid 当主键后立刻踩穿，后果不止查不到：
// persistence.resolveTenant 依赖 DeviceById 推导租户，失败后时序数据会以
// **空 tenant_id** 落库——多租户下等于数据归属丢失。
func TestDeviceByIdWithNonNumericId(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	uuidID := uuid.NewString()
	if err := c.Pool.Table(models.TableDevice).Create(map[string]interface{}{
		"id":        uuidID,
		"name":      "uuid 主键设备",
		"device_sn": "SN-UUID-1",
		"tenant_id": "",
		"created":   1,
		"modified":  1,
	}).Error; err != nil {
		t.Fatalf("insert uuid device: %v", err)
	}

	// 1) uuid 主键必须能查出来（修复前这里会因为 SQL 语法错误而失败）
	got, err := deviceById(c, uuidID)
	if err != nil {
		t.Fatalf("按 uuid 主键查询失败: %v", err)
	}
	if got.Id != uuidID {
		t.Fatalf("查到 id=%s want %s", got.Id, uuidID)
	}

	// 2) 纯数字主键（既有惯例）不能被改坏
	const numID = "08159395"
	if err := c.Pool.Table(models.TableDevice).Create(map[string]interface{}{
		"id":        numID,
		"name":      "数字主键设备",
		"device_sn": "SN-NUM-1",
		"tenant_id": "",
		"created":   1,
		"modified":  1,
	}).Error; err != nil {
		t.Fatalf("insert numeric device: %v", err)
	}
	numGot, err := deviceById(c, numID)
	if err != nil {
		t.Fatalf("数字主键查询回归失败: %v", err)
	}
	if numGot.Id != numID {
		t.Fatalf("数字主键查到 id=%s want %s", numGot.Id, numID)
	}

	// 3) 查不到仍须是「设备不存在」，而不是系统错误
	if _, err := deviceById(c, uuid.NewString()); !errort.Is(errort.DeviceNotExist, err) {
		t.Fatalf("未命中应返回 DeviceNotExist，got %v", err)
	}

	// 4) msgReportDeviceById 与 deviceById 是同一种写法，一并锁定
	if _, err := msgReportDeviceById(c, uuid.NewString()); !errort.Is(errort.DeviceNotExist, err) {
		t.Fatalf("msgReportDeviceById 未命中应返回 DeviceNotExist，got %v", err)
	}
	if reported, err := msgReportDeviceById(c, uuidID); err != nil || reported.Id != uuidID {
		t.Fatalf("msgReportDeviceById 按 uuid 查询失败: id=%s err=%v", reported.Id, err)
	}
}
