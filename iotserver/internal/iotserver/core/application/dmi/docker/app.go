package docker

import (
	"context"
	"fmt"
	"github.com/docker/docker/errdefs"
	"io/ioutil"
	"iotserver/internal/dtos"
	resourceContainer "iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/pkg/utils"
	"path"
	"strings"
	"sync"
)

var _ interfaces.DMI = new(dockerImpl)

type dockerImpl struct {
	dic *di.Container
	lc  logger.LoggingClient
	dm  *DockerManager
	dcm *dtos.DriverConfigManage
}

func (d *dockerImpl) GetDriverInstanceLogPath(serviceName string) string {
	return d.dcm.GetHostLogFilePath(serviceName)
}
func New(dic *di.Container, ctx context.Context, wg *sync.WaitGroup, dcm dtos.DriverConfigManage) (*dockerImpl, error) {
	dm, err := NewDockerManager(ctx, dic, &dcm)
	if err != nil {
		return nil, err
	}
	dil := &dockerImpl{
		dic: dic,
		lc:  container.LoggingClientFrom(dic.Get),
		dm:  dm,
		dcm: &dcm,
	}
	dil.setDcmRootDir()
	return dil, nil
}
func (d *dockerImpl) StopAllInstance() {
	dbClient := resourceContainer.DBClientFrom(d.dic.Get)

	// 平台作用域：系统路径，不按租户过滤
	deviceService, _, err := dbClient.WithCtx(tenant.Platform(context.Background())).DeviceServicesSearch(0, -1, dtos.DeviceServiceSearchQueryRequest{})
	if err != nil {
		d.lc.Errorf("search service error :", err.Error())
	}
	for _, service := range deviceService {
		d.lc.Info(fmt.Sprintf("stop docker instance[%s]", service.ContainerName))
		err := d.StopInstance(dtos.DeviceService{ContainerName: service.ContainerName})
		if err != nil {
			d.lc.Error(fmt.Sprintf("stop docker instance[%s] error:", err.Error()))
		}
	}
}

// DownApp 下载应用
func (d *dockerImpl) DownApp(cfg dtos.DockerConfig, app dtos.DeviceLibrary, toVersion string) (string, error) {
	authToken, err := d.getAuthToken(cfg.Address, cfg.Account, cfg.Password, cfg.SaltKey)
	if err != nil {
		return "", err
	}

	// 2. pull images
	return d.getApp(authToken, d.getImageUrl(cfg.Address, app.DockerRepoName, toVersion))
}

// getAuthToken 获取 docker 认证 token
func (d *dockerImpl) getAuthToken(address, account, pass, salt string) (string, error) {
	if account == "" || pass == "" {
		return "", nil
	}

	// 处理docker密码
	rawPassword, err := utils.DecryptAuthPassword(pass, salt)
	if err != nil {
		d.lc.Errorf("3.getAuthToken docker id:%s, account:%s, password err:%n", address, account, err)
		return "", err
	}
	return d.dm.GetAuthToken(account, rawPassword, address), nil
}

// getApp 下载镜像
func (d *dockerImpl) getApp(token, imageUrl string) (string, error) {
	dockerImageId, dockerErr := d.dm.PullDockerImage(imageUrl, token)
	if dockerErr != nil {
		code := errort.DefaultSystemError
		err := fmt.Errorf("driver library download error")
		if errdefs.IsUnauthorized(dockerErr) || errdefs.IsForbidden(dockerErr) ||
			strings.Contains(dockerErr.Error(), "denied") ||
			strings.Contains(dockerErr.Error(), "unauthorized") {
			code = errort.DeviceLibraryDockerAuthInvalid
			err = fmt.Errorf("docker auth invalid")
		} else if errdefs.IsNotFound(dockerErr) || strings.Contains(dockerErr.Error(), "not found") {
			code = errort.DeviceLibraryDockerImagesNotFound
			err = fmt.Errorf("docker images not found")
		} else if strings.Contains(dockerErr.Error(), "invalid reference format") {
			code = errort.DeviceLibraryDockerImagesNotFound
			err = fmt.Errorf("docker images not found, url invalid")
		}
		d.lc.Errorf("4.getApp imageUrl %s, PullDockerImage err:%v", imageUrl, dockerErr)

		return "", errort.NewCommonErr(code, err)
	}

	return dockerImageId, nil
}

func (d *dockerImpl) getImageUrl(address, repoName, version string) string {
	return path.Clean(address+"/"+repoName) + ":" + version
}

// StateApp 驱动软件下载情况
func (d *dockerImpl) StateApp(dockerImageId string) bool {
	return d.dm.ExistImageById(dockerImageId)
}

// GetAllApp 获取所有镜像信息
func (d *dockerImpl) GetAllApp() []string {
	return d.dm.GetAllImagesIds()
}

func (d *dockerImpl) setDcmRootDir() {
	res, err := d.dm.GetContainerInspect(d.dcm.DockerSelfName)
	if err != nil {
		// 平台没跑在容器里（本机进程模式，如 macOS 上 make run / make start）：
		// inspect 不到自己，容器路径 /var/bin/iotserver 既不存在也没权限创建
		// （报 mkdir /var/bin: permission denied），退回本地工作目录。
		// 此时"平台视角"与"宿主视角"是同一个目录，两个 root 取同值，
		// 驱动容器的 bind mount 源就是本地这个绝对路径。
		local := utils.GetPwdDir()
		d.lc.Infof("platform is not running in a container, use local driver dir: %s", local)
		d.dcm.SetContainerRootDir(local)
		d.dcm.SetHostRootDir(local)
		d.dcm.SetLocalMode(true)
		return
	}

	for _, v := range res.Mounts {
		if v.Destination == constants.DockerIotServerRootDir {
			d.dcm.SetHostRootDir(v.Source)
			break
		}
	}
	networkName := res.HostConfig.NetworkMode.UserDefined()
	d.dcm.SetNetworkName(networkName)
}

func (d *dockerImpl) genRunServiceConfig(name string, cfgContent string, instanceType constants.InstanceType) (string, error) {
	var err error
	var filePath string
	if instanceType == constants.CloudInstance {

	} else if instanceType == constants.DriverInstance {
		filePath = d.dcm.GetRunConfigPath(name)
		err = utils.CreateDirIfNotExist(d.dcm.GetRunConfigDir())
		if err != nil {
			return "", err
		}
	}
	err = ioutil.WriteFile(filePath, []byte(cfgContent), 0644)

	if err != nil {
		d.lc.Error("err:", err)
		return "", err
	}
	if instanceType == constants.CloudInstance {

	} else if instanceType == constants.DriverInstance {
		return d.dcm.GetHostRunConfigPath(name), nil
	}
	return "", nil
}
