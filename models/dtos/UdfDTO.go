package dtos

// Udf 定义 JavaScript UDF 的数据结构
type Udf struct {
	Id          string `json:"id"`
	Description string `json:"description"`
	Script      string `json:"script"`
	IsAgg       bool   `json:"isAgg"`
}
