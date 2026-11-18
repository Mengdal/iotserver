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

package initialize

import (
	"context"
	"time"

	"iotserver/internal/iotserver/core/config"
	pkgContainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/tools/ekuiperclient"
)

func initApp(ctx context.Context, configuration *config.ConfigurationStruct, dic *di.Container) bool {
	lc := pkgContainer.LoggingClientFrom(dic.Get)
	go initEkuiperStreams(ctx, lc, configuration)
	return true
}

func initEkuiperStreams(ctx context.Context, lc logger.LoggingClient, configuration *config.ConfigurationStruct) {
	// eKuiper 可能比平台晚就绪（冷启动、首次拉镜像、崩溃重启）。
	// 固定只试一次会让 mqtt_stream 永久缺失，后续创建告警规则全部失败；
	// 这里按退避重试，服务运行期间 eKuiper 恢复后会自动补上。
	const (
		initialRetry = 10 * time.Second
		maxRetry     = time.Minute
	)
	baseURL := configuration.Clients["Ekuiper"].Address()
	delay := initialRetry
	for {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		created, err := ekuiperclient.EnsureStream(ctx, baseURL, lc, ekuiperclient.MQTTStreamName, ekuiperclient.MQTTStreamSQL)
		if err == nil {
			if created {
				lc.Infof("init ekuiper stream success")
			} else {
				lc.Infof("ekuiper stream %s already exists, skip", ekuiperclient.MQTTStreamName)
			}
			return
		}
		lc.Warnf("init ekuiper stream failed, retry in %s: %v", delay, err)
		if delay < maxRetry {
			delay *= 2
			if delay > maxRetry {
				delay = maxRetry
			}
		}
	}
}
