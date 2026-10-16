/*******************************************************************************
 * Copyright 2017.
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
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"iotserver/internal/iotserver/core/controller/http/openapi"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/middleware"
	"iotserver/internal/tools/jwt"
	"iotserver/internal/tools/openapihelper"
)

func RegisterOpenApi(engine *gin.Engine, dic *di.Container) {
	engine.NoRoute(func(c *gin.Context) {
		// 容错：前端个别接口的 URL 末尾带了空白字符（如
		// /api/v1/rule-engine/{id}/status␠␠、/api/v1/alert-rule/{id}/stop␠，属历史拼写问题），
		// 带 %20 的路径匹配不到任何路由会落到这里。去掉尾部空白后重新派发一次，
		// 避免这类拼写问题让接口永久不可用。
		// 递归安全：重新派发时路径已无尾随空白，若仍不匹配会再次进入本函数并直接走失败分支。
		raw := c.Request.URL.Path
		if trimmed := strings.TrimRight(raw, " \t"); trimmed != raw && trimmed != "" {
			c.Request.URL.Path = trimmed
			c.Request.URL.RawPath = ""
			// gin 的 serveError 在进入 NoRoute 前已把待写状态预设为 404，
			// 而统一响应体写 JSON 时不设置状态码，会把 404 带到状态行上（前端要求 HTTP 200）。
			// 重新派发前显式改回 200，让匹配到的处理器自己决定。
			c.Writer.WriteHeader(http.StatusOK)
			engine.HandleContext(c)
			// HandleContext 会恢复外层索引，必须 Abort 终止 NoRoute 链，
			// 否则外层 Next() 会继续执行已匹配路由的剩余处理器，导致响应被写两次
			c.Abort()
			return
		}
		openapihelper.ReaderFail(c, errort.UrlPathIsInvalid)
	})

	ctl := openapi.New(dic)

	v := engine.Group("/v1.0/openapi")
	v.POST("/auth/login", ctl.Login)
	v.POST("/token/:refreshToken", ctl.RefreshToken)
	v1 := v.Group("", jwt.JWTAuth(false), middleware.TenantContext(), middleware.TenantExpireCheck(dic),
		// 审查日志（操作日志）：OpenAPI 写操作同样入审查日志
		middleware.AuditLog(dic))

	//产品管理的API
	{
		//创建产品
		v1.POST("/product", ctl.OpenApiCreateProduct)
		//更新产品
		v1.PUT("/product/:productId", ctl.OpenApiUpdateProduct)
		//查询产品列表
		v1.GET("/products", ctl.OpenApiProductSearch)
		//查询产品详细信息。
		v1.GET("/product/:productId", ctl.OpenApiProductById)
		// 发布产品
		v1.GET("/product-release/:productId", ctl.OpenApiProductReleaseById)
		// 取消发布产品
		v1.GET("/product-unrelease/:productId", ctl.OpenApiProductUnReleaseById)
		//删除指定产品。
		v1.DELETE("product/:productId", ctl.OpenApiDeleteProduct)
	}
	//设备管理的API
	{
		//创建设备
		v1.POST("/device", ctl.OpenApiCreateDevice)
		//更新设备
		v1.PUT("/device/:deviceId", ctl.OpenApiUpdateDevice)
		//查询设备列表
		v1.GET("/devices", ctl.OpenApiDeviceSearch)
		//查询设备详细信息。
		v1.GET("/device/:deviceId", ctl.OpenApiDeviceById)
		//删除指定设备。
		v1.DELETE("/device/:deviceId", ctl.OpenApiDeleteDevice)
		//获取设备的运行状态。
		//v1.GET("/deviceStatus/:deviceId", ctl.OpenApiDeviceStatus)

	}
	//物模型管理的API
	{
		//为指定产品的物模型新增功能
		v1.POST("/thingModel", ctl.OpenApiThingModelAddOrUpdate)
		//更新指定产品物模型中的单个功能
		v1.PUT("/thingModel", ctl.OpenApiThingModelAddOrUpdate)
		//查看指定产品的物模型中的功能定义详情
		v1.GET("/thingModel", ctl.OpenApiThingModel)
		//DeleteThingModel
		v1.DELETE("/thingModel", ctl.OpenApiDeleteThingModel)

	}
	//物模型使用的API
	{
		//查询设备实时属性数据。
		v1.GET("/queryDeviceEffectivePropertyData", ctl.OpenApiQueryDeviceEffectivePropertyData)
		//设置设备的属性。
		v1.POST("/setDeviceProperty", ctl.OpenApiSetDeviceProperty)
		//调用设备的服务。
		v1.POST("/invokeThingService", ctl.OpenApiInvokeThingService)
		//查询设备的属性历史数据。
		v1.GET("/queryDevicePropertyData", ctl.OpenApiQueryDevicePropertyData)
		//查询设备的事件历史数据。
		v1.GET("/queryDeviceEventData", ctl.OpenApiQueryDeviceEventData)
		//获取设备的服务记录历史数据。
		v1.GET("/queryDeviceServiceData", ctl.OpenApiQueryDeviceServiceData)
	}
}
