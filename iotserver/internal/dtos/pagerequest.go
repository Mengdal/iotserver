package dtos

const (
	PageDefault        = 1
	PageSizeDefault    = 10
	PageSizeMaxDefault = 1000
)

func CorrectionPageParam(query *BaseSearchConditionQuery) {
	if query.Page <= 0 {
		query.Page = PageDefault
	}

	if query.PageSize >= PageSizeMaxDefault {
		query.PageSize = PageSizeMaxDefault
	} else if query.PageSize <= 0 {
		query.PageSize = PageSizeDefault
	}
}

// OffsetLimit 由分页参数换算 offset/limit。
// 缺省值与 CorrectionPageParam 保持一致（page=1、pageSize=10），供应用层查询使用。
func OffsetLimit(query BaseSearchConditionQuery) (int, int) {
	page, pageSize := query.Page, query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	return (page - 1) * pageSize, pageSize
}
