package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"iotServer/common"
	"iotServer/models/dtos"
	"net/http"
)

var UdfURL = common.EkuiperServer + "/udf/javascript"

type UdfController struct {
	BaseController
}

// Create @Title UDF创建
// @Description 创建用户自定义函数
// @Param   Authorization  header    string    true  "Bearer YourToken"
// @Param   body           body      dtos.Udf  true  "标签查询参数"
// @Success 200 {object}   controllers.SimpleResult
// @Failure 400 "错误信息"
// @router /create [post]
func (c *UdfController) Create() {
	var req dtos.Udf
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}

	body, _ := json.Marshal(req)
	resp, err := http.Post(UdfURL, "application/json", bytes.NewReader(body))
	if err != nil {
		c.Error(500, "调用 eKuiper API 失败: "+err.Error())
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	c.Error(resp.StatusCode, string(respBody))
}

// Get @Title UDF获取
// @Description 获取用户自定义函数详情或列表
// @Param   Authorization  header    string    true  "Bearer YourToken"
// @Param   name           query     string    false "唯一函数名称"
// @Success 200 {object}   controllers.SimpleResult
// @Failure 400 "错误信息"
// @router /get [post]
func (c *UdfController) Get() {
	name := c.GetString("name")
	var resp *http.Response
	var err error
	if name != "" {
		resp, err = http.Get(fmt.Sprintf("%s/%s", UdfURL, name))
		if err != nil {
			c.Error(500, "获取 UDF 详情失败: "+err.Error())
		}
	} else {
		resp, err = http.Get(UdfURL)
		if err != nil {
			c.Error(500, "获取 UDF 列表失败: "+err.Error())
		}

	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	c.Error(resp.StatusCode, string(respBody))
}

// Delete @Title UDF删除
// @Description 获取用户自定义函数详情或列表
// @Param   Authorization  header    string    true  "Bearer YourToken"
// @Param   name           query     string    true  "唯一函数名称"
// @Success 200 {object}   controllers.SimpleResult
// @Failure 400 "错误信息"
// @router /delete [post]
func (c *UdfController) Delete() {
	id := c.GetString("name")
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/%s", UdfURL, id), nil)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.CustomAbort(500, "删除 UDF 失败: "+err.Error())
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	c.Error(resp.StatusCode, string(respBody))
}

// Update @Title UDF更新
// @Description 更新用户自定义函数
// @Param   Authorization  header    string    true  "Bearer YourToken"
// @Param   body           body      dtos.Udf  true  "标签查询参数"
// @Success 200 {object}   controllers.SimpleResult
// @Failure 400 "错误信息"
// @router /update [post]
func (c *UdfController) Update() {
	var req dtos.Udf
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}

	body, _ := json.Marshal(req)
	reqHttp, _ := http.NewRequest("PUT", fmt.Sprintf("%s/%s", UdfURL, req.Id), bytes.NewReader(body))
	reqHttp.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(reqHttp)
	if err != nil {
		c.Error(500, "更新 UDF 失败: "+err.Error())
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	c.Error(resp.StatusCode, string(respBody))
}
