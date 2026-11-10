package pluginapp

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"iotserver/internal/models"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/utils"
	"iotserver/internal/tools/pluginpkg"
)

type managedProcess struct {
	cmd     *exec.Cmd
	logFile *os.File
	done    chan struct{}
}

// nativeRuntime 以本机进程方式管理插件实例。
// 每个实例有独立的配置、日志、PID 目录，进程放在独立进程组里便于整组停止。
type nativeRuntime struct {
	root    string
	dataDir string
	lc      logger.LoggingClient

	mu       sync.Mutex
	procs    map[string]*managedProcess
	stopping map[string]bool

	// 端口预留：进程真正 bind 之前，仅靠探测会让并发创建拿到同一个端口。
	portMu sync.Mutex
	ports  map[string]int

	// onExit 由 service 注入，用于把进程退出写回数据库。
	onExit func(instanceId string, err error)
}

func newNativeRuntime(root, dataDir string, lc logger.LoggingClient) *nativeRuntime {
	return &nativeRuntime{
		root:     root,
		dataDir:  dataDir,
		lc:       lc,
		procs:    make(map[string]*managedProcess),
		stopping: make(map[string]bool),
		ports:    make(map[string]int),
	}
}

// Start 启动原生插件实例，返回 PID 和平台访问地址。
func (r *nativeRuntime) Start(library models.DeviceLibrary, instance models.DeviceService) (int, string, error) {
	manifest, err := r.manifestFor(library)
	if err != nil {
		return 0, "", err
	}
	versionDir := filepath.Join(r.root, library.Id, library.Version)
	entry, err := pluginpkg.EntryPath(versionDir, manifest.Entrypoint)
	if err != nil {
		return 0, "", err
	}
	if _, err := os.Stat(entry); err != nil {
		return 0, "", fmt.Errorf("插件入口不存在，请重新安装插件包: %w", err)
	}

	instanceDir := filepath.Join(r.dataDir, instance.Id)
	logDir := filepath.Join(instanceDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return 0, "", err
	}
	port, err := r.allocatePort(manifest, instance.Id)
	if err != nil {
		return 0, "", err
	}
	// 启动失败时释放端口预留，避免泄漏。
	started := false
	defer func() {
		if !started {
			r.releasePort(instance.Id)
		}
	}()
	configPath := filepath.Join(instanceDir, "config.json")
	if err := writePluginConfig(configPath, instance, library, port); err != nil {
		return 0, "", err
	}
	logPath := filepath.Join(logDir, "plugin.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, "", err
	}

	args := make([]string, 0, len(manifest.Args))
	for _, arg := range manifest.Args {
		args = append(args, expandArg(arg, configPath, instanceDir, instance.Id, library.Id, port))
	}
	cmd := exec.Command(entry, args...)
	cmd.Dir = instanceDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(),
		"IOT_PLUGIN_CONFIG="+configPath,
		"IOT_PLUGIN_DATA_DIR="+instanceDir,
		"IOT_PLUGIN_INSTANCE_ID="+instance.Id,
		"IOT_PLUGIN_LIBRARY_ID="+library.Id,
		"IOT_PLUGIN_PORT="+strconv.Itoa(port),
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return 0, "", err
	}
	pid := cmd.Process.Pid
	started = true
	_ = os.WriteFile(filepath.Join(instanceDir, "plugin.pid"), []byte(strconv.Itoa(pid)), 0o644)

	done := make(chan struct{})
	r.mu.Lock()
	r.procs[instance.Id] = &managedProcess{cmd: cmd, logFile: logFile, done: done}
	r.mu.Unlock()

	go func() {
		waitErr := cmd.Wait()
		_ = logFile.Close()
		r.releasePort(instance.Id)
		r.mu.Lock()
		stopping := r.stopping[instance.Id]
		delete(r.procs, instance.Id)
		delete(r.stopping, instance.Id)
		r.mu.Unlock()
		close(done)
		_ = os.Remove(filepath.Join(instanceDir, "plugin.pid"))
		if !stopping && r.onExit != nil {
			r.onExit(instance.Id, waitErr)
		}
	}()

	return pid, "127.0.0.1:" + strconv.Itoa(port), nil
}

