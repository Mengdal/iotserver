package gateway

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type warnRecorder struct{ warns int }

func (w *warnRecorder) Warnf(string, ...interface{}) { w.warns++ }

// pruneBackups 只保留最近 keep 份、只删形如时间戳的目录
func TestPruneBackups(t *testing.T) {
	parent := t.TempDir()
	// 造 5 份备份 + 1 个无关目录 + 1 个文件
	names := []string{
		"20260101-010000",
		"20260102-010000",
		"20260103-010000",
		"20260104-010000",
		"20260105-010000",
	}
	for _, n := range names {
		if err := os.MkdirAll(filepath.Join(parent, n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(parent, "not-a-backup"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "readme.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := &warnRecorder{}
	removed := pruneBackups(parent, 2, w)
	if removed != 3 {
		t.Fatalf("removed=%d want 3", removed)
	}

	// 最新的两份必须保留
	for _, keep := range []string{"20260105-010000", "20260104-010000"} {
		if _, err := os.Stat(filepath.Join(parent, keep)); err != nil {
			t.Fatalf("最新备份 %s 应保留: %v", keep, err)
		}
	}
	// 最旧的三份必须删掉
	for _, gone := range []string{"20260101-010000", "20260102-010000", "20260103-010000"} {
		if _, err := os.Stat(filepath.Join(parent, gone)); !os.IsNotExist(err) {
			t.Fatalf("旧备份 %s 应被删除, err=%v", gone, err)
		}
	}
	// 无关目录与文件不能被误删
	if _, err := os.Stat(filepath.Join(parent, "not-a-backup")); err != nil {
		t.Fatalf("非备份目录被误删: %v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "readme.txt")); err != nil {
		t.Fatalf("普通文件被误删: %v", err)
	}

	// keep 大于已有数量时不应删除
	if removed := pruneBackups(parent, 10, w); removed != 0 {
		t.Fatalf("keep 大于已有数量时 removed=%d want 0", removed)
	}
}

// 目录名不是时间戳格式时一律不动
func TestPruneBackupsIgnoresNonTimestampDirs(t *testing.T) {
	parent := t.TempDir()
	for _, n := range []string{"manual-backup", "2026-01-01", "backup_old"} {
		if err := os.MkdirAll(filepath.Join(parent, n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if removed := pruneBackups(parent, 1, &warnRecorder{}); removed != 0 {
		t.Fatalf("非时间戳目录不应被清理, removed=%d", removed)
	}
	entries, _ := os.ReadDir(parent)
	if len(entries) != 3 {
		t.Fatalf("目录数=%d want 3", len(entries))
	}
}

var _ = time.Now
