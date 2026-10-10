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
package route

import (
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"

	"iotserver/internal/iotserver/core/container"
	pkgcontainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/i18n"
	"iotserver/internal/pkg/middleware"
	"iotserver/internal/pkg/permissions"
	"iotserver/internal/tools/jwt"
)

func LoadRestRoutes(r *gin.Engine, dic *di.Container) {
	// 调试端点 /debug/pprof：默认关闭（pprof 会暴露 goroutine 栈、堆内存等敏感信息，
	// profile/trace 还可被用来做 CPU DoS）。开启后必须带 x-token 且具备 system:debug:pprof 权限。
	if cfg := container.ConfigurationFrom(dic.Get); cfg.Pprof.Enabled {
		pkgcontainer.LoggingClientFrom(dic.Get).Warnf(
			"pprof 调试端点已开启: /debug/pprof (需要 x-token 且具备 %s 权限)", permissions.SystemDebugPprof)
		pprof.RouteRegister(
			r.Group("/debug", jwt.JWTAuth(false), middleware.PermRequired(dic, permissions.SystemDebugPprof)),
			"pprof")
	}

	r.Use(i18n.I18nHandlerGin())
	r.Use(middleware.CorrelationHeader())

	// load gateway router
	RegisterGateway(r, dic)
	// load open api
	RegisterOpenApi(r, dic)
}
