package driverserviceapp

import (
	//"gitlab.com/tedge/edgex/internal/models"
	//"gitlab.com/tedge/edgex/internal/pkg/constants"
	"iotserver/internal/models"
)

// 驱动运行的配置模版
func getDriverConfigTemplate(ds models.DeviceService) string {

	return getDefaultDriverConfig()
}

func getDefaultDriverConfig() string {
	return `[Logger]
FileName = "/mnt/logs/driver.log"
LogLevel = "INFO" # DEBUG INFO WARN ERROR

[Clients]
[Clients.Core]
# 平台 gRPC 地址。这里的 127.0.0.1 是**占位符**，平台启动驱动前会把它替换成
# 驱动容器真正能连到的地址（平台容器 IP，或本机模式下的 host.docker.internal），
# 见 buildDriverCfg() 里的 strings.Replace。不要改成服务名，那样替换不生效。
Address  = "127.0.0.1:57081"
UseTLS = false
CertFilePath = ""

[Service]
ID = ""
Name = ""
ProductList = []
GwId = ""
LocalKey = ""
Activated = false
[Service.Server]
Address = "0.0.0.0:49991"
UseTLS = false
CertFile = ""
KeyFile = ""

[CustomConfig]`
}
