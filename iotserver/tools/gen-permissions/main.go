// gen-permissions 把 internal/pkg/permissions 的权限标识导出为前端 TS 常量，
// 保证前后端权限码只有一份来源。
//
// 用法: go run ./tools/gen-permissions [输出路径]
package main

import (
	"fmt"
	"os"
	"strings"

	"iotserver/internal/pkg/permissions"
)

func main() {
	out := "../iotserver-ui/src/routes/permissions.ts"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}

	groups := permissions.NamedGroups()
	var b strings.Builder
	b.WriteString("/**\n")
	b.WriteString(" * 权限标识常量 —— 本文件由 `cd iotserver && go run ./tools/gen-permissions` 自动生成，请勿手改。\n")
	b.WriteString(" * 唯一来源: iotserver/internal/pkg/permissions/permissions.go\n")
	b.WriteString(" */\n\n")

	total := 0
	for _, group := range groups {
		for _, item := range group {
			b.WriteString(fmt.Sprintf("export const %s = '%s'\n", item.Name, item.Value))
			total++
		}
		b.WriteString("\n")
	}

	b.WriteString("/** 全部权限标识（与后端 /api/v1/user/getinfo 返回的子集一致） */\n")
	b.WriteString("export const PERMISSIONS: string[] = [\n")
	for _, group := range groups {
		b.WriteString("  ")
		for i, item := range group {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(item.Name)
		}
		b.WriteString(",\n")
	}
	b.WriteString("]\n")

	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "写入失败:", err)
		os.Exit(1)
	}
	fmt.Printf("已生成 %s（%d 个权限标识）\n", out, total)
}
