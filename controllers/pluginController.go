package controllers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"iotServer/common"
	"iotServer/models/dtos"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

	var data interface{}
	if err := json.Unmarshal(respBody, &data); err != nil {
		c.Error(500, "Failed to parse service data: "+err.Error())
	}
	c.ErrorDetail(resp.StatusCode, "", data)
}

// Delete @Title UDF删除
// @Description 用户自定义函数删除
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

// EditService @Title 外部函数注册更新
// @Description 简化创建/更新外部函数
// @Param   Authorization  header    string          true  "Bearer YourToken"
// @Param   body           body      dtos.ExService  true  "外部函数DTO"
// @Success 200 {object}   controllers.SimpleResult
// @Failure 400 "错误信息"
// @router /editService [post]
func (c *UdfController) EditService() {
	var req dtos.ExService
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "Invalid JSON")
	}

	// 1. 默认 headers
	headers := map[string]string{
		"Accept-Charset": "utf-8",
	}

	// 2. 合并用户传入 headers
	for k, v := range req.Headers {
		headers[k] = v
	}

	// 1. 构造 JSON 配置
	serviceConfig := map[string]interface{}{
		"about": map[string]interface{}{
			"author": map[string]string{"name": "Beego Proxy"},
			"description": map[string]string{
				"zh_CN": req.Description,
			},
		},
		"interfaces": map[string]interface{}{
			req.Name: map[string]interface{}{
				"address":  req.Address,
				"protocol": "rest",
				"options": map[string]interface{}{
					"headers":            headers,
					"insecureSkipVerify": true,
				},
				"schemaless": true,
			},
		},
	}
	configData, _ := json.MarshalIndent(serviceConfig, "", "  ")

	// 2. 写入 static/ 下的 JSON 文件
	staticDir := "static"
	os.MkdirAll(staticDir, 0755)
	jsonPath := filepath.Join(staticDir, req.Name+".json")
	if err := os.WriteFile(jsonPath, configData, 0644); err != nil {
		c.Error(500, "Failed to write JSON: "+err.Error())
	}

	// 3. 打包成 ZIP 文件（static/xxx.zip）
	zipPath := filepath.Join(staticDir, req.Name+".zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		c.Error(500, "Failed to create zip: "+err.Error())
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	f, _ := zipWriter.Create(req.Name + ".json")

	jsonFile, _ := os.Open(jsonPath)
	defer jsonFile.Close()
	io.Copy(f, jsonFile)
	zipWriter.Close()

	// 4. 调用 eKuiper 注册服务
	zipURL := fmt.Sprintf("http://%s/static/%s.zip", common.GetIPByHostname()+":"+common.Port, req.Name)
	body := fmt.Sprintf(`{"name":"%s","file":"%s"}`, req.Name, zipURL)

	var resp *http.Response

	if req.Action == "create" {
		// 创建服务
		resp, err = http.Post(common.EkuiperServer+"/services", "application/json",
			strings.NewReader(body))
		if err != nil {
			c.Error(500, "Failed to call eKuiper API: "+err.Error())
		}
	} else if req.Action == "update" {
		// 更新服务（和创建一样的 JSON，只是调用 PUT 接口）
		client := &http.Client{}
		reqURL := fmt.Sprintf("%s/services/%s", common.EkuiperServer, req.Name)
		request, _ := http.NewRequest(http.MethodPut, reqURL, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		resp, err = client.Do(request)
		if err != nil {
			c.Error(500, "Failed to update service: "+err.Error())
		}
	} else {
		c.Error(400, "未包含的操作")
	}

	defer resp.Body.Close()

	respData, _ := io.ReadAll(resp.Body)
	c.Ctx.Output.SetStatus(resp.StatusCode)
	c.Ctx.Output.Body(respData)
}

// DeleteService @Title 删除外部函数
// @Description 删除外部函数
// @Param   Authorization  header    string     true  "Bearer YourToken"
// @Param   name           query     string     true  "外部函数名称"
// @Success 200 {object}   controllers.SimpleResult
// @Failure 400 "错误信息"
// @router /deleteService [post]
func (c *UdfController) DeleteService() {
	name := c.GetString("name")
	reqURL := fmt.Sprintf("%s/services/%s", common.EkuiperServer, name)
	client := &http.Client{}
	request, _ := http.NewRequest(http.MethodDelete, reqURL, nil)
	resp, err := client.Do(request)
	if err != nil {
		c.Error(500, "Failed to delete service: "+err.Error())
	}
	defer resp.Body.Close()
	respData, _ := io.ReadAll(resp.Body)
	c.ErrorDetail(resp.StatusCode, "", string(respData))
}

// ListServices @Title 查询所有外部函数
// @Description 查询外部函数List
// @Param   Authorization  header    string     true  "Bearer YourToken"
// @Success 200 {object}   controllers.SimpleResult
// @Failure 400 "错误信息"
// @router /listServices [post]
func (c *UdfController) ListServices() {
	resp, err := http.Get(common.EkuiperServer + "/services/functions")
	if err != nil {
		c.Error(500, "Failed to list services: "+err.Error())
	}
	defer resp.Body.Close()
	respData, _ := io.ReadAll(resp.Body)
	var data interface{}
	if err := json.Unmarshal(respData, &data); err != nil {
		c.Error(500, "Failed to parse functions data: "+err.Error())
	}
	c.ErrorDetail(resp.StatusCode, "", data)
}
