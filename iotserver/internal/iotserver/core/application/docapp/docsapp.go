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

package docapp

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

type docApp struct {
	dic      *di.Container
	dbClient interfaces.DBClient
	lc       logger.LoggingClient
}

func (m docApp) SyncDocs(ctx context.Context, versionName string) (int64, error) {
	filePath := versionName + "/doc.json"
	cosApp := resourceContainer.CosAppNameFrom(m.dic.Get)
	bs, err := cosApp.Get(filePath)
	if err != nil {
		m.lc.Errorf(err.Error())
		return 0, err
	}
	var cosDocTemplateResponse []dtos.CosDocTemplateResponse
	err = json.Unmarshal(bs, &cosDocTemplateResponse)
	if err != nil {
		m.lc.Errorf(err.Error())
		return 0, err
	}

	baseQuery := dtos.BaseSearchConditionQuery{
		IsAll: true,
	}
	dbreq := dtos.DocsSearchQueryRequest{BaseSearchConditionQuery: baseQuery}
	docs, _, err := m.dbClient.DocsSearch(0, -1, dbreq)
	if err != nil {
		return 0, err
	}

	upsertCosTemplate := make([]models.Doc, 0)
	for _, cosDocsTemplate := range cosDocTemplateResponse {
		var find bool
		for _, localDocsResponse := range docs {
			if cosDocsTemplate.Name == localDocsResponse.Name {
				upsertCosTemplate = append(upsertCosTemplate, models.Doc{
					Id:       localDocsResponse.Id,
					Name:     cosDocsTemplate.Name,
					Sort:     cosDocsTemplate.Sort,
					JumpLink: cosDocsTemplate.JumpLink,
				})
				find = true
				break
			}
		}
		if !find {
			upsertCosTemplate = append(upsertCosTemplate, models.Doc{
				Id:       utils.RandomNum(),
				Name:     cosDocsTemplate.Name,
				Sort:     cosDocsTemplate.Sort,
				JumpLink: cosDocsTemplate.JumpLink,
			})
		}
	}
	rows, err := m.dbClient.BatchUpsertDocsTemplate(upsertCosTemplate)
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func NewDocsApp(ctx context.Context, dic *di.Container) interfaces.DocsApp {
	lc := container.LoggingClientFrom(dic.Get)
	dbClient := resourceContainer.DBClientFrom(dic.Get)

	return &docApp{
		dic:      dic,
		dbClient: dbClient,
		lc:       lc,
	}
}
