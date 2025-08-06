package utils

import (
	"crypto/rand"
	"encoding/json"
	"github.com/bwmarrin/snowflake"
	"github.com/golang-jwt/jwt/v5"
	"log"
	"math/big"
	"sync"
	"time"
)

var jwtSecret = []byte("LMGateway")

// GenerateToken 生成 JWT Token
func GenerateToken(userId int64) (string, error) {
	nowTime := time.Now()
	expireTime := nowTime.Add(24 * time.Hour * 7)

	claims := jwt.MapClaims{
		"user_id": userId,                         // 自定义字段：用户ID
		"iss":     "LMGateway",                    // 标准字段：签发者 issuer
		"iat":     jwt.NewNumericDate(nowTime),    // 标准字段：签发时间 issued at
		"exp":     jwt.NewNumericDate(expireTime), // 标准字段：过期时间 expiration
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ParseToken 解析并验证 Token
func ParseToken(tokenString string) (jwt.MapClaims, bool) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return nil, false
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		return claims, true
	}

	return nil, false
}

// ParseJson 解析 JSON 字符串
func ParseJson(s string) map[string]interface{} {
	if s == "" {
		return map[string]interface{}{}
	}

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return map[string]interface{}{}
	}
	return m
}

var (
	node *snowflake.Node
	once sync.Once
)

// GenerateID 返回唯一的 int64 类型 ID（已自动初始化）
func GenerateID() int64 {
	once.Do(func() {
		var err error
		node, err = snowflake.NewNode(1)
		if err != nil {
			log.Fatalf("初始化 Snowflake 节点失败: %v", err)
		}
	})
	return node.Generate().Int64() % 1e12 // 保留后 12 位数字
}

func GenerateDeviceSecret(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	// 使用crypto/rand确保安全性
	for i := range b {
		// 生成0-51的随机数
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[idx.Uint64()]
	}
	return string(b)
}
