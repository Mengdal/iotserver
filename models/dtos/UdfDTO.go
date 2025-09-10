package dtos

// Udf 定义 JavaScript UDF 的数据结构
type Udf struct {
	Id          string `json:"id"`
	Description string `json:"description"`
	Script      string `json:"script"`
	IsAgg       bool   `json:"isAgg"`
}

// ExService 定义外部服务的数据结构
type ExService struct {
	Name        string `json:"name"`
	NewName     string `json:"newName"`
	Address     string `json:"address"`
	Description string `json:"description"`
	Action      string `json:"action"`
}
