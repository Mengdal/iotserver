package common

import (
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
	_ "github.com/mattn/go-sqlite3" // SQLite 驱动
	"net"
	"os"
	"strings"
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

var LocalHost, _ = beego.AppConfig.String("localhost")
var Port, _ = beego.AppConfig.String("httpport")
var CallBackUrl = "http://" + GetIPByHostname() + ":" + Port
var EkuiperServer, _ = beego.AppConfig.String("ekuiperServer")
var Ekuiper = NewEkuiperClient(EkuiperServer)

func GetIPByHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println("获取主机名失败:", err)
		return ""
	}
	ips, err := net.LookupHost(hostname)
	if err != nil {
		fmt.Println("主机名解析失败:", err)
		return ""
	}
	for _, ip := range ips {
		// 过滤回环地址和IPv6
		if strings.HasPrefix(ip, "127.") || strings.Contains(ip, ":") {
			continue
		}
		fmt.Println("主机名对应IP:", ip)
		return ip
	}
	fmt.Println("未找到可用的IPv4地址")
	return ""
}
