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

package quicknavigationapp

import (
	"context"
	"encoding/json"
	"iotserver/internal/dtos"
	resourceContainer "iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/utils"
)

type quickNavigationApp struct {
	dic      *di.Container
	dbClient interfaces.DBClient
	lc       logger.LoggingClient
}

func (m quickNavigationApp) SyncQuickNavigation(ctx context.Context, versionName string) (int64, error) {
	filePath := versionName + "/quick_navigation.json"
	cosApp := resourceContainer.CosAppNameFrom(m.dic.Get)
	bs, err := cosApp.Get(filePath)
	if err != nil {
		m.lc.Errorf(err.Error())
		return 0, err
	}
	var cosQuickNavigationTemplateResponse []dtos.CosQuickNavigationTemplateResponse
	err = json.Unmarshal(bs, &cosQuickNavigationTemplateResponse)
	if err != nil {
		m.lc.Errorf(err.Error())
		return 0, err
	}

	baseQuery := dtos.BaseSearchConditionQuery{
		IsAll: true,
	}
	dbreq := dtos.QuickNavigationSearchQueryRequest{BaseSearchConditionQuery: baseQuery}
	quickNavigations, _, err := m.dbClient.WithCtx(ctx).QuickNavigationSearch(0, -1, dbreq)
	if err != nil {
		return 0, err
	}

	var cosQuickNavigationName []string
	upsertQuickNavigationTemplate := make([]models.QuickNavigation, 0)
	for _, cosQuickNavigationTemplate := range cosQuickNavigationTemplateResponse {
		cosQuickNavigationName = append(cosQuickNavigationName, cosQuickNavigationTemplate.Name)
		var find bool
		for _, localQuickNavigation := range quickNavigations {
			if cosQuickNavigationTemplate.Name == localQuickNavigation.Name {
				upsertQuickNavigationTemplate = append(upsertQuickNavigationTemplate, models.QuickNavigation{
					Id:       localQuickNavigation.Id,
					Name:     cosQuickNavigationTemplate.Name,
					Icon:     cosQuickNavigationTemplate.Icon,
					Sort:     cosQuickNavigationTemplate.Sort,
					JumpLink: cosQuickNavigationTemplate.JumpLink,
				})
				find = true
				break
			}
		}
		if !find {
			upsertQuickNavigationTemplate = append(upsertQuickNavigationTemplate, models.QuickNavigation{
				Id:       utils.RandomNum(),
				Name:     cosQuickNavigationTemplate.Name,
				Icon:     cosQuickNavigationTemplate.Icon,
				Sort:     cosQuickNavigationTemplate.Sort,
				JumpLink: cosQuickNavigationTemplate.JumpLink,
			})
		}
	}
	rows, err := m.dbClient.WithCtx(ctx).BatchUpsertQuickNavigationTemplate(upsertQuickNavigationTemplate)
	if err != nil {
		return 0, err
	}

	for _, navigation := range quickNavigations {
		if !utils.InStringSlice(navigation.Name, cosQuickNavigationName) {
			err = m.dbClient.WithCtx(ctx).DeleteQuickNavigation(navigation.Id)
			if err != nil {
				return 0, err
			}
		}
	}

	return rows, nil
}

func NewQuickNavigationApp(ctx context.Context, dic *di.Container) interfaces.QuickNavigation {
	lc := container.LoggingClientFrom(dic.Get)
	dbClient := resourceContainer.DBClientFrom(dic.Get)
	return &quickNavigationApp{
		dic:      dic,
		dbClient: dbClient,
		lc:       lc,
	}
}
