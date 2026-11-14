package driverapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"iotserver/internal/iotserver/core/config"
	"iotserver/internal/models"
)

func TestSOInstallerInstallAndUninstall(t *testing.T) {
	root := t.TempDir()
	driverDir := filepath.Join(root, "drivers")
	backupDir := filepath.Join(root, "backups")
	src := filepath.Join(root, "ModbusClient.so")
	if err := os.WriteFile(src, []byte("v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.ConfigurationStruct{CollectEngine: config.CollectEngineInfo{
		DriverDir:  driverDir,
		BackupDir:  backupDir,
		ReloadMode: "none",
	}}
	installer := newSOInstaller(cfg, nil)
	library := models.DeviceLibrary{
		PackagePath: src,
		PackageHash: hashOf("v1"),
		Manifest:    `{"fileName":"ModbusClient.so","abi":"1"}`,
	}
	manifest := soManifest{FileName: "ModbusClient.so", ABI: "1"}

	if err := installer.install(context.Background(), library, manifest); err != nil {
		t.Fatalf("install: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(driverDir, manifest.FileName)); string(got) != "v1" {
		t.Fatalf("installed content = %q", got)
	}
	if !installer.installed(library) {
		t.Fatal("driver should be installed")
	}

	if err := installer.uninstall(context.Background(), manifest); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if _, err := os.Stat(filepath.Join(driverDir, manifest.FileName)); !os.IsNotExist(err) {
		t.Fatalf("driver should be removed, err=%v", err)
	}
}

func TestSOInstallerReloadFailureRemovesNewFile(t *testing.T) {
	root := t.TempDir()
	driverDir := filepath.Join(root, "drivers")
	src := filepath.Join(root, "ModbusClient.so")
	if err := os.WriteFile(src, []byte("v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.ConfigurationStruct{CollectEngine: config.CollectEngineInfo{
		DriverDir:     driverDir,
		BackupDir:     filepath.Join(root, "backups"),
		ReloadMode:    "command",
		ReloadCommand: "exit 1",
	}}
	installer := newSOInstaller(cfg, nil)
	library := models.DeviceLibrary{PackagePath: src, PackageHash: hashOf("v1")}
	manifest := soManifest{FileName: "ModbusClient.so"}
	if err := installer.install(context.Background(), library, manifest); err == nil {
		t.Fatal("install should fail when reload fails")
	}
	if _, err := os.Stat(filepath.Join(driverDir, manifest.FileName)); !os.IsNotExist(err) {
		t.Fatalf("failed install should remove new file, err=%v", err)
	}
}

func TestValidateSOFileName(t *testing.T) {
	for _, name := range []string{"ModbusClient.so", "modbus_tcp.so"} {
		if err := validateSOFileName(name); err != nil {
			t.Fatalf("%s should be valid: %v", name, err)
		}
	}
	for _, name := range []string{"", "ModbusClient", "../x.so", "dir/x.so", ".hidden.so"} {
		if err := validateSOFileName(name); err == nil {
			t.Fatalf("%s should be invalid", name)
		}
	}
}

func hashOf(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
