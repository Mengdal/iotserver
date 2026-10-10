// Package valid 提供跨模块共用的参数校验。
package valid

import "regexp"

// namePattern 名称规则：1-32 位，必须字母或中文开头，允许字母/数字/中文/_/-
// 与前端（资源管理、规则引擎表单）的校验保持一致。
var namePattern = regexp.MustCompile("^[a-zA-Z\u4e00-\u9fa5][a-zA-Z0-9\u4e00-\u9fa5_-]{0,31}$")

// Name 校验名称是否符合平台规则
func Name(name string) bool {
	return namePattern.MatchString(name)
}

// NameRuleMsg 名称不合法时的提示（与前端文案一致）
const NameRuleMsg = "1-32位字符，支持中文、英文、数字及特殊字符_-，必须以英文或中文字符开头"
