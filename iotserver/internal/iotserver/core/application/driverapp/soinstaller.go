package driverapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"iotserver/internal/iotserver/core/config"
	"iotserver/internal/models"
	"iotserver/internal/pkg/logger"
)

const maxSODriverSize = 100 << 20 // 100MB

// soManifest 存在 DeviceLibrary.Manifest 里的 .so 驱动元数据。
type soManifest struct {
	FileName string `json:"fileName"`
	ABI      string `json:"abi"`
}

type soInstaller struct {
	cfg *config.ConfigurationStruct
	lc  logger.LoggingClient
}

func newSOInstaller(cfg *config.ConfigurationStruct, lc logger.LoggingClient) *soInstaller {
	return &soInstaller{cfg: cfg, lc: lc}
}

func (s *soInstaller) enabled() bool {
	return s != nil && s.cfg != nil && strings.TrimSpace(s.cfg.CollectEngine.DriverDir) != ""
}

func (s *soInstaller) driverDir() string {
	return strings.TrimSpace(s.cfg.CollectEngine.DriverDir)
}

func (s *soInstaller) backupDir() string {
	if dir := strings.TrimSpace(s.cfg.CollectEngine.BackupDir); dir != "" {
		return dir
	}
	return filepath.Join(s.cfg.PluginDataDir(), "driver-backups")
}

func (s *soInstaller) artifactDir() string {
	return filepath.Join(s.cfg.PluginDataDir(), "so-drivers")
}

func (s *soInstaller) targetPath(fileName string) string {
	return filepath.Join(s.driverDir(), fileName)
}

func (s *soInstaller) installed(library models.DeviceLibrary) bool {
	if !s.enabled() {
		return false
	}
	manifest, err := parseSOManifest(library.Manifest)
	if err != nil || manifest.FileName == "" {
		return false
	}
	info, err := os.Stat(s.targetPath(manifest.FileName))
	return err == nil && !info.IsDir()
}

func (s *soInstaller) install(ctx context.Context, library models.DeviceLibrary, manifest soManifest) error {
	if !s.enabled() {
		return errors.New("未配置 [CollectEngine].DriverDir")
	}
	if err := validateSOFileName(manifest.FileName); err != nil {
		return err
	}
	if strings.TrimSpace(library.PackagePath) == "" {
		return errors.New("驱动安装包路径为空")
	}
	src, err := os.Open(library.PackagePath)
	if err != nil {
		return fmt.Errorf("打开驱动安装包失败: %w", err)
	}
	defer src.Close()

	dir := s.driverDir()
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建驱动目录失败: %w", err)
	}
	staging := filepath.Join(dir, ".staging")
	if err = os.MkdirAll(staging, 0o755); err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	tmp, err := os.CreateTemp(staging, manifest.FileName+".*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	hash := sha256.New()
	if _, err = io.Copy(io.MultiWriter(tmp, hash), src); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("写入驱动文件失败: %w", err)
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("同步驱动文件失败: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("关闭驱动文件失败: %w", err)
	}
	if library.PackageHash != "" && !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), library.PackageHash) {
		return errors.New("驱动文件哈希与登记值不一致")
	}
	if err = os.Chmod(tmpPath, 0o755); err != nil {
		return fmt.Errorf("设置驱动文件权限失败: %w", err)
	}

	target := s.targetPath(manifest.FileName)
	backup, hadOld, err := s.backupTarget(target, manifest.FileName)
	if err != nil {
		return err
	}
	if err = os.Rename(tmpPath, target); err != nil {
		return fmt.Errorf("替换驱动文件失败: %w", err)
	}
	if err = s.reload(ctx); err != nil {
		s.rollback(target, backup, hadOld)
		return err
	}
	return nil
}

func (s *soInstaller) uninstall(ctx context.Context, manifest soManifest) error {
	if !s.enabled() {
		return errors.New("未配置 [CollectEngine].DriverDir")
	}
	if err := validateSOFileName(manifest.FileName); err != nil {
		return err
	}
	target := s.targetPath(manifest.FileName)
	if _, err := os.Stat(target); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	backup, _, err := s.backupTarget(target, manifest.FileName)
	if err != nil {
		return err
	}
	if err = os.Remove(target); err != nil {
		return fmt.Errorf("删除驱动文件失败: %w", err)
	}
	if err = s.reload(ctx); err != nil {
		_ = copyFile(backup, target)
		_ = s.reload(ctx)
		return err
	}
	return nil
}

func (s *soInstaller) backupTarget(target, fileName string) (string, bool, error) {
	if _, err := os.Stat(target); err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	dir := s.backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", false, fmt.Errorf("创建备份目录失败: %w", err)
	}
	backup := filepath.Join(dir, fmt.Sprintf("%s.%s", fileName, time.Now().Format("20060102150405")))
	if err := copyFile(target, backup); err != nil {
		return "", false, fmt.Errorf("备份驱动文件失败: %w", err)
	}
	return backup, true, nil
}

func (s *soInstaller) rollback(target, backup string, hadOld bool) {
	if hadOld && backup != "" {
		if err := copyFile(backup, target); err == nil {
			_ = s.reload(context.Background())
			return
		}
	}
	_ = os.Remove(target)
}

func (s *soInstaller) reload(ctx context.Context) error {
	if s.cfg == nil {
		return nil
	}
	mode := strings.ToLower(strings.TrimSpace(s.cfg.CollectEngine.ReloadMode))
	switch mode {
	case "", "none":
		return nil
	case "command":
		command := strings.TrimSpace(s.cfg.CollectEngine.ReloadCommand)
		if command == "" {
			return errors.New("[CollectEngine].ReloadCommand 为空")
		}
		runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		out, err := exec.CommandContext(runCtx, "/bin/sh", "-c", command).CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				return fmt.Errorf("重新加载 iotpGW 失败: %w", err)
			}
			return fmt.Errorf("重新加载 iotpGW 失败: %s: %w", msg, err)
		}
		return nil
	case "api":
		return errors.New("[CollectEngine].ReloadMode=api 尚未实现")
	default:
		return fmt.Errorf("未知的 ReloadMode: %s", mode)
	}
}

func parseSOManifest(raw string) (soManifest, error) {
	var manifest soManifest
	if strings.TrimSpace(raw) == "" {
		return manifest, errors.New("驱动缺少 manifest 元数据")
	}
	if err := json.Unmarshal([]byte(raw), &manifest); err != nil {
		return manifest, err
	}
	return manifest, validateSOFileName(manifest.FileName)
}

func validateSOFileName(name string) error {
	if name == "" || filepath.Base(name) != name || strings.HasPrefix(name, ".") {
		return errors.New("驱动文件名不合法")
	}
	if !strings.HasSuffix(strings.ToLower(name), ".so") {
		return errors.New("驱动文件必须是 .so")
	}
	if strings.ContainsAny(name, `/\`) {
		return errors.New("驱动文件名不能包含路径分隔符")
	}
	return nil
}

func validateSOArch(arch string) error {
	arch = strings.ToLower(strings.TrimSpace(arch))
	if arch == "" {
		arch = runtime.GOARCH
	}
	if arch == "any" || arch == runtime.GOARCH {
		return nil
	}
	return fmt.Errorf("驱动架构 %s 与当前平台 %s 不匹配", arch, runtime.GOARCH)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
