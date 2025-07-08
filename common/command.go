package common

import "flag"

// install/uninstall
var service string

// 获取service处理方式
func Service() string {
	return service
}

func SetCommandParam() {
	flag.StringVar(&service, "service", "", "服务操作")
	// 解析命令行参数写入注册的flag里
	flag.Parse()
}
