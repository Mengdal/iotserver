package route

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/config"
	"iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	pkgcontainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/tools/streamclient"
)

// userItfStub 仅用于路由注册测试，避免依赖真实数据库
type userItfStub struct {
	interfaces.UserItf
}

func (s userItfStub) PermissionsByUsername(username string) ([]string, error) {
	return []string{}, nil
}

func (s userItfStub) IsAdminUsername(username string) bool {
	return true
}

func TestRegisterGatewayRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lc := logger.NewClient("test", "INFO", "")
	dic := di.NewContainer(di.ServiceConstructorMap{
		pkgcontainer.LoggingClientInterfaceName: func(get di.Get) interface{} {
			return lc
		},
		container.ConfigurationName: func(get di.Get) interface{} {
			return &config.ConfigurationStruct{}
		},
		container.UserItfName: func(get di.Get) interface{} {
			return userItfStub{}
		},
		pkgcontainer.StreamClientName: func(get di.Get) interface{} {
			return streamclient.NewStreamClient(lc)
		},
	})
	// userItfStub 需要满足接口，此处仅注册路由，用真实实现替换
	engine := gin.New()
	RegisterGateway(engine, dic)

	expected := map[string]bool{
		"GET /api/v1/sys-user":               false,
		"POST /api/v1/sys-user":              false,
		"PUT /api/v1/sys-user":               false,
		"DELETE /api/v1/sys-user":            false,
		"GET /api/v1/sys-user/:id":           false,
		"PUT /api/v1/sys-user/status/:id":    false,
		"PUT /api/v1/pwd/reset/:id":          false,
		"GET /api/v1/role":                   false,
		"GET /api/v1/role/all":               false,
		"GET /api/v1/role/:id":               false,
		"POST /api/v1/role":                  false,
		"PUT /api/v1/role":                   false,
		"DELETE /api/v1/role":                false,
		"GET /api/v1/menurole":               false,
		"POST /api/v1/menurole":              false,
		"GET /api/v1/menu":                   false,
		"GET /api/v1/menu/tree":              false,
		"GET /api/v1/menu/:id":               false,
		"POST /api/v1/menu":                  false,
		"PUT /api/v1/menu":                   false,
		"DELETE /api/v1/menu":                false,
		"GET /api/v1/dept":                   false,
		"GET /api/v1/dept-tree":              false,
		"GET /api/v1/dept/:id":               false,
		"POST /api/v1/dept":                  false,
		"PUT /api/v1/dept":                   false,
		"DELETE /api/v1/dept":                false,
		"GET /api/v1/sys-login-log":          false,
		"DELETE /api/v1/sys-login-log":       false,
		"DELETE /api/v1/sys-login-log/clear": false,
		"GET /api/v1/system-setup":           false,
		"GET /api/v1/system-setup/:id":       false,
		"POST /api/v1/system-setup":          false,
		"PUT /api/v1/system-setup":           false,
		"DELETE /api/v1/system-setup":        false,
		"POST /api/v1/system/backup":         false,
		"POST /api/v1/system-setup/save":     false,
		"GET /api/v1/user/profile":           false,
		"PUT /api/v1/user/profile":           false,
		"GET /api/v1/user/getinfo":           false,
	}

	for _, r := range engine.Routes() {
		key := r.Method + " " + r.Path
		if _, ok := expected[key]; ok {
			expected[key] = true
		}
	}
	for key, found := range expected {
		if !found {
			t.Errorf("路由未注册: %s", key)
		}
	}
}

var _ = context.Background
var _ = dtos.LoginRequest{}
