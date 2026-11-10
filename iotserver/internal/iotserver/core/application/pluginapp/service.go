package pluginapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	pkgcontainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/pkg/utils"
	"iotserver/internal/tools/pluginpkg"
)

type service struct {
	dbClient interfaces.DBClient
	lc       logger.LoggingClient
	root     string
	dataDir  string
	native   *nativeRuntime
}

// New 构造原生插件管理服务。
func New(dic *di.Container) interfaces.PluginApp {
	cfg := container.ConfigurationFrom(dic.Get)
	lc := pkgcontainer.LoggingClientFrom(dic.Get)
	// 统一转绝对路径：插件进程的 cmd.Dir 是实例目录，入口路径不能用相对路径。
	root, _ := filepath.Abs(cfg.PluginRoot())
	dataDir, _ := filepath.Abs(cfg.PluginDataDir())
	s := &service{
		dbClient: container.DBClientFrom(dic.Get),
		lc:       lc,
		root:     root,
		dataDir:  dataDir,
	}
	s.native = newNativeRuntime(s.root, s.dataDir, lc)
	s.native.onExit = s.handleNativeExit
	_ = os.MkdirAll(filepath.Join(s.root, "packages"), 0o755)
	_ = os.MkdirAll(s.dataDir, 0o755)
	return s
}

// UploadPackage 保存 tar.gz、校验清单并解压到插件目录。
func (s *service) UploadPackage(ctx context.Context, filename string, reader io.Reader) (dtos.PluginPackageDTO, error) {
	if !strings.HasSuffix(strings.ToLower(filename), ".tar.gz") && !strings.HasSuffix(strings.ToLower(filename), ".tgz") {
		return dtos.PluginPackageDTO{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("只支持 .tar.gz / .tgz 插件包"))
	}
	tmpDir := filepath.Join(s.root, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return dtos.PluginPackageDTO{}, err
	}
	tmp, err := os.CreateTemp(tmpDir, "upload-*.tar.gz")
	if err != nil {
		return dtos.PluginPackageDTO{}, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, hash), reader)
	closeErr := tmp.Close()
	if err != nil {
		return dtos.PluginPackageDTO{}, err
	}
	if closeErr != nil {
		return dtos.PluginPackageDTO{}, closeErr
	}
	manifest, err := pluginpkg.Load(tmpPath)
	if err != nil {
		return dtos.PluginPackageDTO{}, errort.NewCommonErr(errort.DefaultReqParamsError, err)
	}
	if !manifest.MatchesArch() {
		return dtos.PluginPackageDTO{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("插件架构 %v 与当前平台不匹配", manifest.Arch))
	}

	versionDir := filepath.Join(s.root, manifest.Name, manifest.Version)
	// 先解压到临时目录，成功后再替换已安装版本：
	// 解压中途失败（包截断、磁盘满）不会破坏正在使用的旧版本。
	stagingDir := versionDir + ".staging"
	_ = os.RemoveAll(stagingDir)
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return dtos.PluginPackageDTO{}, err
	}
	if err := pluginpkg.Extract(tmpPath, stagingDir); err != nil {
		_ = os.RemoveAll(stagingDir)
		return dtos.PluginPackageDTO{}, errort.NewCommonErr(errort.DefaultReqParamsError, err)
	}
	if err := replaceDir(stagingDir, versionDir); err != nil {
		_ = os.RemoveAll(stagingDir)
		return dtos.PluginPackageDTO{}, err
	}

	packageDir := filepath.Join(s.root, "packages", manifest.Name)
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		return dtos.PluginPackageDTO{}, err
	}
	packagePath := filepath.Join(packageDir, manifest.Version+".tar.gz")
	if err := copyFile(tmpPath, packagePath); err != nil {
		return dtos.PluginPackageDTO{}, err
	}
	manifestData, _ := json.Marshal(manifest)

	library := models.DeviceLibrary{
		Id:              manifest.Name,
		Name:            manifest.DisplayName,
		Description:     manifest.Description,
		Protocol:        strings.Join(manifest.Protocols, ","),
		Version:         manifest.Version,
		SupportVersions: models.SupportVersions{},
		IsInternal:      true,
		Category:        manifest.Category,
		Runtime:         models.PluginRuntimeNative,
		Arch:            strings.Join(manifest.Arch, ","),
		Manifest:        string(manifestData),
		PackagePath:     packagePath,
		PackageHash:     hex.EncodeToString(hash.Sum(nil)),
		PackageSize:     size,
		Source:          "upload",
		DeviceMarket:    models.DeviceMarket{ClassifyId: 1},
	}
	existing, err := s.dbClient.WithCtx(ctx).DeviceLibraryById(manifest.Name)
	if err == nil {
		if existing.Runtime != "" && existing.Runtime != models.PluginRuntimeNative {
			return dtos.PluginPackageDTO{}, errort.NewCommonErr(errort.DefaultReqParamsError,
				fmt.Errorf("同名驱动已存在且不是原生插件，不能覆盖"))
		}
		library.Created = existing.Created
		library.Essential = existing.Essential
		if err := s.dbClient.WithCtx(ctx).UpdateDeviceLibrary(library); err != nil {
			return dtos.PluginPackageDTO{}, err
		}
	} else {
		added, addErr := s.dbClient.WithCtx(ctx).AddDeviceLibrary(library)
		if addErr != nil {
			return dtos.PluginPackageDTO{}, addErr
		}
		library = added
	}
	return packageDTO(library, manifest), nil
}

