package things

import (
	"fmt"
	"os"
	"strings"

	"iotserver/internal/pkg/utils"
	jwt2 "iotserver/internal/tools/jwt"
)

const encryptionKeyEnv = "IOTSERVER_THINGS_KEY"

// EncryptionKey 返回 things 密码加密密钥。
//
// 优先使用部署环境变量，未配置时回落到 JWT 签名密钥（它随实例生成并持久化在
// 管理员账号上，重启后不变）。管理员重置签名密钥后，需要重新录入 things 密码。
func EncryptionKey() string {
	if key := strings.TrimSpace(os.Getenv(encryptionKeyEnv)); key != "" {
		return key
	}
	return jwt2.JwtSignKey
}

// EncryptPassword 加密 things 服务账号密码。空密码原样返回。
func EncryptPassword(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	encrypted, err := utils.EncryptAuthPassword(raw, EncryptionKey())
	if err != nil {
		return "", fmt.Errorf("encrypt things password: %w", err)
	}
	return encrypted, nil
}

// DecryptPassword 解密 things 服务账号密码。空密码原样返回。
func DecryptPassword(encrypted string) (string, error) {
	if encrypted == "" {
		return "", nil
	}
	raw, err := utils.DecryptAuthPassword(encrypted, EncryptionKey())
	if err != nil {
		return "", fmt.Errorf("decrypt things password: %w", err)
	}
	return raw, nil
}

// IsMasked 判断前端回传的是否为掩码值。
func IsMasked(value string) bool {
	return strings.TrimSpace(value) == MaskedValue
}
