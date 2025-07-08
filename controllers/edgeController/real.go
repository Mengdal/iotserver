package edgeController

import (
	"encoding/json"
	"fmt"
	"iotServer/controllers"
	"iotServer/iotp"
	"iotServer/models"
	"strings"
)

type RealController struct {
	controllers.BaseController
}

// QueryReal @Title 实时数据查询
// @Description 根据传入参数查询实时数据
// @Param   body           body    models.HistoryObject  true  "查询条件"
// @Success 200 {object}   controllers.SimpleResult "返回结果"
// @Failure 400 "错误信息"
// @router /queryReal [post]
func (c *RealController) QueryReal() {
	var objects models.HistoryObject
	rawJSON := c.Ctx.Input.RequestBody
	err := json.Unmarshal(rawJSON, &objects)
	if err != nil {
		c.Error(400, err.Error())
	}
	devices := make(map[string][]string)
	for _, tagId := range objects.IDs {
		l := strings.Split(tagId, ".")
		if len(l) != 2 {
			fmt.Println("deviceCode或tagCode中不允许有.")
			continue
		}
		deviceCode := l[0]
		tagCode := l[1]
		if _, ok := devices[deviceCode]; !ok {
			devices[deviceCode] = make([]string, 0)
		}
		devices[deviceCode] = append(devices[deviceCode], tagCode)
	}
	data, err := iotp.GetRealData(devices)
	if err != nil {
		c.Error(400, err.Error())
	}
	trueData := make([]iotp.Record, 0)
	for _, tag := range data {
		if tag.Val == true {
			trueData = append(trueData, tag)
		}
	}
	c.Success(trueData)
}
