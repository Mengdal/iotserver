package models

import (
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/client/orm"
)

type Unit struct {
	Id       int64  `orm:"auto;pk" json:"id"`
	Symbol   string `orm:"type(text)" json:"symbol"`
	UnitName string `orm:"type(text)" json:"unit_name"`
}

func init() {
	// 注册模型
	orm.RegisterModel(new(Unit))
}

// 批量存入数据库
func SaveUnits(jsonData string) error {
	// 解析 JSON
	var units []Unit
	if err := json.Unmarshal([]byte(jsonData), &units); err != nil {
		return fmt.Errorf("解析 JSON 失败: %v", err)
	}

	o := orm.NewOrm()
	for _, u := range units {
		// 插入或更新
		_, err := o.InsertOrUpdate(&u, "Id") // 如果 Id 存在则更新
		if err != nil {
			return fmt.Errorf("保存失败 id=%d: %v", u.Id, err)
		}
	}
	return nil
}
