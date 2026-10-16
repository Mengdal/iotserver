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
package docker

import (
	"iotserver/internal/dtos"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/utils"
	"net"
	"time"
)

// RemoveApp 删除App文件
func (d *dockerImpl) RemoveApp(app dtos.DeviceLibrary) error {
	return d.dm.ImageRemove(app.DockerImageId)
}

// GetSelfIp 返回驱动容器应该连的"平台地址"。
//
// 场景一：平台跑在容器里 → 返回该容器的网络 IP，驱动容器在同一 docker 网络内直连。
// 场景二：平台跑在本机进程（inspect 不到自己）→ 返回 host.docker.internal，
// 因为此时驱动容器要连的是宿主机；拿 127.0.0.1 会指向容器自己，连不上。
func (d *dockerImpl) GetSelfIp() string {
	ip, err := d.dm.GetContainerIp(d.dcm.DockerSelfName)
	if err != nil || ip == "" {
		d.lc.Infof("platform is not in a container, drivers will connect via %s", constants.HostDockerInternal)
		return constants.HostDockerInternal
	}
	return ip
}

// IsLocalMode 平台是否以本机进程运行（见 setDcmRootDir 的降级分支）
func (d *dockerImpl) IsLocalMode() bool {
	return d.dcm.IsLocalMode()
}

// instance
func (d *dockerImpl) InstanceState(ins dtos.DeviceService) bool {
	// 先判断docker 是否在运行中
	stats, err := d.dm.GetContainerRunStatus(ins.ContainerName)
	if err != nil {
		d.lc.Errorf("GetContainerRunStatus err:%v", err)
		return false
	}
	if stats != constants.ContainerRunStatusRunning {
		return false
	}

	// 在通过ping 实例服务存在否
	client, err := net.DialTimeout("tcp", ins.BaseAddress, 2*time.Second)
	defer func() {
		if client != nil {
			_ = client.Close()
		}
	}()
	if err != nil {
		return false
	}

	return true
}

func (d *dockerImpl) StartInstance(ins dtos.DeviceService, cfg dtos.RunServiceCfg) (string, error) {
	// 关闭自定义开关
	if !ins.DockerParamsSwitch {
		cfg.DockerParams = ""
	}
	filePath, err := d.genRunServiceConfig(ins.ContainerName, cfg.RunConfig, constants.DriverInstance)
	if err != nil {
		return "", err
	}
	ip, err := d.dm.ContainerStart(cfg.ImageRepo, ins.ContainerName, filePath, cfg.DockerMountDevices, cfg.DockerParams, constants.DriverInstance, cfg.DriverPort)
	return ip, err
}

func (d *dockerImpl) StopInstance(ins dtos.DeviceService) error {
	err := d.dm.ContainerStop(ins.ContainerName)
	if err != nil {
		return err
	}
	return nil
}

func (d *dockerImpl) DeleteInstance(ins dtos.DeviceService) error {
	// 删除容器
	err := d.dm.ContainerRemove(ins.ContainerName)
	if err != nil {
		return err
	}
	paths := []string{
		d.dcm.GetRunConfigPath(ins.ContainerName),
		d.dcm.GetMntDir(ins.ContainerName),
	}
	for _, v := range paths {
		err = utils.RemoveFileOrDir(v)
		if err != nil {
			d.lc.Errorf("RemoveFileOrDir [%s] err %v", v, err)
		}
	}
	return nil
}
