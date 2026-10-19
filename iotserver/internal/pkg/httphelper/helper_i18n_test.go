package httphelper

import (
	"context"
	"encoding/json"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/i18n"
	"iotserver/internal/pkg/logger"
	"net/http/httptest"
	"strings"
	"testing"
)

func ctxWithLang(lang string) context.Context {
	return context.WithValue(context.Background(), i18n.AcceptLanguage, lang)
}

// 带词条的错误细节：中文按「原文即模板」渲染，英文取词条
func TestErrorParamsLocalizedMessage(t *testing.T) {
	err := errort.NewCommonErr(errort.DefaultReqParamsError,
		errort.NewMessageErr("一次最多查询 {{.p1}} 个组件", map[string]interface{}{"p1": 5}))

	zh, _ := errorParams(ctxWithLang(i18n.LangZhCN), errort.NewCommonEdgeXWrapper(err))["info"].(string)
	en, _ := errorParams(ctxWithLang(i18n.LangEnUS), errort.NewCommonEdgeXWrapper(err))["info"].(string)

	if zh != "一次最多查询 5 个组件" {
		t.Errorf("中文细节不对：%q", zh)
	}
	if en != "At most 5 widgets per query" {
		t.Errorf("英文细节不对：%q", en)
	}
}

// 不带参数的词条：中文就是 ID 本身
func TestErrorParamsPlainMessage(t *testing.T) {
	err := errort.NewCommonErr(errort.DefaultReqParamsError, errort.NewMessageErr("没有参数的中文提示", nil))
	zh, _ := errorParams(ctxWithLang(i18n.LangZhCN), errort.NewCommonEdgeXWrapper(err))["info"].(string)
	if zh != "没有参数的中文提示" {
		t.Errorf("无参数词条渲染不对：%q", zh)
	}
}

func renderFail(t *testing.T, lang string, err error) CommonResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	RenderFail(ctxWithLang(lang), err, rec, logger.NewMockClient())
	var resp CommonResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	return resp
}

// 整条响应：中文用户看到中文，英文用户看到英文
func TestRenderFailLocalized(t *testing.T) {
	err := errort.NewCommonErr(errort.DefaultReqParamsError, errort.NewMessageErr("缺少 rule_id", nil))

	zh := renderFail(t, i18n.LangZhCN, err)
	en := renderFail(t, i18n.LangEnUS, err)

	if zh.Success || zh.ErrorCode != errort.DefaultReqParamsError {
		t.Errorf("错误码/成功位不对：%+v", zh)
	}
	if !strings.Contains(zh.ErrorMsg, "请求参数错误") || !strings.Contains(zh.ErrorMsg, "缺少 rule_id") {
		t.Errorf("中文响应文案不对：%q", zh.ErrorMsg)
	}
	if !strings.Contains(en.ErrorMsg, "Missing rule_id") {
		t.Errorf("英文响应文案不对：%q", en.ErrorMsg)
	}
	if strings.Contains(en.ErrorMsg, "缺少") {
		t.Errorf("英文响应里不应出现中文：%q", en.ErrorMsg)
	}
}