// Stop 停止实例：先 SIGTERM，5 秒后仍未退出则 SIGKILL 整个进程组。
func (r *nativeRuntime) Stop(instanceId string) error {
	r.mu.Lock()
	proc := r.procs[instanceId]
	if proc != nil {
		r.stopping[instanceId] = true
	}
	r.mu.Unlock()
	pid := 0
	if proc != nil && proc.cmd.Process != nil {
		pid = proc.cmd.Process.Pid
	} else {
		pidFile := filepath.Join(r.dataDir, instanceId, "plugin.pid")
		pid = readPidFile(pidFile)
		// 平台重启后 PID 可能已被其他进程复用：确认工作目录属于本实例再发信号，
		// 否则会误杀无关进程（进程组）。
		if pid > 0 && !pidMatchesDir(pid, filepath.Join(r.dataDir, instanceId)) {
			_ = os.Remove(pidFile)
			return nil
		}
	}
	if pid <= 0 {
		return nil
	}
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	if proc != nil {
		select {
		case <-proc.done:
			return nil
		case <-time.After(5 * time.Second):
		}
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		select {
		case <-proc.done:
		case <-time.After(time.Second):
		}
		return nil
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	return nil
}

func (r *nativeRuntime) Running(instanceId string) bool {
	r.mu.Lock()
	_, ok := r.procs[instanceId]
	r.mu.Unlock()
	if ok {
		return true
	}
	// 进程重启后 PID 文件里可能是旧 PID，而该 PID 已被其他进程复用。
	// 必须确认它的工作目录属于本实例，否则视为未运行并清理脏 PID 文件。
	pidFile := filepath.Join(r.dataDir, instanceId, "plugin.pid")
	pid := readPidFile(pidFile)
	if pid <= 0 || syscall.Kill(pid, 0) != nil {
		return false
	}
	if !pidMatchesDir(pid, filepath.Join(r.dataDir, instanceId)) {
		_ = os.Remove(pidFile)
		return false
	}
	return true
}

// Logs 读取实例日志的最后 tail 行。
func (r *nativeRuntime) Logs(instanceId string, tail int) ([]string, error) {
	if tail <= 0 {
		tail = 200
	}
	data, err := os.ReadFile(filepath.Join(r.dataDir, instanceId, "logs", "plugin.log"))
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > tail {
		lines = lines[len(lines)-tail:]
	}
	return lines, nil
}

// Remove 停止并删除实例数据目录。
func (r *nativeRuntime) Remove(instanceId string) error {
	_ = r.Stop(instanceId)
	return os.RemoveAll(filepath.Join(r.dataDir, instanceId))
}

func (r *nativeRuntime) manifestFor(library models.DeviceLibrary) (pluginpkg.Manifest, error) {
	if strings.TrimSpace(library.Manifest) != "" {
		return pluginpkg.Parse([]byte(library.Manifest))
	}
	path := filepath.Join(r.root, library.Id, library.Version, pluginpkg.ManifestFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return pluginpkg.Manifest{}, fmt.Errorf("读取插件清单失败: %w", err)
	}
	return pluginpkg.Parse(data)
}

// allocatePort 为实例分配端口，并在进程真正 bind 之前先预留，
// 避免并发创建的两个实例拿到同一个端口。
func (r *nativeRuntime) allocatePort(manifest pluginpkg.Manifest, instanceId string) (int, error) {
	r.portMu.Lock()
	defer r.portMu.Unlock()

	reserve := func(port int) int {
		r.ports[instanceId] = port
		return port
	}
	taken := func(port int) bool {
		for id, p := range r.ports {
			if id != instanceId && p == port {
				return true
			}
		}
		return false
	}

	// 先尝试清单里的默认端口。
	if len(manifest.Ports) > 0 && manifest.Ports[0].Default > 0 {
		preferred := manifest.Ports[0].Default
		if !taken(preferred) {
			if port, err := utils.GetAvailablePort(strconv.Itoa(preferred)); err == nil && port == preferred {
				return reserve(port), nil
			}
		}
	}

	// 默认端口不可用或已被其他实例预留，退回系统分配的临时端口。
	address, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	if err != nil {
		return 0, err
	}
	for i := 0; i < 20; i++ {
		port, portErr := utils.AvailablePort(address)
		if portErr != nil {
			return 0, portErr
		}
		if !taken(port) {
			return reserve(port), nil
		}
	}
	return 0, fmt.Errorf("没有可用的插件端口")
}

// releasePort 释放实例的端口预留（进程退出或启动失败时调用）。
func (r *nativeRuntime) releasePort(instanceId string) {
	r.portMu.Lock()
	delete(r.ports, instanceId)
	r.portMu.Unlock()
}

// pidMatchesDir 判断 pid 的工作目录是否是实例目录。
// Linux 下通过 /proc/<pid>/cwd 校验；其他平台无法校验，按存活处理。
func pidMatchesDir(pid int, dir string) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS != "linux" {
		return true
	}
	cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
	if err != nil {
		return false
	}
	cwd = strings.TrimSuffix(cwd, " (deleted)")
	want := filepath.Clean(dir)
	if resolved, err := filepath.EvalSymlinks(want); err == nil {
		want = resolved
	}
	return filepath.Clean(cwd) == want
}

func writePluginConfig(path string, instance models.DeviceService, library models.DeviceLibrary, port int) error {
	payload := make(map[string]interface{}, len(instance.Config)+6)
	for key, value := range instance.Config {
		payload[key] = value
	}
	payload["port"] = port
	payload["instanceId"] = instance.Id
	payload["libraryId"] = library.Id
	payload["libraryVersion"] = library.Version
	payload["dataDir"] = filepath.Dir(path)
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func expandArg(arg, configPath, dataDir, instanceId, libraryId string, port int) string {
	return strings.NewReplacer(
		"{configPath}", configPath,
		"{config}", configPath,
		"{dataDir}", dataDir,
		"{instanceId}", instanceId,
		"{libraryId}", libraryId,
		"{port}", strconv.Itoa(port),
		"{httpPort}", strconv.Itoa(port),
	).Replace(arg)
}

func readPidFile(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}
