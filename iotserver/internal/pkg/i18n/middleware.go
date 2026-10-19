package i18n

import (
	"context"
	"strings"

	"golang.org/x/text/language"

	"github.com/gin-gonic/gin"
)

// 支持的语言标签（前端 ?lang= 与 Accept-Language 都传这套值，与 locales 里的词条对应）
const (
	LangZhCN = "zh-CN"
	LangEnUS = "en-US"
)

// DefaultLang 默认语言：与前端默认语言保持一致（中文）
const DefaultLang = LangZhCN

// 顺序即优先级：命中不了任何一项时按 No 处理，交给默认语言
var (
	supportedTags = []language.Tag{language.SimplifiedChinese, language.AmericanEnglish}
	langMatcher   = language.NewMatcher(supportedTags)
)

// matchLang 把任意语言标签归一化成 LangZhCN / LangEnUS。
// 支持 "zh"、"zh-Hans"、"zh-CN"、"en"、"en-GB" 以及带 q 值的 "en-US,en;q=0.9" 这类写法。
func matchLang(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	tags, _, err := language.ParseAcceptLanguage(raw)
	if err != nil || len(tags) == 0 {
		return "", false
	}
	_, idx, confidence := langMatcher.Match(tags...)
	if confidence == language.No {
		return "", false
	}
	if idx == 0 {
		return LangZhCN, true
	}
	return LangEnUS, true
}

// NormalizeLang 把任意语言标签归一化成 LangZhCN / LangEnUS；不认识的返回空串。
// 个人语言偏好等需要落库的场景用它做校验。
func NormalizeLang(raw string) string {
	if lang, ok := matchLang(raw); ok {
		return lang
	}
	return ""
}

// I18nHandlerGin 设置语言。
// 优先级：?lang= > Accept-Language > 默认语言。
// ?lang= 是给 WebSocket 用的：浏览器的 WebSocket API 不能自定义请求头，
// 只能把语言放在 query 里带过来（同时它也方便用 curl 直接验接口）。
func I18nHandlerGin() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := ""
		if query := c.Query("lang"); query != "" {
			if matched, ok := matchLang(query); ok {
				lang = matched
			}
		}
		if lang == "" {
			if matched, ok := matchLang(c.GetHeader(AcceptLanguage)); ok {
				lang = matched
			}
		}
		if lang == "" {
			lang = DefaultLang
		}
		c.Request = c.Request.WithContext(context.WithValue(c, AcceptLanguage, lang))
		c.Set(AcceptLanguage, lang)
		c.Next()
	}
}

// GetLang 取当前请求的语言（缺省用默认语言）
func GetLang(ctx context.Context) string {
	lang, ok := ctx.Value(AcceptLanguage).(string)
	if !ok || lang == "" {
		return DefaultLang
	}
	return lang
}