func (s *service) ListPackages(ctx context.Context) ([]dtos.PluginPackageDTO, error) {
	// 不按 category 过滤：原生插件包可以是 driver / connector / system，
	// 只要 runtime=native 就应该能创建实例。
	libraries, _, err := s.dbClient.WithCtx(ctx).DeviceLibrariesSearch(0, -1, dtos.DeviceLibrarySearchQueryRequest{})
	if err != nil {
		return nil, err
	}
	result := make([]dtos.PluginPackageDTO, 0)
	for _, library := range libraries {
		if library.Runtime != models.PluginRuntimeNative {
			continue
		}
		manifest, _ := pluginpkg.Parse([]byte(library.Manifest))
		result = append(result, packageDTO(library, manifest))
	}
	return result, nil
}

func (s *service) DeletePackage(ctx context.Context, id string) error {
	instances, _, err := s.dbClient.WithCtx(ctx).DeviceServicesSearch(0, -1, dtos.DeviceServiceSearchQueryRequest{
		DeviceLibraryId: id,
	})
	if err != nil {
		return err
	}
	if len(instances) > 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("该插件还有 %d 个实例，请先删除实例", len(instances)))
	}
	library, err := s.dbClient.WithCtx(ctx).DeviceLibraryById(id)
	if err != nil {
		return err
	}
	if library.Runtime != models.PluginRuntimeNative {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("只能删除原生插件包"))
	}
	if err := s.dbClient.WithCtx(ctx).DeleteDeviceLibraryById(id); err != nil {
		return err
	}
	_ = os.RemoveAll(filepath.Join(s.root, id))
	_ = os.RemoveAll(filepath.Join(s.root, "packages", id))
	return nil
}

func (s *service) CreateInstance(ctx context.Context, req dtos.PluginInstanceCreateRequest) (dtos.PluginInstanceDTO, error) {
	library, err := s.dbClient.WithCtx(ctx).DeviceLibraryById(req.LibraryId)
	if err != nil {
		return dtos.PluginInstanceDTO{}, err
	}
	if library.Runtime != models.PluginRuntimeNative {
		return dtos.PluginInstanceDTO{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("该插件不是原生运行时"))
	}
	versionDir := filepath.Join(s.root, library.Id, library.Version)
	if _, statErr := os.Stat(versionDir); statErr != nil {
		if strings.TrimSpace(library.PackagePath) == "" {
			return dtos.PluginInstanceDTO{}, fmt.Errorf("插件文件不存在: %w", statErr)
		}
		if err := os.MkdirAll(versionDir, 0o755); err != nil {
			return dtos.PluginInstanceDTO{}, err
		}
		if err := pluginpkg.Extract(library.PackagePath, versionDir); err != nil {
			return dtos.PluginInstanceDTO{}, err
		}
	}
	name, nameErr := s.uniqueInstanceName(ctx, library, strings.TrimSpace(req.Name))
	if nameErr != nil {
		return dtos.PluginInstanceDTO{}, nameErr
	}
	instanceId := utils.RandomNum()
	instance := models.DeviceService{
		Id:              instanceId,
		Name:            name,
		InstanceName:    name,
		DeviceLibraryId: library.Id,
		Config:          models.MapStringInterface(req.Config),
		Runtime:         models.PluginRuntimeNative,
		InstallPath:     versionDir,
		DesiredState:    "stopped",
		ActualState:     "stopped",
		HealthStatus:    "unknown",
		DriverType:      1,
	}
	added, err := s.dbClient.WithCtx(ctx).AddDeviceService(instance)
	if err != nil {
		return dtos.PluginInstanceDTO{}, err
	}
	if req.AutoStart {
		if err := s.StartInstance(ctx, added.Id); err != nil {
			_ = s.native.Remove(added.Id)
			_ = s.dbClient.WithCtx(ctx).DeleteDeviceServiceById(added.Id)
			return dtos.PluginInstanceDTO{}, err
		}
		added, _ = s.dbClient.WithCtx(ctx).DeviceServiceById(added.Id)
	}
	return instanceDTO(added, library, s.native.Running(added.Id)), nil
}

