package route

import (
	"context"
	"os"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	gs "github.com/swaggo/gin-swagger"
	"golang.org/x/net/webdav"

	"iotserver/internal/iotserver/core/container"
	"iotserver/internal/pkg/di"
)

// registerSwagger 只在调试模式（Pprof.Enabled=true）下注册 Swagger UI。
//
// swaggo/files 的 init 会把约 5MB 的 UI 资源写进内存文件系统；生产模式既不注册路由，
// 也把这些缓存文件删掉，释放堆内存。下次以调试模式重启时会由 init 重新加载。
func registerSwagger(v1 *gin.RouterGroup, dic *di.Container) {
	if container.ConfigurationFrom(dic.Get).Pprof.Enabled {
		v1.GET("/swagger/*any", gs.WrapHandler(swaggerFiles.Handler))
		return
	}
	releaseSwaggerAssets()
}

// releaseSwaggerAssets 清空 swaggo/files 的内存文件系统（根目录 RemoveAll 不被支持，需逐个删除）。
func releaseSwaggerAssets() {
	releaseSwaggerFileSystem(swaggerFiles.FS)
}

func releaseSwaggerFileSystem(fs webdav.FileSystem) {
	ctx := context.Background()
	root, err := fs.OpenFile(ctx, "/", os.O_RDONLY, 0)
	if err != nil {
		return
	}
	entries, err := root.Readdir(-1)
	_ = root.Close()
	if err != nil {
		return
	}
	for _, entry := range entries {
		_ = fs.RemoveAll(ctx, "/"+entry.Name())
	}
}
