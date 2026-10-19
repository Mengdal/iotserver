package errort

// MessageError 带词条的错误：文案放在 i18n 词条里，渲染响应时按请求语言翻译。
//
// 约定：ID 就是「中文原文」（参数位置用 {{.p1}} {{.p2}} 占位），
// 与前端「中文原文即词条 key」的做法保持一致：
//   - 中文不需要单独维护词条：取不到词条时按 ID 当模板把参数渲染出来，就是完整中文；
//   - 英文只需要在 internal/pkg/i18n/locales/en.go 里补一条。
//
// 用法：
//
//	errort.NewCommonErr(errort.DefaultReqParamsError,
//	    errort.NewMessageErr("一次最多查询 {{.p1}} 个组件", map[string]interface{}{"p1": max}))
type MessageError struct {
	ID     string
	Params map[string]interface{}
}

func (e *MessageError) Error() string { return e.ID }

// NewMessageErr 构造带词条的错误，params 为 nil 时表示没有参数
func NewMessageErr(id string, params map[string]interface{}) *MessageError {
	return &MessageError{ID: id, Params: params}
}
