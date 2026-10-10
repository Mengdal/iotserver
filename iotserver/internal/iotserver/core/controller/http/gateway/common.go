package gateway

import (
	"fmt"
	"github.com/gin-gonic/gin"

	"github.com/gorilla/schema"
	"iotserver/internal/dtos"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
	"iotserver/internal/pkg/logger"
	"net/http"
	"net/http/httputil"
	"net/url"
)

const (
	UrlParamSceneId         = "sceneId"
	UrlParamActionId        = "actionId"
	UrlParamStrategyId      = "strategyId"
	UrlParamConditionId     = "conditionId"
	UrlParamJobId           = "jobId"
	UrlParamProductId       = "productId"
	UrlParamCategoryKey     = "categoryKey"
	UrlParamCloudInstanceId = "cloudInstanceId"
	UrlParamDeviceId        = "deviceId"
	UrlParamFuncPointId     = "funcPointId"
	UrlParamDeviceLibraryId = "deviceLibraryId"
	UrlParamDeviceServiceId = "deviceServiceId"
	UrlParamDockerConfigId  = "dockerConfigId"
	UrlParamRuleId          = "ruleId"
	UrlDataResourceId       = "dataResourceId"
	RuleEngineId            = "ruleEngineId"
	SceneId                 = "sceneId"
)

// checkCallbackToken 校验 eKuiper 回调共享密钥。
// eKuiper 规则动作在 /api/v1/ekuiper/* 上回调平台，该路径无法走 JWT，因此用配置的共享密钥保护；
// 未配置密钥时保持历史行为（不校验）。
func checkCallbackToken(c *gin.Context) error {
	expect := dtos.CallbackTokenHeader()
	if expect == nil {
		return nil
	}
	if c.GetHeader(dtos.CallbackTokenHeaderName) != expect[dtos.CallbackTokenHeaderName] {
		return errort.NewCommonErr(errort.DefaultTokenPermission, fmt.Errorf("回调鉴权失败"))
	}
	return nil
}

var decoder *schema.Decoder

func init() {
	decoder = schema.NewDecoder()
	decoder.IgnoreUnknownKeys(true)
}

func urlDecodeParam(obj interface{}, r *http.Request, lc logger.LoggingClient) {
	err := decoder.Decode(obj, r.URL.Query())
	if err != nil {
		lc.Errorf("url decoding err %v", err)
	}
}

func (ctl *controller) ProxyAgentServer(c *gin.Context) {
	proxy := ctl.cfg.Clients["Agent"]
	ctl.lc.Infof("agentProxy: %v", proxy)
	ctl.ServeHTTP(c, fmt.Sprintf("http://%v:%v", proxy.Host, proxy.Port))
}

//func (ctl *controller) ProxySharpServer(c *gin.Context) {
//	proxy := ctl.cfg.Clients["Sharp"]
//	ctl.lc.Infof("sharpProxy: %v", proxy)
//	ctl.ServeHTTP(c, fmt.Sprintf("http://%v:%v", proxy.Host, proxy.Port))
//}

func (ctl *controller) ServeHTTP(c *gin.Context, URL string) {
	parseRootUrl, err := url.Parse(URL)
	if err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(parseRootUrl)
	proxy.ServeHTTP(c.Writer, c.Request)
}
