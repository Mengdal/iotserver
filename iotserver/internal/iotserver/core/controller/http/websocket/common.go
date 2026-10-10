package websocket

import (
	"encoding/json"
	"iotserver/internal/dtos"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
	"iotserver/internal/pkg/i18n"

	"github.com/gin-gonic/gin/binding"
)

type SystemCheckLangReq struct {
	Lang string `json:"lang"`
}

/**
receive: {"code":10003,"data":{"lang":"zh"}}
*/
func CheckLang(c *wsClient, data interface{}, code dtos.WsCode) {
	var req SystemCheckLangReq
	bytes, _ := json.Marshal(data)
	err := binding.JSON.BindBody(bytes, &req)
	if err != nil {
		//c.lc.Error(err.Error())
		c.sendData(
			code,
			httphelper.WsResultFail(
				errort.DefaultReqParamsError,
				i18n.TransCode(c.ctx, errort.DefaultReqParamsError, nil),
			),
		)
		return
	}

	c.ChangeLang(req.Lang)
	c.sendData(code, httphelper.WsResult(errort.DefaultSuccess, nil, "", ""))
}
