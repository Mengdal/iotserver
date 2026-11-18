package route

import (
	"context"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/webdav"

	"iotserver/internal/iotserver/core/config"
	"iotserver/internal/iotserver/core/container"
	pkgcontainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/tools/streamclient"
)

// 生产模式（Pprof.Enabled=false）下清空内存文件系统，释放 Swagger UI 资源。
func TestReleaseSwaggerFileSystem(t *testing.T) {
	ctx := context.Background()
	fs := webdav.NewMemFS()
	f, err := fs.OpenFile(ctx, "/index.html", os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("create test file: %v", err)
	}
	if _, err := f.Write([]byte("swagger")); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	_ = f.Close()

	releaseSwaggerFileSystem(fs)

	if _, err := fs.Stat(ctx, "/index.html"); err == nil {
		t.Fatal("swagger assets should be released")
	}
}

func TestSwaggerRouteFollowsPprof(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, enabled := range []bool{false, true} {
		lc := logger.NewClient("test", "INFO", "")
		dic := di.NewContainer(di.ServiceConstructorMap{
			pkgcontainer.LoggingClientInterfaceName: func(get di.Get) interface{} {
				return lc
			},
			container.ConfigurationName: func(get di.Get) interface{} {
				return &config.ConfigurationStruct{Pprof: config.PprofInfo{Enabled: enabled}}
			},
			container.UserItfName: func(get di.Get) interface{} {
				return userItfStub{}
			},
			pkgcontainer.StreamClientName: func(get di.Get) interface{} {
				return streamclient.NewStreamClient(lc)
			},
		})
		engine := gin.New()
		RegisterGateway(engine, dic)
		found := false
		for _, r := range engine.Routes() {
			if r.Method == "GET" && r.Path == "/api/v1/swagger/*any" {
				found = true
				break
			}
		}
		if found != enabled {
			t.Fatalf("pprof enabled=%v, swagger route found=%v", enabled, found)
		}
	}
}
