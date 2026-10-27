package utils

import (
	"fmt"
	"hash/crc32"
	rand2 "math/rand"

	"github.com/google/uuid"
)

func GenUUID() string {
	uuidHash := crc32.ChecksumIEEE([]byte(uuid.New().String()))
	return fmt.Sprint(uuidHash)
}

// RandomNum 返回 8 位数字 ID。
//
// 使用全局随机源：之前每次调用都用 time.Now().UnixNano() 重新播种，
// 同一纳秒内的并发调用会得到完全相同的序列，高频写入（如网关日志）
// 会因此撞主键。全局源由 math/rand 保证并发安全且只初始化一次。
func RandomNum() string {
	return fmt.Sprintf("%08v", rand2.Int31n(100000000))
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func GenerateDeviceSecret(n int) string {
	b := make([]byte, n)
	// A rand.Int63() generates 63 random bits, enough for letterIdxMax letters!
	for i, cache, remain := n-1, rand2.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = rand2.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}
	return string(b)
}

func GenerateSaltKey() string {
	return uuid.New().String()
}
