package logtail

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
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
