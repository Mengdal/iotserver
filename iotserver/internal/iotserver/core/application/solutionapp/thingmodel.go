// 方案物模型（TSL）条目维护：属性 / 事件 / 服务 的新增、编辑、删除。
//
// 核心语义：**内置方案只读**。tenant_id=” 且 built_in=1 的记录绝不允许原地修改；
// 租户首次修改某内置方案时，克隆一份该租户自己的副本（同一 key、tenant_id=当前租户、
// built_in=0、id 为新随机值），改动只写进副本，之后再改都改副本。
package solutionapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/utils"
)

// solutionThingModelArrayKeys 能力类型 → 物模型 JSON 数组字段
var solutionThingModelArrayKeys = map[string]string{
	dtos.SolutionItemTypeProperty: "properties",
	dtos.SolutionItemTypeEvent:    "events",
	dtos.SolutionItemTypeAction:   "services",
}

// paramsErr 参数错误（10003）
func paramsErr(format string, args ...interface{}) error {
	return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf(format, args...))
}

// thingModelArrayKey 校验并换算 type 对应的数组字段
func thingModelArrayKey(itemType string) (string, error) {
	arrayKey, ok := solutionThingModelArrayKeys[strings.ToLower(strings.TrimSpace(itemType))]
	if !ok {
		return "", paramsErr("物模型类型 type 非法，仅支持 property/event/action")
	}
	return arrayKey, nil
}

// thingModelItemIdentifier 取条目标识符（去空白）
func thingModelItemIdentifier(item map[string]interface{}) string {
	if item == nil {
		return ""
	}
	identifier, _ := item["identifier"].(string)
	return strings.TrimSpace(identifier)
}

// indexOfThingModelItem 按标识符定位条目，未找到返回 -1
func indexOfThingModelItem(list []map[string]interface{}, identifier string) int {
	for i, item := range list {
		if thingModelItemIdentifier(item) == identifier {
			return i
		}
	}
	return -1
}

