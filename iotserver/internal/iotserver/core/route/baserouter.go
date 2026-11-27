package route

import (
	"iotserver/internal/iotserver/core/container"
	pkgContainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/httphelper"
	"iotserver/internal/pkg/utils"
	//"iotserver/internal/system/monitor/container"

	//"gitlab.com/tedge/edgex/internal/pkg/constants"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

func LoadWebProxyRoutes(r *gin.Engine, webBuildPath string, dic *di.Container) {
	r.Use(ProxyWeb(r, webBuildPath, dic)).Use(NoCacheAppShell()).Use(static.ServeRoot("/", webBuildPath))
}

// NoCacheAppShell 让 index.html 始终回源校验，避免浏览器缓存旧「应用外壳」；
// 带内容哈希的静态资源（/static/...）保持默认缓存，不重复下载。
func NoCacheAppShell() gin.HandlerFunc {
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		if p == "/" || strings.HasSuffix(p, "/index.html") || !strings.Contains(p, ".") {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}
		c.Next()
	}
}

// ProxyServer http proxy
func ProxyServer(c *gin.Context, dic *di.Container) {
	configuration := container.ConfigurationFrom(dic.Get)

	port := strconv.Itoa(configuration.Service.Port)
	addr := configuration.Service.ServerBindAddr + ":" + port

	lc := pkgContainer.LoggingClientFrom(dic.Get)
	parseRootUrl, err := url.Parse("http://" + addr)
	if err != nil {
		lc.Errorf("parse server url err:%v", err)
		c.Data(502, "", []byte("proxy server error"))
	}
	// 规则试运行 SSE 是长连接：清掉 WebProxy 的 WriteTimeout（默认 120s），否则长调试会被截断
	if strings.Contains(c.Request.URL.Path, "/rule-engine/debug/") && strings.HasSuffix(c.Request.URL.Path, "/stream") {
		if rw, ok := httphelper.RawResponseWriter(c.Request.Context()); ok {
			if err := http.NewResponseController(rw).SetWriteDeadline(time.Time{}); err != nil {
				lc.Warnf("proxy rule debug stream: clear write deadline failed: %v", err)
			}
		}
	}
	proxy := httputil.NewSingleHostReverseProxy(parseRootUrl)
	proxy.ServeHTTP(c.Writer, c.Request)
}

// ProxyWeb 转发
func ProxyWeb(g *gin.Engine, webBuildPath string, dic *di.Container) gin.HandlerFunc {
	return func(context *gin.Context) {
		ReplaceURLPrefix(context, dic)
		uri := context.Request.URL.Path
		if ok, _ := regexp.MatchString("^(/api/|/v1.0/)", uri); ok {
			ProxyServer(context, dic)
			context.Abort()
			return
		}

		absPath := webBuildPath + context.Request.URL.Path
		if utils.FilePathIsExist(absPath) {
			return
		}
		context.Request.URL.Path = "/"
		// 判断index.html文件是否存在
		indexPath := webBuildPath + "/index.html"
		if !utils.FilePathIsExist(indexPath) {
			context.Data(404, "", []byte("404 not found"))
			context.Abort()
			return
		}
		g.HandleContext(context)
	}
}

func ReplaceURLPrefix(context *gin.Context, dic *di.Container) {
	lc := pkgContainer.LoggingClientFrom(dic.Get)
	//get prefix from env
	prefix := os.Getenv("URLPrefix")
	if prefix == "" {
		return
	}

	prefix = prefix + "/"
	context.Request.URL.Path = strings.ReplaceAll(context.Request.URL.Path, prefix, "")
	lc.Debugf("after replace url path:%s", context.Request.URL.Path)
}