func (s *service) ListInstances(ctx context.Context) ([]dtos.PluginInstanceDTO, error) {
	instances, _, err := s.dbClient.WithCtx(ctx).DeviceServicesSearch(0, -1, dtos.DeviceServiceSearchQueryRequest{})
	if err != nil {
		return nil, err
	}
	result := make([]dtos.PluginInstanceDTO, 0)
	for _, instance := range instances {
		if instance.Runtime != models.PluginRuntimeNative {
			continue
		}
		library, libErr := s.dbClient.WithCtx(ctx).DeviceLibraryById(instance.DeviceLibraryId)
		if libErr != nil {
			continue
		}
		result = append(result, instanceDTO(instance, library, s.native.Running(instance.Id)))
	}
	return result, nil
}

func (s *service) StartInstance(ctx context.Context, id string) error {
	instance, library, err := s.load(ctx, id)
	if err != nil {
		return err
	}
	if s.native.Running(id) {
		return nil
	}
	pid, address, startErr := s.native.Start(library, instance)
	if startErr != nil {
		instance.ActualState = "error"
		instance.LastError = startErr.Error()
		_ = s.dbClient.WithCtx(ctx).UpdateDeviceService(instance)
		return startErr
	}
	instance.Pid = pid
	instance.BaseAddress = address
	instance.DesiredState = "running"
	instance.ActualState = "running"
	instance.HealthStatus = "unknown"
	instance.LastError = ""
	return s.dbClient.WithCtx(ctx).UpdateDeviceService(instance)
}

func (s *service) StopInstance(ctx context.Context, id string) error {
	instance, _, err := s.load(ctx, id)
	if err != nil {
		return err
	}
	if err := s.native.Stop(id); err != nil {
		return err
	}
	instance.Pid = 0
	instance.DesiredState = "stopped"
	instance.ActualState = "stopped"
	instance.HealthStatus = "unknown"
	return s.dbClient.WithCtx(ctx).UpdateDeviceService(instance)
}

func (s *service) RestartInstance(ctx context.Context, id string) error {
	if err := s.StopInstance(ctx, id); err != nil {
		return err
	}
	return s.StartInstance(ctx, id)
}

func (s *service) DeleteInstance(ctx context.Context, id string) error {
	instance, _, err := s.load(ctx, id)
	if err != nil {
		return err
	}
	if instance.Runtime != models.PluginRuntimeNative {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("只能删除原生插件实例"))
	}
	if err := s.native.Remove(id); err != nil {
		return err
	}
	return s.dbClient.WithCtx(ctx).DeleteDeviceServiceById(id)
}

func (s *service) InstanceLogs(ctx context.Context, id string, tail int) (dtos.PluginLogsDTO, error) {
	if _, _, err := s.load(ctx, id); err != nil {
		return dtos.PluginLogsDTO{}, err
	}
	lines, err := s.native.Logs(id, tail)
	if err != nil {
		return dtos.PluginLogsDTO{}, err
	}
	return dtos.PluginLogsDTO{Lines: lines}, nil
}