// decodeThingModel 物模型 JSON → 顶层字段表（保留 properties/events/services 之外的字段）
func decodeThingModel(raw string) (map[string]json.RawMessage, error) {
	doc := map[string]json.RawMessage{}
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return doc, nil
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// decodeThingModelArray 取某个能力数组（缺失 / null 视为空数组）
func decodeThingModelArray(doc map[string]json.RawMessage, arrayKey string) ([]map[string]interface{}, error) {
	list := []map[string]interface{}{}
	raw, ok := doc[arrayKey]
	if !ok {
		return list, nil
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return list, nil
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	if list == nil {
		list = []map[string]interface{}{}
	}
	return list, nil
}

// storeThingModelArray 把改动后的能力数组写回物模型文档
func storeThingModelArray(doc map[string]json.RawMessage, arrayKey string, list []map[string]interface{}) error {
	raw, err := json.Marshal(list)
	if err != nil {
		return err
	}
	doc[arrayKey] = raw
	return nil
}

// newSolutionCloneId 生成租户副本主键：sol-<key>-<随机>（与内置的 mt-<key> 区分）
func newSolutionCloneId(key string) string {
	raw := strings.ReplaceAll(utils.GenerateSaltKey(), "-", "")
	if len(raw) > 12 {
		raw = raw[:12]
	}
	return fmt.Sprintf("sol-%s-%s", key, raw)
}

// cloneSolutionForTenant 以内置方案为蓝本克隆出当前租户的副本
func cloneSolutionForTenant(src models.Solution, tenantId string) models.Solution {
	clone := src
	clone.Id = newSolutionCloneId(src.Key)
	clone.TenantId = tenantId
	clone.BuiltIn = 0
	clone.Timestamps = models.Timestamps{}
	return clone
}

// mutateSolutionThingModel 在「当前租户可写的那一行」上执行物模型改动。
//
// mutate 先在内存里完成并校验（重复标识符等错误不会留下半成品副本）；
// 若可写行仍是内置行，则一次性插入带改动的租户副本，内置行保持原样。
func (app *machineTemplateApp) mutateSolutionThingModel(ctx context.Context, key string,
	mutate func(doc map[string]json.RawMessage) error) (dtos.SolutionResponse, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return dtos.SolutionResponse{}, paramsErr("模板 key 不能为空")
	}
	tenantId := tenantIDFromCtx(ctx)
	db := app.dbClient.WithCtx(ctx)
	tpl, err := db.SolutionByKey(tenantId, key)
	if err != nil {
		return dtos.SolutionResponse{}, errort.NewCommonErr(errort.DefaultResourcesNotFound,
			errort.NewMessageErr("方案 {{.p1}} 不存在", map[string]interface{}{"p1": key}))
	}

	doc, err := decodeThingModel(tpl.ThingModelJson)
	if err != nil {
		return dtos.SolutionResponse{}, errort.NewCommonErr(errort.DefaultJsonParseError,
			errort.NewMessageErr("方案 {{.p1}} 的物模型 JSON 解析失败", map[string]interface{}{"p1": key}))
	}
	if err = mutate(doc); err != nil {
		return dtos.SolutionResponse{}, err
	}
	thingModelJson, err := json.Marshal(doc)
	if err != nil {
		return dtos.SolutionResponse{}, errort.NewCommonErr(errort.DefaultJsonParseError,
			errort.NewMessageErr("方案 {{.p1}} 的物模型 JSON 序列化失败", map[string]interface{}{"p1": key}))
	}

	// 内置行：克隆副本（带改动一次性落库），绝不原地修改。
	// 租户上下文 → 租户副本；平台管理员（无租户上下文）→ 平台级副本（tenant_id=''、built_in=0），
	// 与内置行同 key，读取时以「非内置优先」覆盖内置行，因此内置行始终保持只读。
	if tpl.TenantId == "" {
		clone := cloneSolutionForTenant(tpl, tenantId)
		clone.ThingModelJson = string(thingModelJson)
		if err = db.AddSolution(clone); err != nil {
			return dtos.SolutionResponse{}, errort.NewCommonErr(errort.DefaultSystemError,
				errort.NewMessageErr("方案 {{.p1}} 的租户副本创建失败", map[string]interface{}{"p1": key}))
		}
		return machineTemplateResponseFromModel(clone), nil
	}

	// 租户副本：直接更新
	if err = db.UpdateSolutionThingModel(tpl.Id, string(thingModelJson)); err != nil {
		return dtos.SolutionResponse{}, errort.NewCommonErr(errort.DefaultSystemError,
			errort.NewMessageErr("方案 {{.p1}} 的物模型保存失败", map[string]interface{}{"p1": key}))
	}
	tpl.ThingModelJson = string(thingModelJson)
	return machineTemplateResponseFromModel(tpl), nil
}

// SolutionThingModelItemAdd 新增物模型条目（属性/事件/服务）
func (app *machineTemplateApp) SolutionThingModelItemAdd(ctx context.Context, key string,
	req dtos.SolutionThingModelItemRequest) (dtos.SolutionResponse, error) {
	arrayKey, err := thingModelArrayKey(req.Type)
	if err != nil {
		return dtos.SolutionResponse{}, err
	}
	identifier := thingModelItemIdentifier(req.Item)
	if identifier == "" {
		return dtos.SolutionResponse{}, paramsErr("物模型条目的 identifier 不能为空")
	}
	item := req.Item
	return app.mutateSolutionThingModel(ctx, key, func(doc map[string]json.RawMessage) error {
		list, dErr := decodeThingModelArray(doc, arrayKey)
		if dErr != nil {
			return errort.NewCommonErr(errort.DefaultJsonParseError,
				errort.NewMessageErr("方案 {{.p1}} 的物模型 {{.p2}} 解析失败", map[string]interface{}{"p1": key, "p2": arrayKey}))
		}
		if indexOfThingModelItem(list, identifier) >= 0 {
			return errort.NewCommonErr(errort.ThingModelCodeExist,
				errort.NewMessageErr("物模型标识符 {{.p1}} 已存在，不能重复添加", map[string]interface{}{"p1": identifier}))
		}
		return storeThingModelArray(doc, arrayKey, append(list, item))
	})
}

// SolutionThingModelItemUpdate 按旧 identifier 替换物模型条目
func (app *machineTemplateApp) SolutionThingModelItemUpdate(ctx context.Context, key string,
	req dtos.SolutionThingModelItemRequest) (dtos.SolutionResponse, error) {
	arrayKey, err := thingModelArrayKey(req.Type)
	if err != nil {
		return dtos.SolutionResponse{}, err
	}
	oldIdentifier := strings.TrimSpace(req.Identifier)
	if oldIdentifier == "" {
		return dtos.SolutionResponse{}, paramsErr("被修改条目的 identifier 不能为空")
	}
	newIdentifier := thingModelItemIdentifier(req.Item)
	if newIdentifier == "" {
		return dtos.SolutionResponse{}, paramsErr("物模型条目的 identifier 不能为空")
	}
	item := req.Item
	return app.mutateSolutionThingModel(ctx, key, func(doc map[string]json.RawMessage) error {
		list, dErr := decodeThingModelArray(doc, arrayKey)
		if dErr != nil {
			return errort.NewCommonErr(errort.DefaultJsonParseError,
				errort.NewMessageErr("方案 {{.p1}} 的物模型 {{.p2}} 解析失败", map[string]interface{}{"p1": key, "p2": arrayKey}))
		}
		idx := indexOfThingModelItem(list, oldIdentifier)
		if idx < 0 {
			return errort.NewCommonErr(errort.ThingModelNotExist,
				errort.NewMessageErr("物模型条目 {{.p1}} 不存在", map[string]interface{}{"p1": oldIdentifier}))
		}
		if newIdentifier != oldIdentifier && indexOfThingModelItem(list, newIdentifier) >= 0 {
			return errort.NewCommonErr(errort.ThingModelCodeExist,
				errort.NewMessageErr("物模型标识符 {{.p1}} 已存在，不能重复", map[string]interface{}{"p1": newIdentifier}))
		}
		list[idx] = item
		return storeThingModelArray(doc, arrayKey, list)
	})
}

// SolutionThingModelItemDelete 按 identifier 删除物模型条目
func (app *machineTemplateApp) SolutionThingModelItemDelete(ctx context.Context, key string,
	req dtos.SolutionThingModelItemDeleteRequest) (dtos.SolutionResponse, error) {
	arrayKey, err := thingModelArrayKey(req.Type)
	if err != nil {
		return dtos.SolutionResponse{}, err
	}
	identifier := strings.TrimSpace(req.Identifier)
	if identifier == "" {
		return dtos.SolutionResponse{}, paramsErr("identifier 不能为空")
	}
	return app.mutateSolutionThingModel(ctx, key, func(doc map[string]json.RawMessage) error {
		list, dErr := decodeThingModelArray(doc, arrayKey)
		if dErr != nil {
			return errort.NewCommonErr(errort.DefaultJsonParseError,
				errort.NewMessageErr("方案 {{.p1}} 的物模型 {{.p2}} 解析失败", map[string]interface{}{"p1": key, "p2": arrayKey}))
		}
		idx := indexOfThingModelItem(list, identifier)
		if idx < 0 {
			return errort.NewCommonErr(errort.ThingModelNotExist,
				errort.NewMessageErr("物模型条目 {{.p1}} 不存在", map[string]interface{}{"p1": identifier}))
		}
		return storeThingModelArray(doc, arrayKey, append(list[:idx], list[idx+1:]...))
	})
}
