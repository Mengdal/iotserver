package controllers

import (
	"github.com/beego/beego/v2/client/orm"
	"iotServer/models"
	"iotServer/services"
	"iotServer/utils"
)

type WriteController struct {
	BaseController
}

// Command @Title 控制下发
// @Description 发送控制命令
// @Param Authorization    header   string  true   "Bearer YourToken"
// @Param deviceCode query  string true "设备"
// @Param tagCode    query  string true "tag点"
// @Param val        query  string true "值"
// @Success 200 {string} string "控制命令发送成功"
// @Failure 500 {object} map[string]interface{} "控制命令发送失败"
// @router /command [post]
func (c *WriteController) Command(deviceCode, tagCode, val string) {
	userId := c.Ctx.Input.GetData("user_id").(int64)
	// 发布控制命令
	if _, err := services.Processor.Deal(deviceCode, tagCode, val, "手动控制", userId); err != nil {
		c.Error(400, "控制命令发送失败,"+err.Error())
	}
	c.SuccessMsg()
}

// Log @Title 控制日志
// @Description 场景控制下userId = 场景控制,手动控制userId = 操作用户
// @Param Authorization    header   string  true   "Bearer YourToken"
// @Param   id      	   query   id      false "筛选ID"
// @Param   type 		   query   string  false "查询类型：场景控制/手动控制/组态下发"
// @Param   page           query   int     false "当前页码，默认1"
// @Param   size           query   int     false "每页数量，默认10"
// @Success 200 {object} controllers.Result
// @Failure 400 "请求出错"
// @router /log [post]
func (c *WriteController) Log() {
	id, _ := c.GetInt64("id")
	logType := c.GetString("type")
	page, _ := c.GetInt("page", 1)
	size, _ := c.GetInt("size", 10)

	var logs []*models.WriteLog
	o := orm.NewOrm()
	qs := o.QueryTable(new(models.WriteLog))
	if logType == "手动控制" || logType == "组态下发" {
		qs = qs.Filter("channel", logType)
	}

	var sceneName string
	if logType == "场景控制" {
		qs = qs.Filter("channel", logType)
		if id != 0 {
			var scene models.Scene
			scene.Id = id
			if err := o.Read(&scene); err != nil {
				c.Error(400, "查询失败")
			}
			sceneName = scene.Name
			qs.Filter("user_id", id)
		}
	}
	paginate, err := utils.Paginate(qs, page, size, &logs)
	if err != nil {
		c.Error(400, "查询失败")
	}

	// 修改List中的每个元素，添加场景名称
	if paginate.List != nil {
		// 类型断言获取 []*models.WriteLog
		if logList, ok := paginate.List.(*[]*models.WriteLog); ok && len(*logList) > 0 {
			// 创建新的结果列表
			resultList := make([]map[string]interface{}, len(*logList))
			for i, log := range *logList {
				// 将原始日志对象转换为map，然后添加场景名称
				resultList[i] = map[string]interface{}{
					"seq":      log.Seq,
					"UserId":   log.UserId,
					"sn":       log.Sn,
					"dn":       log.Dn,
					"tag":      log.Tag,
					"val":      log.Val,
					"status":   log.Status,
					"channel":  log.Channel,
					"respTime": log.RespTime,
					"created":  log.Created,
					"name":     sceneName, // 添加场景名称字段
				}
			}

			// 更新分页结果
			paginate.List = resultList
		}
	}

	c.Success(paginate)

}
