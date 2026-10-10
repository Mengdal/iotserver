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
package gateway

import (
	"github.com/gin-gonic/gin"
	"iotserver/internal/iotserver/core/config"
	"iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	pkgcontainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/middleware"
)

type controller struct {
	dic *di.Container
	lc  logger.LoggingClient
	cfg *config.ConfigurationStruct
}

func New(dic *di.Container) *controller {
	lc := pkgcontainer.LoggingClientFrom(dic.Get)
	cfg := container.ConfigurationFrom(dic.Get)
	return &controller{
		dic: dic,
		lc:  lc,
		cfg: cfg,
	}
}

func (ctl *controller) getDriverLibApp() interfaces.DriverLibApp {
	return container.DriverAppFrom(ctl.dic.Get)
}

func (ctl *controller) getSceneApp() interfaces.SceneItf {
	return container.SceneItfFrom(ctl.dic.Get)
}

func (ctl *controller) getRuleEngineApp() interfaces.RuleEngineItf {
	return container.RuleEngineItfFrom(ctl.dic.Get)
}

func (ctl *controller) getResourceApp() interfaces.ResourceItf {
	return container.ResourceItfFrom(ctl.dic.Get)
}

func (ctl *controller) getUserApp() interfaces.UserItf {
	return container.UserItfFrom(ctl.dic.Get)
}

func (ctl *controller) getDriverServiceApp() interfaces.DriverServiceApp {
	return container.DriverServiceAppFrom(ctl.dic.Get)
}

func (ctl *controller) getSystemMonitorApp() interfaces.MonitorItf {
	return container.MonitorAppNameFrom(ctl.dic.Get)
}

func (ctl *controller) getLanguageApp() interfaces.LanguageSDKApp {
	return container.LanguageAppNameFrom(ctl.dic.Get)
}

func (ctl *controller) getProductApp() interfaces.ProductItf {
	return container.ProductAppNameFrom(ctl.dic.Get)
}

func (ctl *controller) getDeviceApp() interfaces.DeviceItf {
	return container.DeviceItfFrom(ctl.dic.Get)
}

func (ctl *controller) getPersistApp() interfaces.PersistItf {
	return container.PersistItfFrom(ctl.dic.Get)
}

func (ctl *controller) getCategoryTemplateApp() interfaces.CategoryApp {
	return container.CategoryTemplateAppFrom(ctl.dic.Get)
}

func (ctl *controller) getThingModelTemplateApp() interfaces.ThingModelTemplateApp {
	return container.ThingModelTemplateAppFrom(ctl.dic.Get)
}

func (ctl *controller) getThingModelApp() interfaces.ThingModelCtlItf {
	return container.ThingModelAppNameFrom(ctl.dic.Get)
}

func (ctl *controller) getAlertRuleApp() interfaces.AlertRuleApp {
	return container.AlertRuleAppNameFrom(ctl.dic.Get)
}

func (ctl *controller) getUnitModelApp() interfaces.UnitApp {
	return container.UnitTemplateAppFrom(ctl.dic.Get)
}

func (ctl *controller) getHomePageApp() interfaces.HomePageItf {
	return container.HomePageAppNameFrom(ctl.dic.Get)
}

func (ctl *controller) getDocsApp() interfaces.DocsApp {
	return container.DocsTemplateAppFrom(ctl.dic.Get)
}

func (ctl *controller) getQuickNavigationApp() interfaces.QuickNavigation {
	return container.QuickNavigationAppTemplateAppFrom(ctl.dic.Get)
}

// Perm 生成系统管理接口的权限校验中间件
func (ctl *controller) Perm(perms ...string) gin.HandlerFunc {
	return middleware.PermRequired(ctl.dic, perms...)
}
