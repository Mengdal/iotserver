package resourceapp

import (
	"context"
	"fmt"
	"strings"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	pkgcontainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/valid"
	"iotserver/internal/tools/healthcheck"
)

// 敏感字段：读接口打码，更新时收到打码值则保留库中原值
var sensitiveOptionKeys = []string{"password", "saslPassword", "privateKey", "secret"}

const maskedValue = "******"

const (
	// maxOptionFieldLen 单个连接参数值上限
	maxOptionFieldLen = 1024
	// maxOptionFields 连接参数个数上限
	maxOptionFields = 32
)

// requiredOptions 各资源类型的必填参数
var requiredOptions = map[constants.DataResourceType][]string{
	constants.HttpResource:     {"url"},
	constants.MQTTResource:     {"server", "topic"},
	constants.KafkaResource:    {"brokers", "topic"},
	constants.InfluxDBResource: {"addr", "databasename"},
	constants.TDengineResource: {"host", "database", "table"},
}

type resourceApp struct {
	dic      *di.Container
	dbClient interfaces.DBClient
	lc       logger.LoggingClient
}

func New(dic *di.Container) *resourceApp {
	return &resourceApp{
		dic:      dic,
		lc:       pkgcontainer.LoggingClientFrom(dic.Get),
		dbClient: container.DBClientFrom(dic.Get),
	}
}

/** ***************************** 查询 ***************************** **/

// DataResourceSearch 按类型分页查询资源实例
func (app *resourceApp) DataResourceSearch(ctx context.Context, req dtos.DataResourceSearchRequest) ([]dtos.DataResourceResponse, uint32, error) {
	offset, limit := dtos.OffsetLimit(req.BaseSearchConditionQuery)
	list, total, err := app.dbClient.DataResourceSearch(offset, limit, req)
	if err != nil {
		return nil, 0, err
	}
	out := make([]dtos.DataResourceResponse, 0, len(list))
	for i := range list {
		out = append(out, toResponse(&list[i]))
	}
	return out, total, nil
}

// TypeResource 返回全部资源类型（前端页签数据源）
func (app *resourceApp) TypeResource(ctx context.Context) []constants.DataResourceType {
	return constants.DataResources
}

/** ***************************** 增删改 ***************************** **/

// DataResourceAdd 新增资源实例
func (app *resourceApp) DataResourceAdd(ctx context.Context, req dtos.DataResourceAddRequest) error {
	if err := validate(req.Name, req.Type, req.Option); err != nil {
		return err
	}
	option := models.DataResourceOption(normalizeOption(req.Option))
	if _, err := app.dbClient.AddDataResource(models.DataResource{
		Name:   req.Name,
		Type:   req.Type,
		Option: option,
	}); err != nil {
		return err
	}
	return nil
}

// DataResourceUpdate 修改资源实例
func (app *resourceApp) DataResourceUpdate(ctx context.Context, req dtos.DataResourceUpdateRequest) error {
	if strings.TrimSpace(req.Id) == "" {
		return errort.NewCommonErr(errort.DefaultIdEmpty, fmt.Errorf("id is empty"))
	}
	if err := validate(req.Name, req.Type, req.Option); err != nil {
		return err
	}
	// 前端拿到的密码是打码值，原样提交时保留库中真实密码
	option, err := app.keepMaskedSecrets(req.Id, normalizeOption(req.Option))
	if err != nil {
		return err
	}
	return app.dbClient.UpdateDataResource(models.DataResource{
		Id:     req.Id,
		Name:   req.Name,
		Type:   req.Type,
		Option: option,
	})
}

// DataResourceDelete 删除资源实例
func (app *resourceApp) DataResourceDelete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return errort.NewCommonErr(errort.DefaultIdEmpty, fmt.Errorf("id is empty"))
	}
	return app.dbClient.DeleteDataResource(id)
}

/** ***************************** 验证 ***************************** **/

