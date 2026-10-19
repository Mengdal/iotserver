package i18n

import (
	"context"
	"strings"
	"testing"
)

func ctxWithLang(lang string) context.Context {
	return context.WithValue(context.Background(), AcceptLanguage, lang)
}

// 验证整条链路：请求语言 → bundle → 词条。前端传的就是 zh-CN / en-US 这两个值。
func TestTransCodeLanguageSwitch(t *testing.T) {
	const systemError = uint32(10001)
	zh := TransCode(ctxWithLang(LangZhCN), systemError, nil)
	en := TransCode(ctxWithLang(LangEnUS), systemError, nil)

	if zh == en {
		t.Fatalf("中英文返回了同一段文案，说明语言没生效：%q", zh)
	}
	if !strings.Contains(zh, "系统错误") {
		t.Errorf("中文文案不对：%q", zh)
	}
	if !strings.Contains(strings.ToLower(en), "system error") {
		t.Errorf("英文文案不对：%q", en)
	}
}

// 没有上下文语言时用默认语言（中文）
func TestTransCodeDefaultLang(t *testing.T) {
	if got := TransCode(context.Background(), 10001, nil); !strings.Contains(got, "系统错误") {
		t.Errorf("默认语言下应返回中文，实际：%q", got)
	}
}

// 词条里的模板参数（10003 请求参数错误 + info）
func TestTransCodeWithParams(t *testing.T) {
	params := map[string]interface{}{"info": "name"}
	zh := TransCode(ctxWithLang(LangZhCN), 10003, params)
	en := TransCode(ctxWithLang(LangEnUS), 10003, params)

	if !strings.Contains(zh, "name") {
		t.Errorf("中文文案没带上参数：%q", zh)
	}
	if !strings.Contains(en, "name") {
		t.Errorf("英文文案没带上参数：%q", en)
	}
}

// 没收录的错误码要原样返回错误码，而不是空串
func TestTransCodeUnknownCode(t *testing.T) {
	const unknown = uint32(999999)
	if got := TransCode(ctxWithLang(LangEnUS), unknown, nil); got != "999999" {
		t.Errorf("未收录的错误码应原样返回，实际：%q", got)
	}
}

// 告警用的 TransCodeDefault 固定英文，不随请求语言变化
func TestTransCodeDefaultAlwaysEnglish(t *testing.T) {
	got := TransCodeDefault(10001, nil)
	if !strings.HasPrefix(got, "[Code(10001)] ") {
		t.Errorf("前缀不对：%q", got)
	}
	if !strings.Contains(strings.ToLower(got), "system error") {
		t.Errorf("应固定为英文：%q", got)
	}
}

// 命名文案（非错误码）也走同一套 bundle
func TestTransNamedMessage(t *testing.T) {
	zh := Trans(LangZhCN, CloudInstanceLogResp, map[string]interface{}{"name": "svc", "status": ""})
	en := Trans(LangEnUS, CloudInstanceLogResp, map[string]interface{}{"name": "svc", "status": ""})
	if zh == en {
		t.Fatalf("命名文案的中英文一样：%q", zh)
	}
	if !strings.Contains(zh, "svc") || !strings.Contains(en, "svc") {
		t.Errorf("命名文案没带上参数：zh=%q en=%q", zh, en)
	}
}
