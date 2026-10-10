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
package initialize

import (
	"context"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/application"
	"iotserver/internal/iotserver/core/application/alertcentreapp"
	"iotserver/internal/iotserver/core/application/categorytemplate"
	"iotserver/internal/iotserver/core/application/deviceapp"
	"iotserver/internal/iotserver/core/application/dmi"
	"iotserver/internal/iotserver/core/application/docapp"
	"iotserver/internal/iotserver/core/application/driverapp"
	"iotserver/internal/iotserver/core/application/driverserviceapp"
	"iotserver/internal/iotserver/core/application/homepageapp"
	"iotserver/internal/iotserver/core/application/languagesdkapp"
	"iotserver/internal/iotserver/core/application/messageapp"
	"iotserver/internal/iotserver/core/application/messagestore"
	"iotserver/internal/iotserver/core/application/monitor"
	"iotserver/internal/iotserver/core/application/persistence"
	"iotserver/internal/iotserver/core/application/productapp"
	"iotserver/internal/iotserver/core/application/quicknavigationapp"
	"iotserver/internal/iotserver/core/application/resourceapp"
	"iotserver/internal/iotserver/core/application/ruleengineapp"
	"iotserver/internal/iotserver/core/application/sceneapp"
	"iotserver/internal/iotserver/core/application/thingmodelapp"
	"iotserver/internal/iotserver/core/application/thingmodeltemplate"
	"iotserver/internal/iotserver/core/application/unittemplate"
	"iotserver/internal/iotserver/core/application/userapp"
	"iotserver/internal/iotserver/core/config"
	"iotserver/internal/iotserver/core/container"
	"iotserver/internal/iotserver/core/controller/rpcserver/driverserver"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/iotserver/core/route"
	"iotserver/internal/pkg/constants"
	pkgContainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/cos"
	"iotserver/internal/pkg/crontab"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/handlers"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/startup"
	"iotserver/internal/tools/ekuiperclient"
	"iotserver/internal/tools/hpcloudclient"
	"iotserver/internal/tools/notify/sms"
	"iotserver/internal/tools/streamclient"
	"strings"
	"sync"
)

// Bootstrap contains references to dependencies required by the BootstrapHandler.
type Bootstrap struct {
	router *gin.Engine
}

// NewBootstrap is a factory method that returns an initialized Bootstrap receiver struct.
func NewBootstrap(router *gin.Engine) *Bootstrap {
	return &Bootstrap{
		router: router,
	}
}

func (b *Bootstrap) BootstrapHandler(ctx context.Context, wg *sync.WaitGroup, _ startup.Timer, dic *di.Container) bool {

	configuration := container.ConfigurationFrom(dic.Get)
	lc := pkgContainer.LoggingClientFrom(dic.Get)

	if !b.initClient(ctx, wg, dic, configuration, lc) {
		return false
	}

	if !initApp(ctx, configuration, dic) {
		return false
	}

	// rpc 服务
	if ok := initRPCServer(ctx, wg, dic); !ok {
		return false
	}
	lc.Infof("init rpc server")

	// http 路由
	route.LoadRestRoutes(b.router, dic)

	// 业务逻辑
	application.InitSchedule(dic, lc)

	wg.Add(1)
	go func() {
		defer wg.Done()

		<-ctx.Done()
		crontab.Stop()
	}()

	return true
}

