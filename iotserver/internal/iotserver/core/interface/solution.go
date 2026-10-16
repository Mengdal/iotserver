package interfaces

import (
	"context"

	"iotserver/internal/dtos"
)

// SolutionItf 行业方案：内置模板查询、一键应用与物模型条目维护
type SolutionItf interface {
	// SolutionSearch 查询当前租户可见的模板（内置 + 本租户自定义，同 key 只返回一条）
	SolutionSearch(ctx context.Context, req dtos.SolutionSearchRequest) ([]dtos.SolutionResponse, error)
	// SolutionByKey 模板详情
	SolutionByKey(ctx context.Context, key string) (dtos.SolutionResponse, error)
	// SolutionApply 一键应用：生成产品（含物模型）+ 告警规则
	SolutionApply(ctx context.Context, key string, req dtos.SolutionApplyRequest) (dtos.SolutionApplyResponse, error)
	// SolutionThingModelItemAdd 新增物模型条目（property/event/action），返回更新后的方案
	SolutionThingModelItemAdd(ctx context.Context, key string, req dtos.SolutionThingModelItemRequest) (dtos.SolutionResponse, error)
	// SolutionThingModelItemUpdate 按旧 identifier 替换物模型条目，返回更新后的方案
	SolutionThingModelItemUpdate(ctx context.Context, key string, req dtos.SolutionThingModelItemRequest) (dtos.SolutionResponse, error)
	// SolutionThingModelItemDelete 按 identifier 删除物模型条目，返回更新后的方案
	SolutionThingModelItemDelete(ctx context.Context, key string, req dtos.SolutionThingModelItemDeleteRequest) (dtos.SolutionResponse, error)
}
