package pluginapp

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"iotserver/internal/models"
	"iotserver/internal/pkg/utils"
	"iotserver/internal/tools/pluginpkg"
)

func TestNativeRuntimeStartStopAndLogs(t *testing.T) {
	root := filepath.Join(t.TempDir(), "plugins")
	dataDir := filepath.Join(t.TempDir(), "data")
	versionDir := filepath.Join(root, "demo", "1.0.0")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	script := "#!/bin/sh\necho \"plugin started port=$IOT_PLUGIN_PORT\"\ntrap 'exit 0' TERM\nwhile true; do sleep 0.2; done\n"
	if err := os.WriteFile(filepath.Join(versionDir, "run.sh"), []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	manifest := `{"name":"demo","version":"1.0.0","entrypoint":"run.sh","arch":["any"],"ports":[{"name":"http","default":0}]}`
	library := models.DeviceLibrary{Id: "demo", Name: "Demo", Version: "1.0.0", Manifest: manifest}
	instance := models.DeviceService{Id: "instance-1", Config: models.MapStringInterface{"addr": "40001"}}

	runtime := newNativeRuntime(root, dataDir, nil)
	pid, address, err := runtime.Start(library, instance)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if pid <= 0 || address == "" {
		t.Fatalf("unexpected start result: pid=%d address=%q", pid, address)
	}
	defer func() { _ = runtime.Stop(instance.Id) }()

	deadline := time.Now().Add(5 * time.Second)
	for !runtime.Running(instance.Id) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if !runtime.Running(instance.Id) {
		t.Fatal("process should be running")
	}

	deadline = time.Now().Add(5 * time.Second)
	var lines []string
	for time.Now().Before(deadline) {
		lines, err = runtime.Logs(instance.Id, 20)
		if err == nil && len(lines) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil || len(lines) == 0 {
		t.Fatalf("expected plugin log output, lines=%v err=%v", lines, err)
	}
	configData, err := os.ReadFile(filepath.Join(dataDir, instance.Id, "config.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if len(configData) == 0 {
		t.Fatal("config file should not be empty")
	}

	if err := runtime.Stop(instance.Id); err != nil {
		t.Fatalf("stop: %v", err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for runtime.Running(instance.Id) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if runtime.Running(instance.Id) {
		t.Fatal("process should be stopped")
	}
}

// 平台重启后 plugin.pid 里可能是被复用的旧 PID。
// 工作目录不属于实例时必须判定为未运行，并清掉脏 PID 文件。
func TestRunningIgnoresStalePidFile(t *testing.T) {
	if _, err := os.Stat("/proc/self"); err != nil {
		t.Skip("PID 归属校验仅在 Linux 上生效")
	}
	dataDir := t.TempDir()
	instanceId := "inst-stale"
	instanceDir := filepath.Join(dataDir, instanceId)
	if err := os.MkdirAll(instanceDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	pidFile := filepath.Join(instanceDir, "plugin.pid")
	// 用测试进程自己的 PID 模拟「PID 被别的不相关进程复用」。
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		t.Fatalf("write pid: %v", err)
	}

	r := newNativeRuntime(t.TempDir(), dataDir, nil)
	if r.Running(instanceId) {
		t.Fatal("stale pid from another process must not be treated as running")
	}
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Fatal("stale pid file should be removed")
	}

	// 测试进程本身必须还活着。
	if err := syscall.Kill(os.Getpid(), 0); err != nil {
		t.Fatalf("test process should still be alive: %v", err)
	}
}

func TestReplaceDir(t *testing.T) {
	root := t.TempDir()
	dst := filepath.Join(root, "demo", "1.0.0")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dst, "old.txt"), []byte("old"), 0o644); err != nil {
		t.Fatalf("write old: %v", err)
	}

	src := filepath.Join(root, "staging")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatalf("write new: %v", err)
	}

	if err := replaceDir(src, dst); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "new.txt")); err != nil {
		t.Fatalf("new file should exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "old.txt")); !os.IsNotExist(err) {
		t.Fatal("old file should be gone")
	}
	if _, err := os.Stat(dst + ".old"); !os.IsNotExist(err) {
		t.Fatal("backup dir should be removed")
	}

	// 源目录不存在时替换失败，旧版本必须保持可用。
	src2 := filepath.Join(root, "missing")
	if err := replaceDir(src2, dst); err == nil {
		t.Fatal("replace with missing source should fail")
	}
	if _, err := os.Stat(filepath.Join(dst, "new.txt")); err != nil {
		t.Fatalf("previous version should stay usable after failed replace: %v", err)
	}
}

func TestAllocatePortReserves(t *testing.T) {
	address, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	free, err := utils.AvailablePort(address)
	if err != nil {
		t.Fatalf("pick free port: %v", err)
	}
	manifest, err := pluginpkg.Parse([]byte(fmt.Sprintf(
		`{"name":"demo","version":"1.0.0","entrypoint":"run.sh","arch":["any"],"ports":[{"name":"http","default":%d}]}`, free)))
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	r := newNativeRuntime(t.TempDir(), t.TempDir(), nil)
	p1, err := r.allocatePort(manifest, "instance-1")
	if err != nil {
		t.Fatalf("allocate first: %v", err)
	}
	if p1 != free {
		t.Fatalf("first instance should get the preferred port %d, got %d", free, p1)
	}
	p2, err := r.allocatePort(manifest, "instance-2")
	if err != nil {
		t.Fatalf("allocate second: %v", err)
	}
	if p2 == p1 {
		t.Fatalf("concurrent instances must not share a port: %d", p1)
	}

	r.releasePort("instance-1")
	p3, err := r.allocatePort(manifest, "instance-3")
	if err != nil {
		t.Fatalf("allocate third: %v", err)
	}
	if p3 != free {
		t.Fatalf("released port should be reusable, got %d want %d", p3, free)
	}
}
