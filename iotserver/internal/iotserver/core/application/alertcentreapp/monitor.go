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

package alertcentreapp

import (
	"context"
	"iotserver/internal/dtos"
	resourceContainer "iotserver/internal/iotserver/core/container"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/tenant"
	"time"
)

func (p alertApp) monitor() {
	tickTime := time.Second * 5
	timeTickerChan := time.Tick(tickTime)
	for {
		select {
		case <-timeTickerChan:
			p.checkRuleStatus()
		}
	}
}

func (p alertApp) checkRuleStatus() {
	// 平台作用域：系统路径，不按租户过滤
	alerts, _, err := p.dbClient.WithCtx(tenant.Platform(context.Background())).AlertRuleSearch(0, -1, dtos.AlertRuleSearchQueryRequest{})
	if err != nil {
		p.lc.Errorf("get alerts err:", err)
	}
	ekuiperApp := resourceContainer.EkuiperAppFrom(p.dic.Get)
	for _, alert := range alerts {
		if len(alert.SubRule) == 0 {
			continue
		}
		resp, err := ekuiperApp.GetRuleStats(context.Background(), alert.Id)
		if err != nil {
			continue
		}
		status, ok := resp["status"]
		if ok {
			if status != string(alert.Status) {
				if status == string(constants.RuleStop) {
					p.dbClient.WithCtx(tenant.Platform(context.Background())).AlertRuleStop(alert.Id)
				} else if status == string(constants.RuleStart) {
					p.dbClient.WithCtx(tenant.Platform(context.Background())).AlertRuleStart(alert.Id)
				}
			}
		}
	}
}
