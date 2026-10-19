package seed

import (
	"strings"
	"testing"
)

// 存量看板的占位组件要能被内置定义升级，同时不能动用户自己的内容
func TestUpgradeBoardWidgets(t *testing.T) {
	preset, ok := BoardPresetById("builtin-home")
	if !ok {
		t.Fatal("缺少首页看板预设")
	}
	if !strings.Contains(string(preset.Canvas), `"type": "rank"`) {
		t.Fatalf("首页看板预设里的排行榜组件没更新: %s", string(preset.Canvas))
	}

	// 老画布：排行榜位置还是占位文本（用户调过布局和标题）+ 一个用户自己的文本组件
	old := `{"cols":12,"rowHeight":40,"widgets":[` +
		`{"id":"home-msg-rank","type":"text","text":"占位","title":"自定义标题","layout":{"x":8,"y":6,"w":4,"h":10}},` +
		`{"id":"user-text","type":"text","text":"用户自己的"}]}`
	next, changed := UpgradeBoardWidgets(old, preset.Canvas)
	if !changed {
		t.Fatal("占位组件应被升级")
	}
	if !strings.Contains(next, `"type":"rank"`) {
		t.Fatalf("占位组件没变成排行榜: %s", next)
	}
	if !strings.Contains(next, `"cols":12`) {
		t.Fatalf("画布其它字段丢失: %s", next)
	}
	if !strings.Contains(next, `"y":6`) || !strings.Contains(next, `"h":10`) {
		t.Fatalf("用户调过的布局被重置了: %s", next)
	}
	if !strings.Contains(next, "自定义标题") {
		t.Fatalf("自定义标题被覆盖了: %s", next)
	}
	if strings.Contains(next, "占位") {
		t.Fatalf("占位正文应被清掉: %s", next)
	}
	if !strings.Contains(next, "用户自己的") {
		t.Fatalf("自定义组件不应该被动: %s", next)
	}
	if _, again := UpgradeBoardWidgets(next, preset.Canvas); again {
		t.Fatal("升级应幂等，第二次不应再变化")
	}

	// 用户把该组件改成了别的类型：不要覆盖
	edited := `{"widgets":[{"id":"home-msg-rank","type":"table","config":{}}]}`
	if _, changed := UpgradeBoardWidgets(edited, preset.Canvas); changed {
		t.Fatal("用户改过类型的组件不应被覆盖")
	}
}
