package mqttclient

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// InstanceClientID 生成当前进程唯一的 MQTT clientId。
//
// 背景：平台可能同时运行多个实例（本地 + 服务器、灰度 + 生产），它们连的是同一个
// 外部 broker（如参考系统 things）。MQTT 规定同一 clientId 只允许一条连接，后连的
// 会把先连的踢下线；如果多个实例都用固定的 "iotserver-core-edge" 之类 clientId，
// 就会互相踢、反复重连，日志刷 "Connect comms goroutine - error triggered EOF"。
//
// 后缀 = 主机名（截断）+ 进程级随机串：同一台机器上起多个实例也不会冲突。
func InstanceClientID(prefix string) string {
	return prefix + "-" + instanceSuffix
}

var instanceSuffix = func() string {
	host, _ := os.Hostname()
	host = strings.ToLower(host)
	var clean strings.Builder
	for _, r := range host {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			clean.WriteRune(r)
		}
	}
	name := clean.String()
	if len(name) > 16 {
		name = name[:16]
	}
	if name == "" {
		name = "node"
	}
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s-%s", name, hex.EncodeToString(b[:]))
}()
