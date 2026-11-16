package logtail

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTailReturnsLastLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "component.log")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, err := Tail(path, 3)
	if err != nil {
		t.Fatalf("tail: %v", err)
	}
	if len(lines) != 3 || lines[0] != "line3" || lines[2] != "line5" {
		t.Fatalf("unexpected lines: %#v", lines)
	}
}

func TestTailMissingFile(t *testing.T) {
	_, err := Tail(filepath.Join(t.TempDir(), "missing.log"), 10)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expect ErrNotFound, got %v", err)
	}
}

func TestTailEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.log")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	lines, err := Tail(path, 10)
	if err != nil {
		t.Fatalf("tail: %v", err)
	}
	if len(lines) != 0 {
		t.Fatalf("expect empty, got %#v", lines)
	}
}

func TestResolvePlainPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "component.log")
	if got := Resolve(path); got != path {
		t.Fatalf("expect plain path unchanged, got %q", got)
	}
}

func TestResolveGlobPicksNewest(t *testing.T) {
	dir := t.TempDir()
	older := filepath.Join(dir, "iotp.log.2026-11-13")
	newer := filepath.Join(dir, "iotp.log.2026-11-14")
	if err := os.WriteFile(older, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newer, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(older, now.Add(-2*time.Hour), now.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, now.Add(-time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	pattern := filepath.Join(dir, "iotp.log.*")
	if got := Resolve(pattern); got != newer {
		t.Fatalf("expect newest %q, got %q", newer, got)
	}
	lines, err := Tail(pattern, 10)
	if err != nil {
		t.Fatalf("tail: %v", err)
	}
	if len(lines) != 1 || lines[0] != "new" {
		t.Fatalf("unexpected lines: %#v", lines)
	}
	info := Stat(pattern)
	if !info.Exists || info.Path != newer {
		t.Fatalf("unexpected info: %+v", info)
	}
}

// TailFile 读取的是调用方指定的文件，不会重新挑「最新」文件：
// ComponentLogs 先 Stat 拿路径再 TailFile，避免两次解析之间轮转导致元信息与内容不一致。
func TestTailFileDoesNotResolveGlob(t *testing.T) {
	dir := t.TempDir()
	older := filepath.Join(dir, "iotp.log.2026-11-13")
	newer := filepath.Join(dir, "iotp.log.2026-11-14")
	if err := os.WriteFile(older, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newer, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(older, now.Add(-2*time.Hour), now.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, now.Add(-time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	lines, err := TailFile(older, 10)
	if err != nil {
		t.Fatalf("tail file: %v", err)
	}
	if len(lines) != 1 || lines[0] != "old" {
		t.Fatalf("TailFile 应读取指定文件而不是最新文件，实际 %#v", lines)
	}
}

func TestResolveNoMatch(t *testing.T) {
	if got := Resolve(filepath.Join(t.TempDir(), "iotp.log.*")); got != "" {
		t.Fatalf("expect empty, got %q", got)
	}
}

func TestStat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "component.log")
	if info := Stat(path); info.Exists {
		t.Fatal("missing file should not exist")
	}
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	info := Stat(path)
	if !info.Exists || info.Size != 5 || info.Path != path {
		t.Fatalf("unexpected info: %+v", info)
	}
}
