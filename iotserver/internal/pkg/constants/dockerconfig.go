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
package constants

// 「驱动市场」内置驱动的镜像仓库默认值。
//
// 市场里的驱动（device_library.is_internal = 1）不走用户在「镜像仓库管理」里配的凭据，
// 而是直接用这里的固定地址拉取，因此：
//   - 地址指向本项目的镜像仓库（ghcr.io）；
//   - 账号/密码为空 = 匿名拉取，要求镜像包是公开的。
//
// 如果要把市场驱动放在私有仓库，需要改造 downloadVersion，让 is_internal 的驱动
// 也能关联 docker_config（目前是刻意跳过的，见 driverapp/manage.go:187）。
const (
	Address  = "ghcr.io"
	Account  = ""
	Password = ""
	SaltKey  = ""
)

