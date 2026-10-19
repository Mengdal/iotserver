package i18n

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/i18n/locales"
	"strconv"
	"strings"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	//"gitlab.com/tedge/edgex/internal/pkg/i18n/locales"
	"golang.org/x/text/language"
)

var bundle *i18n.Bundle

func init() {
	bundle = i18n.NewBundle(language.Chinese)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	// load message
	initMessages()
}

func initMessages() {
	err := bundle.AddMessages(language.Make(language.English.String()), locales.GetEnMessages()...)
	if err != nil {
		panic(err)
	}
	err = bundle.AddMessages(language.Make(language.Chinese.String()), locales.GetZhMessages()...)
	if err != nil {
		panic(err)
	}
}

// 常量的翻译 Trans("en", "device.name", nil)
func Trans(lang string, messageId string, params map[string]interface{}) string {
	localizer := i18n.NewLocalizer(bundle, lang)
	retStr, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    messageId,
		TemplateData: params,
		Funcs: template.FuncMap{
			"filter":     funcFilter,
			"trim":       funcTrim,
			"left_space": funcAddLeftSpace,
		},
	})
	if err != nil {
		// 词条缺失时的兜底：按「ID 就是原文模板」的约定把参数填进去。
		// 中文因此不用单独维护词条（见 errort.NewMessageErr）。
		if params != nil {
			return renderFallback(messageId, params)
		}
		return messageId
	}
	return retStr
}

// LocalizeError 把错误转成当前语言的文案。
// 错误链里带 errort.MessageError（词条 ID + 参数）时返回按请求语言渲染的文案与 ok=true；
// 其它错误返回 ok=false，调用方自己决定怎么展示（避免把内部错误细节拼进用户文案）。
func LocalizeError(ctx context.Context, err error) (string, bool) {
	if err == nil {
		return "", false
	}
	var msgErr *errort.MessageError
	if errors.As(err, &msgErr) {
		return Trans(GetLang(ctx), msgErr.ID, msgErr.Params), true
	}
	return "", false
}

// renderFallback 把 ID 当模板渲染出带参数的中文原文；渲染不了就原样返回
func renderFallback(text string, params map[string]interface{}) string {
	tmpl, err := template.New("i18n.fallback").Parse(text)
	if err != nil {
		return text
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return text
	}
	return buf.String()
}

// 错误码的翻译 TransCode(ctx,10001, nil)：语言取自请求上下文（Accept-Language / ?lang=）
func TransCode(ctx context.Context, code uint32, params map[string]interface{}) string {
	return transCodeWithLang(GetLang(ctx), code, params)
}

// transCodeWithLang 按指定语言翻译错误码
func transCodeWithLang(lang string, code uint32, params map[string]interface{}) string {
	localizer := i18n.NewLocalizer(bundle, lang)
	strCode := strconv.Itoa(int(code))
	retStr, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    strCode,
		TemplateData: params,
		Funcs: template.FuncMap{
			"filter":     funcFilter,
			"trim":       funcTrim,
			"left_space": funcAddLeftSpace,
		},
	})
	if err != nil {
		return strCode
	}
	return retStr
}

// TransCodeDefault 错误码转换, 用于告警显示, 只显示英文错误信息。
// 这里显式指定英文，不再依赖「上下文取不到语言时」的默认值，避免跟着默认语言一起变。
func TransCodeDefault(code uint32, params map[string]interface{}) string {
	return "[Code(" + strconv.Itoa(int(code)) + ")] " + transCodeWithLang(LangEnUS, code, params)
}

func funcFilter(a interface{}) string {
	if a == nil {
		return ""
	}
	return fmt.Sprintf("%v", a)
}

func funcTrim(a interface{}) string {
	if a == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", a))
}

func funcAddLeftSpace(a interface{}) string {
	if a == nil || a == "" {
		return ""
	}
	return fmt.Sprintf(" %v", a)
}
