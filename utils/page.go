package utils

import "github.com/beego/beego/v2/client/orm"

func Paginate(qs orm.QuerySeter, page, size int, list interface{}) (*PageResult, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}

	var total int64
	var err error

	// 获取总数
	total, err = qs.Count()
	if err != nil {
		return nil, err
	}

	// 查询数据
	_, err = qs.Limit(size, (page-1)*size).All(list)
	if err != nil {
		return nil, err
	}

	// 计算总页数
	totalPage := int((total + int64(size) - 1) / int64(size))

	// 判断是否为首页或末页
	isFirstPage := page == 1
	isLastPage := page >= totalPage

	return &PageResult{
		List:       list,
		PageNumber: page,
		PageSize:   size,
		TotalPage:  totalPage,
		TotalRow:   total,
		FirstPage:  isFirstPage,
		LastPage:   isLastPage,
	}, nil
}

type PageResult struct {
	List       interface{} `json:"list"`
	PageNumber int         `json:"pageNumber"`
	PageSize   int         `json:"pageSize"`
	TotalPage  int         `json:"totalPage"`
	TotalRow   int64       `json:"totalRow"`
	FirstPage  bool        `json:"firstPage"`
	LastPage   bool        `json:"lastPage"`
}
