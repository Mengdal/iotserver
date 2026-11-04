package jwt

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
	"iotserver/internal/pkg/middleware"
)

const (
	JwtIssuer = "iotserver"
	tokenKey  = "x-token"
)

var (
	// RefreshKey OpenAPI refresh token 的签名密钥。
	// 必须由服务启动时的持久化密钥派生（见 DeriveRefreshKey），不能硬编码或留空。
	RefreshKey string
	OpenAPIKey string
	JwtSignKey string
)

func SetOpenAPIKey(key string) {
	OpenAPIKey = key
}

// SetRefreshKey 设置 refresh token 签名密钥。
func SetRefreshKey(key string) {
	RefreshKey = key
}

// DeriveRefreshKey 从安装级密钥派生 refresh token 签名密钥。
//
// 使用域分隔的 SHA-256，避免与 access token 的签名密钥共用原始材料；
// base 为空时返回空串，调用方应视为未初始化并拒绝签发/校验 refresh token。
func DeriveRefreshKey(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("iotserver-refresh-token:" + base))
	return hex.EncodeToString(sum[:])
}

func SetJwtSignKey(key string) {
	JwtSignKey = key
}

func GenerateJwtSignKey() string {
	return uuid.New().String()
}

// JWT 令牌认证中间件
func JWTAuth(CloseAuthToken bool) gin.HandlerFunc {
	if CloseAuthToken {
		return func(c *gin.Context) {}
	}
	return func(c *gin.Context) {
		token := c.Request.Header.Get(tokenKey)
		vars := c.Request.URL.Query()
		if v, ok := vars[tokenKey]; ok {
			token = v[0]
		}
		if token == "" {
			httphelper.RenderFailNoLog(c, TokenInvalid, c.Writer)
			c.Abort()
			return
		}
		j := NewJWT(JwtSignKey)
		claims, err := j.ParseToken(token)
		if err != nil {
			httphelper.RenderFailNoLog(c, TokenExpired, c.Writer)
			c.Abort()
			return
		}
		if time.Now().Unix() > claims.ExpiresAt {
			httphelper.RenderFailNoLog(c, TokenExpired, c.Writer)
			c.Abort()
			return
		}
		c.Set(constants.JwtParsedInfo, claims)
		c.Next()
	}
}

type JWT struct {
	SigningKey []byte
}

var (
	TokenExpired     = errort.NewCommonErr(errort.DefaultTokenPermission, fmt.Errorf("Token expired, please log in again"))
	TokenNotValidYet = errort.NewCommonErr(errort.DefaultTokenPermission, fmt.Errorf("Token expired, please log in again"))
	TokenMalformed   = errort.NewCommonErr(errort.DefaultTokenPermission, fmt.Errorf("Token expired, please log in again"))
	TokenInvalid     = errort.NewCommonErr(errort.DefaultTokenPermission, fmt.Errorf("Token expired, please log in again"))
)

func NewJWT(key string) *JWT {
	return &JWT{
		[]byte(key),
	}
}

// 创建一个token
func (j *JWT) CreateToken(claims middleware.CustomClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.SigningKey)
}

// 解析 token
func (j *JWT) ParseToken(tokenString string) (*middleware.CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &middleware.CustomClaims{}, func(token *jwt.Token) (i interface{}, e error) {
		return j.SigningKey, nil
	})
	if err != nil {
		if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				return nil, TokenMalformed
			} else if ve.Errors&jwt.ValidationErrorExpired != 0 {
				return nil, TokenExpired
			} else if ve.Errors&jwt.ValidationErrorNotValidYet != 0 {
				return nil, TokenNotValidYet
			} else {
				return nil, TokenInvalid
			}
		}
	}
	if token != nil {
		if claims, ok := token.Claims.(*middleware.CustomClaims); ok && token.Valid {
			return claims, nil
		}
		return nil, TokenInvalid
	} else {
		return nil, TokenInvalid
	}
}