func (b *Bootstrap) initClient(ctx context.Context, wg *sync.WaitGroup, dic *di.Container, configuration *config.ConfigurationStruct, lc logger.LoggingClient) bool {

	appMode, err := dmi.New(dic, ctx, wg, dtos.DriverConfigManage{
		DockerManageConfig: dtos.DockerManageConfig{
			ContainerConfigPath: configuration.DockerManage.ContainerConfigPath,
			DockerApiVersion:    configuration.DockerManage.DockerApiVersion,
			DockerRunMode:       constants.NetworkModeHost,
			DockerSelfName:      constants.CoreServiceName,
			Privileged:          configuration.DockerManage.Privileged,
		},
	})
	if err != nil {
		lc.Error("create driver model interface error %v", err)
		return false
	}

	dic.Update(di.ServiceConstructorMap{
		interfaces.DriverModelInterfaceName: func(get di.Get) interface{} {
			return appMode
		},
	})
	homePageApp := homepageapp.NewHomePageApp(ctx, dic)
	dic.Update(di.ServiceConstructorMap{
		container.HomePageAppName: func(get di.Get) interface{} {
			return homePageApp
		},
	})

	languageApp := languagesdkapp.NewLanguageSDKApp(ctx, dic)
	dic.Update(di.ServiceConstructorMap{
		container.LanguageSDKAppName: func(get di.Get) interface{} {
			return languageApp
		},
	})

	monitorApp := monitor.NewMonitor(ctx, dic)
	dic.Update(di.ServiceConstructorMap{
		container.MonitorAppName: func(get di.Get) interface{} {
			return monitorApp
		},
	})

	streamClient := streamclient.NewStreamClient(lc)
	dic.Update(di.ServiceConstructorMap{
		pkgContainer.StreamClientName: func(get di.Get) interface{} {
			return streamClient
		},
	})

	driverApp := driverapp.NewDriverApp(ctx, dic)
	dic.Update(di.ServiceConstructorMap{
		container.DriverAppName: func(get di.Get) interface{} {
			return driverApp
		},
	})

	driverServiceApp := driverserviceapp.NewDriverServiceApp(ctx, dic)
	dic.Update(di.ServiceConstructorMap{
		container.DriverServiceAppName: func(get di.Get) interface{} {
			return driverServiceApp
		},
	})

	productApp := productapp.NewProductApp(ctx, dic)
	dic.Update(di.ServiceConstructorMap{
		container.ProductAppName: func(get di.Get) interface{} {
			return productApp
		},
	})

	thingModelApp := thingmodelapp.NewThingModelApp(ctx, dic)
	dic.Update(di.ServiceConstructorMap{
		container.ThingModelAppName: func(get di.Get) interface{} {
			return thingModelApp
		},
	})

	deviceApp := deviceapp.NewDeviceApp(ctx, dic)
	dic.Update(di.ServiceConstructorMap{
		container.DeviceItfName: func(get di.Get) interface{} {
			return deviceApp
		},
	})

	alertCentreApp := alertcentreapp.NewAlertCentreApp(ctx, dic)
	dic.Update(di.ServiceConstructorMap{
		container.AlertRuleAppName: func(get di.Get) interface{} {
			return alertCentreApp
		},
	})

	cosApp := cos.NewCos("", "", "")
	dic.Update(di.ServiceConstructorMap{
		container.CosAppName: func(get di.Get) interface{} {
			return cosApp
		},
	})

	categoryTemplateApp := categorytemplate.NewCategoryTemplateApp(ctx, dic)
	dic.Update(di.ServiceConstructorMap{
		container.CategoryTemplateAppName: func(get di.Get) interface{} {
			return categoryTemplateApp
		},
	})

	unitTemplateApp := unittemplate.NewUnitTemplateApp(ctx, dic)
	dic.Update(di.ServiceConstructorMap{
		container.UnitTemplateAppName: func(get di.Get) interface{} {
			return unitTemplateApp
		},
	})

	docsApp := docapp.NewDocsApp(ctx, dic)
	dic.Update(di.ServiceConstructorMap{
		container.DocsAppName: func(get di.Get) interface{} {
			return docsApp
		},
	})

	quickNavigationApp := quicknavigationapp.NewQuickNavigationApp(ctx, dic)
	dic.Update(di.ServiceConstructorMap{
		container.QuickNavigationAppName: func(get di.Get) interface{} {
			return quickNavigationApp
		},
	})

	thingModelTemplateApp := thingmodeltemplate.NewThingModelTemplateApp(ctx, dic)
	dic.Update(di.ServiceConstructorMap{
		container.ThingModelTemplateAppName: func(get di.Get) interface{} {
			return thingModelTemplateApp
		},
	})

	hpcloudServiceApp := hpcloudclient.NewHpcloud(lc)
	dic.Update(di.ServiceConstructorMap{
		container.HpcServiceAppName: func(get di.Get) interface{} {
			return hpcloudServiceApp
		},
	})

	smsServiceApp := sms.NewSmsClient(lc, "",
		"", "")
	dic.Update(di.ServiceConstructorMap{
		container.SmsServiceAppName: func(get di.Get) interface{} {
			return smsServiceApp
		},
	})

	limitMethodApp := application.NewLimitMethodConf(*configuration)
	dic.Update(di.ServiceConstructorMap{
		pkgContainer.LimitMethodConfName: func(get di.Get) interface{} {
			return limitMethodApp
		},
	})

	ekuiperApp := ekuiperclient.New(configuration.Clients["Ekuiper"].Address(), lc)
	dic.Update(di.ServiceConstructorMap{
		container.EkuiperAppName: func(get di.Get) interface{} {
			return ekuiperApp
		},
	})

	persistItf := persistence.NewPersistApp(dic)
	dic.Update(di.ServiceConstructorMap{
		container.PersistItfName: func(get di.Get) interface{} {
			return persistItf
		},
	})

	userItf := userapp.New(dic)
	// 系统管理相关表：自动建表 + 初始化数据
	if err := userItf.MigrateSystemData(); err != nil {
		lc.Errorf("init system manage tables error %v", err)
		return false
	}
	// 加载 JWT 签名密钥
	userItf.InitJwtKey()
	dic.Update(di.ServiceConstructorMap{
		container.UserItfName: func(get di.Get) interface{} {
			return userItf
		},
	})

	// 高级能力 - 资源管理
	resourceItf := resourceapp.New(dic)
	dic.Update(di.ServiceConstructorMap{
		container.ResourceItfName: func(get di.Get) interface{} {
			return resourceItf
		},
	})

	// eKuiper 回调鉴权密钥（未配置时不校验）
	dtos.SetCallbackToken(configuration.Service.CallbackToken)
	// 回调地址格式校验：配错时立即暴露，而不是等到下发规则才失败
	if callbackUrl := configuration.Service.CallbackUrl; strings.TrimSpace(callbackUrl) != "" {
		if !strings.HasPrefix(callbackUrl, "http://") && !strings.HasPrefix(callbackUrl, "https://") {
			lc.Errorf("invalid Service.CallbackUrl %q: must start with http:// or https://", callbackUrl)
			return false
		}
	}

	// 高级能力 - 规则引擎（闭包外构造：构造函数内部会访问 dic）
	ruleEngineItf := ruleengineapp.New(dic)
	dic.Update(di.ServiceConstructorMap{
		container.RuleEngineItfName: func(get di.Get) interface{} {
			return ruleEngineItf
		},
	})

	// 高级能力 - 场景联动（闭包外构造：构造函数内部会访问 dic）
	sceneItf := sceneapp.New(dic)
	dic.Update(di.ServiceConstructorMap{
		container.SceneItfName: func(get di.Get) interface{} {
			return sceneItf
		},
	})

	messageItf := messageapp.NewMessageApp(dic, configuration.Clients["Ekuiper"].Address())
	dic.Update(di.ServiceConstructorMap{
		container.MessageItfName: func(get di.Get) interface{} {
			return messageItf
		},
	})

	messageStoreItf := messagestore.NewMessageStore(dic)
	dic.Update(di.ServiceConstructorMap{
		container.MessageStoreItfName: func(get di.Get) interface{} {
			return messageStoreItf
		},
	})
	return true
}

func initRPCServer(ctx context.Context, wg *sync.WaitGroup, dic *di.Container) bool {
	lc := pkgContainer.LoggingClientFrom(dic.Get)
	_, err := handlers.NewRPCServer(ctx, wg, dic, func(serve *grpc.Server) {
		driverserver.RegisterRPCService(lc, dic, serve)
		reflection.Register(serve)
	})
	if err != nil {
		lc.Errorf("initRPCServer err:%v", err)
		return false
	}
	return true
}
