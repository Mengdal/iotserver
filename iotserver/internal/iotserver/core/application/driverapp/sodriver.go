package driverapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/config"
	"iotserver/internal/iotserver/core/container"
	"iotserver/internal/models"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/utils"
)

// UploadSODriver 保存上传的 .so 驱动并安装到 iotpGW 的驱动目录。
func (app *driverLibApp) UploadSODriver(ctx context.Context, req dtos.SODriverUploadRequest, reader io.Reader) (dtos.SODriverDTO, error) {
	cfg := container.ConfigurationFrom(app.dic.Get)
	if cfg.DeploymentMode() != config.DeploymentModeEmbedded {
		return dtos.SODriverDTO{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("只有嵌入式部署支持安装 .so 驱动"))
	}
	if app.so == nil || !app.so.enabled() {
		return dtos.SODriverDTO{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("未配置 [CollectEngine].DriverDir"))
	}

	fileName := filepath.Base(strings.TrimSpace(req.FileName))
	if err := validateSOFileName(fileName); err != nil {
		return dtos.SODriverDTO{}, errort.NewCommonErr(errort.DefaultReqParamsError, err)
	}
	// Phase 1 不做多版本共存：同一个 .so 文件名只允许一条记录，先卸载再升级。
	existing, _, err := app.dbClient.WithCtx(ctx).DeviceLibrariesSearch(0, -1, dtos.DeviceLibrarySearchQueryRequest{
		Runtime: models.PluginRuntimeIotpGW,
	})
	if err != nil {
		return dtos.SODriverDTO{}, err
	}
	for _, library := range existing {
		manifest, _ := parseSOManifest(library.Manifest)
		if strings.EqualFold(manifest.FileName, fileName) {
			return dtos.SODriverDTO{}, errort.NewCommonErr(errort.DefaultNameRepeat,
				fmt.Errorf("驱动文件 %s 已存在，请先卸载", fileName))
		}
	}
	arch := strings.TrimSpace(req.Arch)
	if arch == "" {
		arch = runtime.GOARCH
	}
	if err := validateSOArch(arch); err != nil {
		return dtos.SODriverDTO{}, errort.NewCommonErr(errort.DefaultReqParamsError, err)
	}
	abi := strings.TrimSpace(req.ABI)
	expectedABI := strings.TrimSpace(cfg.CollectEngine.DriverABI)
	if expectedABI != "" && abi != "" && abi != expectedABI {
		return dtos.SODriverDTO{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("驱动 ABI %s 与 iotpGW 期望的 %s 不一致", abi, expectedABI))
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.TrimSuffix(fileName, filepath.Ext(fileName))
	}
	version := strings.TrimSpace(req.Version)
	if version == "" {
		version = "1.0.0"
	}

	id := "so-" + utils.RandomNum()
	artifactDir := filepath.Join(app.so.artifactDir(), id)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return dtos.SODriverDTO{}, errort.NewCommonEdgeX(errort.DefaultSystemError, "创建驱动目录失败", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(artifactDir)
		}
	}()

	packagePath := filepath.Join(artifactDir, fileName)
	file, err := os.Create(packagePath)
	if err != nil {
		return dtos.SODriverDTO{}, errort.NewCommonEdgeX(errort.DefaultSystemError, "保存驱动文件失败", err)
	}
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(reader, maxSODriverSize+1))
	closeErr := file.Close()
	if copyErr != nil {
		return dtos.SODriverDTO{}, errort.NewCommonEdgeX(errort.DefaultSystemError, "保存驱动文件失败", copyErr)
	}
	if closeErr != nil {
		return dtos.SODriverDTO{}, errort.NewCommonEdgeX(errort.DefaultSystemError, "保存驱动文件失败", closeErr)
	}
	if size > maxSODriverSize {
		return dtos.SODriverDTO{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("驱动文件不能超过 %d MB", maxSODriverSize>>20))
	}
	actualHash := hex.EncodeToString(hash.Sum(nil))
	if requested := strings.TrimSpace(req.SHA256); requested != "" && !strings.EqualFold(requested, actualHash) {
		return dtos.SODriverDTO{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("驱动文件 SHA256 与请求不一致"))
	}

	manifestData, _ := json.Marshal(soManifest{FileName: fileName, ABI: abi})
	library := models.DeviceLibrary{
		Id:              id,
		Name:            name,
		Description:     req.Description,
		Protocol:        req.Protocol,
		Version:         version,
		Arch:            arch,
		Runtime:         models.PluginRuntimeIotpGW,
		Category:        models.PluginCategoryDriver,
		IsInternal:      true,
		Source:          "upload",
		PackagePath:     packagePath,
		PackageHash:     actualHash,
		PackageSize:     size,
		Manifest:        string(manifestData),
		SupportVersions: models.SupportVersions{{Version: version, IsDefault: true}},
		DeviceMarket:    models.DeviceMarket{ClassifyId: 1},
	}
	library.Created = utils.MakeTimestamp()
	library.Modified = library.Created
	created, err := app.dbClient.WithCtx(ctx).AddDeviceLibrary(library)
	if err != nil {
		return dtos.SODriverDTO{}, err
	}
	if err = app.so.install(ctx, created, soManifest{FileName: fileName, ABI: abi}); err != nil {
		_ = app.dbClient.WithCtx(ctx).DeleteDeviceLibraryById(created.Id)
		return dtos.SODriverDTO{}, err
	}
	cleanup = false
	return soDTO(created, soManifest{FileName: fileName, ABI: abi}, true), nil
}

// ListSODrivers 返回平台登记过的 iotpGW .so 驱动及其安装状态。
func (app *driverLibApp) ListSODrivers(ctx context.Context) ([]dtos.SODriverDTO, error) {
	libraries, _, err := app.dbClient.WithCtx(ctx).DeviceLibrariesSearch(0, -1, dtos.DeviceLibrarySearchQueryRequest{
		Runtime: models.PluginRuntimeIotpGW,
	})
	if err != nil {
		return nil, err
	}
	result := make([]dtos.SODriverDTO, 0, len(libraries))
	for _, library := range libraries {
		manifest, _ := parseSOManifest(library.Manifest)
		installed := app.so != nil && app.so.installed(library)
		result = append(result, soDTO(library, manifest, installed))
	}
	return result, nil
}

// DeleteSODriver 从 iotpGW 卸载 .so 驱动，并删除平台登记和安装包。
func (app *driverLibApp) DeleteSODriver(ctx context.Context, id string) error {
	library, err := app.dbClient.WithCtx(ctx).DeviceLibraryById(id)
	if err != nil {
		return err
	}
	if library.Runtime != models.PluginRuntimeIotpGW {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("不是 iotpGW .so 驱动"))
	}
	if app.so == nil || !app.so.enabled() {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("未配置 [CollectEngine].DriverDir"))
	}
	manifest, err := parseSOManifest(library.Manifest)
	if err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "驱动元数据损坏", err)
	}
	if app.so != nil {
		if err = app.so.uninstall(ctx, manifest); err != nil {
			return err
		}
	}
	if err = app.dbClient.WithCtx(ctx).DeleteDeviceLibraryById(id); err != nil {
		return err
	}
	if strings.TrimSpace(library.PackagePath) != "" {
		_ = os.RemoveAll(filepath.Dir(library.PackagePath))
	}
	return nil
}

func soDTO(library models.DeviceLibrary, manifest soManifest, installed bool) dtos.SODriverDTO {
	return dtos.SODriverDTOFromModel(library, manifest.FileName, manifest.ABI, installed)
}
