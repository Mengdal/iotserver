package locales

import (
	"regexp"
	"testing"
)

var cjk = regexp.MustCompile(`[\p{Han}]`)

// 词条约定的守卫测试：
//
//  1. 错误码/命名文案：中英文必须一一对应，少一条就会出现「某种语言下直接显示错误码」。
//  2. 后端接口的细节文案（errort.NewMessageErr）用「中文原文即 ID」的写法，
//     这类词条只有英文是正常的——中文取不到词条时会按 ID 当模板渲染出中文原文。
//     但要保证「只有英文」的词条 ID 里必须含中文，否则中文用户会看到一串英文 key。
func TestMessageIDsMatch(t *testing.T) {
	// 同一个 ID 出现多次时后面的会覆盖前面的，容易埋「改了文案不生效」的坑
	zh := map[string]string{}
	zhDup := make([]string, 0)
	for _, m := range GetZhMessages() {
		if _, ok := zh[m.ID]; ok {
			zhDup = append(zhDup, m.ID)
		}
		zh[m.ID] = m.Other
	}
	en := map[string]string{}
	enDup := make([]string, 0)
	for _, m := range GetEnMessages() {
		if _, ok := en[m.ID]; ok {
			enDup = append(enDup, m.ID)
		}
		en[m.ID] = m.Other
	}
	if len(zhDup) > 0 {
		t.Errorf("中文词条 ID 重复：%v", zhDup)
	}
	if len(enDup) > 0 {
		t.Errorf("英文词条 ID 重复：%v", enDup)
	}

	// 中文有的，英文必须有
	for id := range zh {
		if _, ok := en[id]; !ok {
			t.Errorf("英文缺少词条 %s（中文为 %q）", id, zh[id])
		}
	}

	// 英文独有的词条：必须是「中文原文作为 ID」的约定
	for id := range en {
		if _, ok := zh[id]; ok {
			continue
		}
		if !cjk.MatchString(id) {
			t.Errorf("英文独有词条 %s 不含中文，中文用户会直接看到这个 key", id)
		}
	}

	// 空的 Other 会让 localizer 找不到内容，等于没翻译
	for id, other := range zh {
		if other == "" {
			t.Errorf("中文词条 %s 内容为空", id)
		}
	}
	for id, other := range en {
		if other == "" {
			t.Errorf("英文词条 %s 内容为空", id)
		}
	}
}
