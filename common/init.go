package common

import (
	"github.com/beego/beego/v2/client/orm"
	_ "github.com/mattn/go-sqlite3" // SQLite 驱动
)

func InitDB() {
	// 注册数据库
	orm.RegisterDriver("sqlite", orm.DRSqlite)
	orm.RegisterDataBase("default", "sqlite3", "./database/server.db?cache=shared&_fk=1")

	// 注册模型由 models 包中的 init() 自动完成
	// 自动建表
	err := orm.RunSyncdb("default", false, true)
	// 调试模式
	orm.Debug = true
	if err != nil {
		panic("数据库初始化失败: " + err.Error())
	}
}