func (s *service) load(ctx context.Context, id string) (models.DeviceService, models.DeviceLibrary, error) {
	instance, err := s.dbClient.WithCtx(ctx).DeviceServiceById(id)
	if err != nil {
		return instance, models.DeviceLibrary{}, err
	}
	if instance.Runtime != models.PluginRuntimeNative {
		return instance, models.DeviceLibrary{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("该实例不是原生插件实例"))
	}
	library, err := s.dbClient.WithCtx(ctx).DeviceLibraryById(instance.DeviceLibraryId)
	if err != nil {
		return instance, library, err
	}
	return instance, library, nil
}

func (s *service) handleNativeExit(instanceId string, exitErr error) {
	ctx := s.dbClient.WithCtx(tenant.Platform(context.Background()))
	instance, err := ctx.DeviceServiceById(instanceId)
	if err != nil {
		return
	}
	instance.Pid = 0
	instance.ActualState = "stopped"
	instance.HealthStatus = "unhealthy"
	if exitErr != nil {
		instance.LastError = exitErr.Error()
	}
	_ = ctx.UpdateDeviceService(instance)
}

func packageDTO(library models.DeviceLibrary, manifest pluginpkg.Manifest) dtos.PluginPackageDTO {
	protocols := manifest.Protocols
	if len(protocols) == 0 && library.Protocol != "" {
		protocols = strings.Split(library.Protocol, ",")
	}
	return dtos.PluginPackageDTO{
		Id:          library.Id,
		Name:        library.Name,
		DisplayName: manifest.DisplayName,
		Version:     library.Version,
		Description: library.Description,
		Category:    library.Category,
		Runtime:     library.Runtime,
		Arch:        library.Arch,
		Protocols:   protocols,
		Essential:   library.Essential,
		Source:      library.Source,
		PackagePath: library.PackagePath,
		PackageHash: library.PackageHash,
		Updated:     library.Modified,
	}
}

func instanceDTO(instance models.DeviceService, library models.DeviceLibrary, running bool) dtos.PluginInstanceDTO {
	status := instance.ActualState
	if running {
		status = "running"
	} else if status == "" {
		status = "stopped"
	}
	config := map[string]interface{}{}
	for key, value := range instance.Config {
		config[key] = value
	}
	return dtos.PluginInstanceDTO{
		Id:           instance.Id,
		Name:         instance.InstanceName,
		LibraryId:    instance.DeviceLibraryId,
		LibraryName:  library.Name,
		Version:      library.Version,
		Runtime:      instance.Runtime,
		Status:       status,
		HealthStatus: instance.HealthStatus,
		Pid:          instance.Pid,
		BaseAddress:  instance.BaseAddress,
		InstallPath:  instance.InstallPath,
		LastError:    instance.LastError,
		Config:       config,
		Updated:      instance.Modified,
	}
}

// uniqueInstanceName 生成同一驱动库下唯一的实例名。
// 用户留空时按插件名自动加序号；显式填了重复名称则返回明确错误，
// 避免直接抛出 (device_library_id, instance_name) 的唯一索引冲突。
func (s *service) uniqueInstanceName(ctx context.Context, library models.DeviceLibrary, requested string) (string, error) {
	base := requested
	auto := base == ""
	if auto {
		base = strings.TrimSpace(library.Name)
	}
	if base == "" {
		base = library.Id
	}
	db := s.dbClient.WithCtx(ctx)
	for i := 1; i <= 100; i++ {
		name := base
		if i > 1 {
			name = fmt.Sprintf("%s-%d", base, i)
		}
		exists, err := db.DeviceServiceInstanceNameExists(library.Id, name)
		if err != nil {
			return "", err
		}
		if !exists {
			return name, nil
		}
		if !auto {
			return "", errort.NewCommonErr(errort.DefaultReqParamsError,
				errort.NewMessageErr("实例名称已存在: {{.p1}}", map[string]interface{}{"p1": base}))
		}
	}
	return "", errort.NewCommonErr(errort.DefaultReqParamsError,
		errort.NewMessageErr("同名实例过多，请手动填写实例名称", nil))
}

// replaceDir 用 src 替换 dst；先备份旧目录，替换失败时恢复。
func replaceDir(src, dst string) error {
	backup := dst + ".old"
	_ = os.RemoveAll(backup)
	if _, err := os.Stat(dst); err == nil {
		if err := os.Rename(dst, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(src, dst); err != nil {
		if _, statErr := os.Stat(backup); statErr == nil {
			_ = os.Rename(backup, dst)
		}
		return err
	}
	_ = os.RemoveAll(backup)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
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
