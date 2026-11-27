package i18n

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMatchLang(t *testing.T) {
	cases := []struct {
		raw  string
		want string
		ok   bool
	}{
		{"zh-CN", LangZhCN, true},
		{"zh", LangZhCN, true},
		{"zh-Hans", LangZhCN, true},
		{"en-US", LangEnUS, true},
		{"en", LangEnUS, true},
		{"en-GB", LangEnUS, true},
		{"en-US,en;q=0.9,zh-CN;q=0.8", LangEnUS, true},
		{"zh-CN,zh;q=0.9,en;q=0.8", LangZhCN, true},
		{" fr-FR ", "", false},
		{"de,fr;q=0.8", "", false},
		{"", "", false},
		{"   ", "", false},
	}
	for _, c := range cases {
		got, ok := matchLang(c.raw)
		if got != c.want || ok != c.ok {
			t.Errorf("matchLang(%q) = (%q, %v), 期望 (%q, %v)", c.raw, got, ok, c.want, c.ok)
		}
	}
}

func TestGetLangDefault(t *testing.T) {
	if got := GetLang(httptest.NewRequest(http.MethodGet, "/", nil).Context()); got != DefaultLang {
		t.Errorf("没有上下文语言时 GetLang = %q, 期望 %q", got, DefaultLang)
	}
}

// 语言来源优先级：?lang= > Accept-Language > 默认语言
func TestI18nHandlerGin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name   string
		target string
		header string
		want   string
	}{
		{"只有 Accept-Language", "/api/v1/x", "en-US", LangEnUS},
		{"带 q 值的 Accept-Language", "/api/v1/x", "en-US,en;q=0.9", LangEnUS},
		{"query 覆盖 header", "/api/v1/ws/?lang=en-US", "zh-CN", LangEnUS},
		{"query 也认 zh", "/api/v1/ws/?lang=zh", "en-US", LangZhCN},
		{"都不认识时用默认语言", "/api/v1/x", "fr-FR", DefaultLang},
		{"什么都没有时用默认语言", "/api/v1/x", "", DefaultLang},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got string
			var fromCtx string
			engine := gin.New()
			engine.Use(I18nHandlerGin())
			engine.GET("/api/v1/x", func(ctx *gin.Context) {
				got = GetLang(ctx.Request.Context())
				fromCtx = ctx.GetString(AcceptLanguage)
				ctx.Status(http.StatusOK)
			})
			engine.GET("/api/v1/ws/", func(ctx *gin.Context) {
				got = GetLang(ctx.Request.Context())
				fromCtx = ctx.GetString(AcceptLanguage)
				ctx.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, c.target, nil)
			if c.header != "" {
				req.Header.Set(AcceptLanguage, c.header)
			}
			engine.ServeHTTP(httptest.NewRecorder(), req)

			if got != c.want {
				t.Errorf("请求上下文语言 = %q, 期望 %q", got, c.want)
			}
			// gin 的 c.Set 与 request context 必须一致，WebSocket 那边取的是 c.ctx
			if fromCtx != c.want {
				t.Errorf("gin.Context 里的语言 = %q, 期望 %q", fromCtx, c.want)
			}
		})
	}
}

type requestContextKey struct{}

// 语言中间件替换 Request.Context() 时必须保留上游放入的值
// （例如 httphelper 保存的原始 ResponseWriter，SSE 清写超时依赖它）。
func TestI18nMiddlewareKeepsRequestContextValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), requestContextKey{}, "kept")
		c.Request = c.Request.WithContext(ctx)
	})
	r.Use(I18nHandlerGin())
	r.GET("/", func(c *gin.Context) {
		value, _ := c.Request.Context().Value(requestContextKey{}).(string)
		c.String(http.StatusOK, value)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)
	if w.Body.String() != "kept" {
		t.Fatalf("请求上下文值丢失: %q", w.Body.String())
	}
}