// keepMaskedSecrets 提交值里仍为打码值的敏感字段，用库中原值替换
func (app *resourceApp) keepMaskedSecrets(id string, option map[string]interface{}) (models.DataResourceOption, error) {
	needKeep := false
	for _, key := range sensitiveOptionKeys {
		if v, ok := option[key]; ok {
			if text, isStr := v.(string); isStr && text == maskedValue {
				needKeep = true
				break
			}
		}
	}
	if !needKeep {
		return models.DataResourceOption(option), nil
	}
	old, err := app.dbClient.DataResourceById(id)
	if err != nil {
		return nil, err
	}
	for _, key := range sensitiveOptionKeys {
		if v, ok := option[key]; ok {
			if text, isStr := v.(string); isStr && text == maskedValue {
				if oldValue, exists := old.Option[key]; exists {
					option[key] = oldValue
				} else {
					delete(option, key)
				}
			}
		}
	}
	return models.DataResourceOption(option), nil
}

// maskSecrets 读接口对敏感字段打码，避免密码回显给列表浏览者
func maskSecrets(option models.DataResourceOption) map[string]interface{} {
	if option == nil {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(option))
	for k, v := range option {
		out[k] = v
	}
	for _, key := range sensitiveOptionKeys {
		if v, ok := out[key]; ok {
			if text, isStr := v.(string); isStr && text != "" {
				out[key] = maskedValue
			}
		}
	}
	return out
}

// DataResourceVerify 对资源实例做连通性验证，并把结果写回 health
func (app *resourceApp) DataResourceVerify(ctx context.Context, id string) (dtos.DataResourceVerifyResponse, error) {
	resource, err := app.dbClient.DataResourceById(id)
	if err != nil {
		return dtos.DataResourceVerifyResponse{}, err
	}

	checkErr := healthcheck.Check(resource.Type, map[string]interface{}(resource.Option))
	health := checkErr == nil
	if err := app.dbClient.UpdateDataResourceHealth(id, health); err != nil {
		return dtos.DataResourceVerifyResponse{}, err
	}

	resp := dtos.DataResourceVerifyResponse{Health: health, Message: "验证成功"}
	if checkErr != nil {
		resp.Message = checkErr.Error()
		app.lc.Warnf("data resource %s(%s) verify failed: %v", resource.Name, resource.Type, checkErr)
	}
	return resp, nil
}

/** ***************************** 内部 ***************************** **/

// validate 名称、类型与按类型的必填校验
func validate(name, resourceType string, option map[string]interface{}) error {
	if !valid.Name(name) {
		return errort.NewCommonErr(errort.DefaultNameSpecialCharacters, fmt.Errorf("%s", valid.NameRuleMsg))
	}
	if !isValidType(resourceType) {
		return errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("不支持的资源类型: %s", resourceType))
	}
	if err := checkOptionSize(option); err != nil {
		return err
	}
	missing := make([]string, 0, 2)
	for _, key := range requiredOptions[constants.DataResourceType(resourceType)] {
		if v, ok := option[key]; !ok || v == nil || strings.TrimSpace(fmt.Sprintf("%v", v)) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("缺少必填参数: %s", strings.Join(missing, ", ")))
	}
	return nil
}

// checkOptionSize 限制连接参数的数量与单值长度，避免超大请求体写爆列
func checkOptionSize(option map[string]interface{}) error {
	if len(option) > maxOptionFields {
		return errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("连接参数项过多（最多 %d 项）", maxOptionFields))
	}
	for key, value := range option {
		if text, ok := value.(string); ok && len([]rune(text)) > maxOptionFieldLen {
			return errort.NewCommonErr(errort.DefaultReqParamsError,
				fmt.Errorf("连接参数 %s 过长（最多 %d 个字符）", key, maxOptionFieldLen))
		}
	}
	return nil
}

func isValidType(resourceType string) bool {
	for _, t := range constants.DataResources {
		if string(t) == resourceType {
			return true
		}
	}
	return false
}

// normalizeOption 保证入库的 option 不是 nil
func normalizeOption(option map[string]interface{}) map[string]interface{} {
	if option == nil {
		return map[string]interface{}{}
	}
	return option
}

func toResponse(r *models.DataResource) dtos.DataResourceResponse {
	return dtos.DataResourceResponse{
		Id:       r.Id,
		Name:     r.Name,
		Type:     r.Type,
		Option:   maskSecrets(r.Option),
		Health:   r.Health,
		Created:  r.Created,
		Modified: r.Modified,
	}
}
