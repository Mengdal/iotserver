/*******************************************************************************
 * Copyright 2017 Dell Inc.
 * Copyright (c) 2019 Intel Corporation
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *******************************************************************************/
package dtos

import (
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/utils"
	"sync"
)

var dcMutex sync.Mutex

// DriverConfigManage 驱动实例的目录与 Docker 运行参数。
//
// 目录有两套视角，因为平台和驱动是**两个容器**：
//   - ContainerRootDir：平台自己看到的根目录，配置/日志都写在这里；
//   - HostRootDir：宿主机上与之对应的真实目录，作为驱动容器 bind mount 的源。
//
// 容器部署时两者不同（容器内 /var/bin/iotserver ↔ 宿主上某个目录），
// 由 setDcmRootDir 从平台自身的挂载信息反推；
// 本机进程部署时（如 macOS 上 make run）平台不在容器里，两者取同一个本地目录。
type DriverConfigManage struct {
	// HostRootDir 宿主机目录：驱动容器 bind mount 的源路径
	HostRootDir string
	// ContainerRootDir 平台视角的根目录：驱动配置/日志的写入位置。
	// 留空时退回 constants.DockerIotServerRootDir（容器部署的默认值）
	ContainerRootDir string
	// LocalMode 平台以本机进程运行（不是容器）。
	// 此时驱动容器不在同一 docker 网络内：地址要用 host.docker.internal / 127.0.0.1，
	// 且驱动的 gRPC 端口要发布到宿主机，平台才连得回去。
	LocalMode bool
	NetWorkName string
	DockerManageConfig
}

type DockerManageConfig struct {
	ContainerConfigPath string // 容器内部驱动运行启动的配置文件路径
	DockerApiVersion    string // docker 版本号
	DockerRunMode       string // 默认运行模式 默认是host
	DockerSelfName      string // edge启动的容器名
	Privileged          bool   // 容器特权
}

func (m *DriverConfigManage) SetHostRootDir(dir string) {
	dcMutex.Lock()
	defer dcMutex.Unlock()
	m.HostRootDir = dir
}

// SetContainerRootDir 设置平台视角的根目录（本机进程模式下由 setDcmRootDir 调用）
func (m *DriverConfigManage) SetContainerRootDir(dir string) {
	dcMutex.Lock()
	defer dcMutex.Unlock()
	m.ContainerRootDir = dir
}

// SetLocalMode 标记平台以本机进程运行
func (m *DriverConfigManage) SetLocalMode(local bool) {
	dcMutex.Lock()
	defer dcMutex.Unlock()
	m.LocalMode = local
}

// IsLocalMode 平台是否以本机进程运行
func (m *DriverConfigManage) IsLocalMode() bool {
	dcMutex.Lock()
	defer dcMutex.Unlock()
	return m.LocalMode
}

func (m *DriverConfigManage) SetNetworkName(networkName string) {
	dcMutex.Lock()
	defer dcMutex.Unlock()
	m.NetWorkName = networkName
}

// containerRoot 平台视角根目录；未设置时用内置默认（容器部署）
func (m *DriverConfigManage) containerRoot() string {
	if m.ContainerRootDir != "" {
		return m.ContainerRootDir
	}
	return constants.DockerIotServerRootDir
}

// hostRoot 宿主视角根目录；未设置时与平台视角一致
// （本机进程模式下容器与宿主就是同一台机器、同一个目录）
func (m *DriverConfigManage) hostRoot() string {
	if m.HostRootDir != "" {
		return m.HostRootDir
	}
	return m.containerRoot()
}

func (m *DriverConfigManage) GetLibraryDir() string {
	return utils.GetPwdDir() + "/" + constants.DriverLibraryDir + "/"
}

func (m *DriverConfigManage) GetBinPath(serverName string) string {
	return utils.GetPwdDir() + "/" + constants.DriverBinDir + "/" + serverName
}

// GetRunConfigPath 驱动运行配置的写入路径（平台视角）
func (m *DriverConfigManage) GetRunConfigPath(serviceName string) string {
	return m.containerRoot() + "/" + constants.DriverRunConfigDir + "/" + serviceName + constants.ConfigSuffix
}

// GetHostRunConfigPath 驱动运行配置在宿主上的路径（驱动容器的 mount 源）
func (m *DriverConfigManage) GetHostRunConfigPath(serviceName string) string {
	return m.hostRoot() + "/" + constants.DriverRunConfigDir + "/" + serviceName + constants.ConfigSuffix
}

// GetRunConfigDir 驱动运行配置目录（平台视角），首次写入前要先创建
func (m *DriverConfigManage) GetRunConfigDir() string {
	return m.containerRoot() + "/" + constants.DriverRunConfigDir
}

// GetMntDir 驱动日志目录（平台视角，删除实例时清理）
func (m *DriverConfigManage) GetMntDir(serviceName string) string {
	return m.containerRoot() + "/" + constants.DriverMntDir + "/" + serviceName
}

// GetHostMntDir 驱动日志目录在宿主上的路径（驱动容器的 mount 源）
func (m *DriverConfigManage) GetHostMntDir(serviceName string) string {
	return m.hostRoot() + "/" + constants.DriverMntDir + "/" + serviceName
}

func (m *DriverConfigManage) GetLogFilePath(serviceName string) string {
	return utils.GetPwdDir() + "/" + constants.DriverMntDir + "/" + serviceName + "/" + constants.DriverDefaultLogPath
}

func (m *DriverConfigManage) GetHostLogFilePath(serviceName string) string {
	return m.hostRoot() + "/" + constants.DriverMntDir + "/" + serviceName + "/" + constants.DriverDefaultLogPath
}