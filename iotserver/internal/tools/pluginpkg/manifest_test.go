package pluginpkg

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func writeArchive(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, "plugin.tar.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(content)),
		}); err != nil {
			t.Fatalf("write header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("write body: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
	return path
}

func TestLoadAndExtract(t *testing.T) {
	dir := t.TempDir()
	archive := writeArchive(t, dir, map[string]string{
		"manifest.json": `{"name":"demo","displayName":"Demo","version":"1.0.0","entrypoint":"run.sh","arch":["any"]}`,
		"run.sh":        "#!/bin/sh\necho ok\n",
	})
	manifest, err := Load(archive)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if manifest.Name != "demo" || manifest.Version != "1.0.0" || !manifest.MatchesArch() {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	dest := filepath.Join(dir, "extract")
	if err := Extract(archive, dest); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if _, err := EntryPath(dest, "run.sh"); err != nil {
		t.Fatalf("entry path: %v", err)
	}
}

func TestExtractRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	archive := writeArchive(t, dir, map[string]string{
		"manifest.json": `{"name":"demo","version":"1.0.0","entrypoint":"run.sh","arch":["any"]}`,
		"../evil.sh":    "bad",
	})
	if err := Extract(archive, filepath.Join(dir, "extract")); err == nil {
		t.Fatal("expected path traversal error")
	}
}

func TestParseRejectsInvalidName(t *testing.T) {
	if _, err := Parse([]byte(`{"name":"../demo","version":"1.0.0","entrypoint":"run.sh"}`)); err == nil {
		t.Fatal("expected invalid manifest name error")
	}
}
