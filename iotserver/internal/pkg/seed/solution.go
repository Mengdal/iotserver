package seed

import (
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"iotserver/internal/models"
)

// 内置工业方案（Smart Factory 用）。
//
// 每个模板文件同时提供三份数据：
//   - category_template / thing_model_template：让前端现有「一键建产品」链路直接可用（零改动）；
//   - solution：模板包（含告警规则预设、看板预设、OEE 指标映射），供一键应用。
//
//go:embed templates/*.json
var solutionFS embed.FS

// SolutionFile 模板文件的顶层结构
type SolutionFile struct {
	Key            string          `json:"key"`
	Name           string          `json:"name"`
	NameEn         string          `json:"nameEn"`
	Industry       string          `json:"industry"`
	Version        int             `json:"version"`
	Icon           string          `json:"icon"`
	Description    string          `json:"description"`
	DescriptionEn  string          `json:"descriptionEn"`
	ThingModelJson json.RawMessage `json:"thingModelJson"`
	AlarmRules     []SolutionAlarm `json:"alarmRules"`
	Dashboard      json.RawMessage `json:"dashboard"`
	Metrics        json.RawMessage `json:"metrics"`
}

// SolutionAlarm 模板里的告警规则预设（一键应用时转成平台告警规则）
type SolutionAlarm struct {
	Name         string `json:"name"`
	PropertyCode string `json:"propertyCode"`
	Operator     string `json:"operator"`
	Value        string `json:"value"`
	AlertLevel   int    `json:"alertLevel"`
	AlertType    int    `json:"alertType"`
	SilenceTime  int64  `json:"silenceTime"`
	Description  string `json:"description"`
}

// SolutionSet 一个模板对应的三份落库数据
type SolutionSet struct {
	Template      models.Solution
	CategoryTpl   models.CategoryTemplate
	ThingModelTpl models.ThingModelTemplate
}

// CategoryTemplateKey 内置模板生成的品类 key 前缀（前端选品类时用）
const CategoryTemplateKeyPrefix = "sol_"

// LoadSolutions 读取并解析全部内置模板文件（按 key 排序，保证 seed 幂等且顺序稳定）
func LoadSolutions() ([]SolutionSet, error) {
	entries, err := solutionFS.ReadDir("templates")
	if err != nil {
		return nil, fmt.Errorf("读取内置模板目录失败: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	sets := make([]SolutionSet, 0, len(names))
	for _, name := range names {
		raw, err := solutionFS.ReadFile(path.Join("templates", name))
		if err != nil {
			return nil, fmt.Errorf("读取模板 %s 失败: %w", name, err)
		}
		var f SolutionFile
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, fmt.Errorf("解析模板 %s 失败: %w", name, err)
		}
		if strings.TrimSpace(f.Key) == "" || strings.TrimSpace(f.Name) == "" {
			return nil, fmt.Errorf("模板 %s 缺少 key 或 name", name)
		}
		// 物模型必须能被现有解析器消费（properties/events/services）
		var probe struct {
			Properties []json.RawMessage `json:"properties"`
			Events     []json.RawMessage `json:"events"`
			Services   []json.RawMessage `json:"services"`
		}
		if err := json.Unmarshal(f.ThingModelJson, &probe); err != nil {
			return nil, fmt.Errorf("模板 %s 的 thingModelJson 非法: %w", name, err)
		}

		alarmJSON, _ := json.Marshal(f.AlarmRules)
		metrics := string(f.Metrics)
		if strings.TrimSpace(metrics) == "" || metrics == "null" {
			metrics = "{}"
		}
		categoryKey := CategoryTemplateKeyPrefix + f.Key

		sets = append(sets, SolutionSet{
			Template: models.Solution{
				Id:             "mt-" + f.Key,
				Key:            f.Key,
				TenantId:       "", // 平台内置，所有租户可见
				Name:           f.Name,
				NameEn:         f.NameEn,
				Industry:       f.Industry,
				Version:        f.Version,
				BuiltIn:        1,
				Icon:           f.Icon,
				Description:    f.Description,
				DescriptionEn:  f.DescriptionEn,
				ThingModelJson: string(f.ThingModelJson),
				AlarmRulesJson: string(alarmJSON),
				DashboardJson:  string(f.Dashboard),
				MetricsJson:    metrics,
			},
			CategoryTpl: models.CategoryTemplate{
				Id:             categoryKey,
				CategoryKey:    categoryKey,
				CategoryName:   f.Name,
				CategoryNameEn: f.NameEn,
				Scene:          f.Industry,
			},
			ThingModelTpl: models.ThingModelTemplate{
				Id:             "tmt-" + f.Key,
				CategoryKey:    categoryKey,
				CategoryName:   f.Name,
				CategoryNameEn: f.NameEn,
				ThingModelJSON: string(f.ThingModelJson),
			},
		})
	}
	return sets, nil
}

// DefaultSolutions 返回内置模板的三份落库数据（seed 用）
func DefaultSolutions() ([]models.Solution, []models.CategoryTemplate, []models.ThingModelTemplate, error) {
	sets, err := LoadSolutions()
	if err != nil {
		return nil, nil, nil, err
	}
	templates := make([]models.Solution, 0, len(sets))
	categories := make([]models.CategoryTemplate, 0, len(sets))
	thingModels := make([]models.ThingModelTemplate, 0, len(sets))
	for _, s := range sets {
		templates = append(templates, s.Template)
		categories = append(categories, s.CategoryTpl)
		thingModels = append(thingModels, s.ThingModelTpl)
	}
	return templates, categories, thingModels, nil
}
