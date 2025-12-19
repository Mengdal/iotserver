package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"iotServer/models"
	"iotServer/models/constants"
	"iotServer/models/dtos"
	"iotServer/services"
	"iotServer/utils"
	"time"
)

var GlobalSceneService = services.NewSceneService()

// SceneController 场景管理
type SceneController struct {
	BaseController
	sceneService *services.SceneService
}

func (c *SceneController) Prepare() {
	c.sceneService = GlobalSceneService // 全局实例
}

// Edit @Title 创建/更新场景联动
// @Description 创建或更新基础的场景联动
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body       	   body    dtos.SceneCreate  true  "场景（更新+ID,无法更改名称）"
// @Success 200 {object} controllers.Result
// @Failure 400 "请求出错"
// @router /edit [post]
func (c *SceneController) Edit() {
	var req dtos.SceneCreate
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)

	o := orm.NewOrm()
	scene := models.Scene{Name: req.Name, Description: req.Description, Department: &models.Department{Id: tenantId}, UserId: userId}
	scene.Status = string(constants.RuleStop) //默认关闭
	if req.Id == 0 {
		if err := o.Read(&scene, "name"); err != nil || scene.Id != 0 {
			c.Error(400, "current name is not available")
		}
		// 创建告警规则
		scene.BeforeInsert()
		_, err := o.Insert(&scene)
		if err != nil {
			c.Error(400, "插入失败:"+err.Error())
		}
	} else {
		scene.Id = req.Id
		scene.BeforeUpdate()
		_, err := o.Update(&scene)
		if err != nil {
			c.Error(400, "更新失败")
		}
	}
	c.SuccessMsg()
}

// List @Title 查询场景联动列表
// @Description 分页查询场景联动
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param	name		   query   string  false "单位名称"
// @Param	status		   query   string  false "状态"
// @Param   page           query   int     false "当前页码，默认1"
// @Param   size           query   int     false "每页数量，默认10"
// @Success 200 {object} controllers.Result
// @Failure 400 "请求出错"
// @router /list [post]
func (c *SceneController) List() {
	page, _ := c.GetInt("page", 1)
	size, _ := c.GetInt("size", 10)
	name := c.GetString("name")
	status := c.GetString("status")
	userId := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)

	var scens []*models.Scene
	o := orm.NewOrm()

	qs := o.QueryTable(new(models.Scene)).Filter("department_id", tenantId)
	if name != "" {
		qs = qs.Filter("name__icontains", name)
	}
	if status != "" {
		qs = qs.Filter("status", status)
	}
	paginate, err := utils.Paginate(qs, page, size, &scens)
	if err != nil {
		c.Error(400, "查询失败")
	}

	c.Success(paginate)
}

// Update @Title 配置场景联动
// @Description 配置场景联动并更新到 eKuiper
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body    	   body    dtos.SceneUpdateRequest  true  "场景联动信息"
// @Success 200 {object} controllers.Result
// @Failure 400 "请求出错"
// @router /update [post]
func (c *SceneController) Update() {
	var req dtos.SceneUpdateRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}
	// 2. 取出设备及属性类型
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)
	o := orm.NewOrm()
	scene := models.Scene{Id: req.Id}
	if err := o.Read(&scene); err != nil || scene.Department == nil || scene.Department.Id != tenantId {
		c.Error(400, "scene not found or no permission")
	}

	ActionMarshal, _ := json.Marshal(req.Action)
	ConditionMarshal, _ := json.Marshal(req.Condition)
	scene.Action = string(ActionMarshal)
	scene.Condition = string(ConditionMarshal)

	// 先更新场景数据
	if _, err := o.Update(&scene); err != nil {
		c.Error(400, "更新场景出错"+err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	//生成场景联动
	if req.Condition[0].ConditionType == "notify" && req.Id != 0 {
		services.BuildEkuiperRule(ctx, req, scene.Name)
	} else if req.Condition[0].ConditionType == "timer" && req.Id != 0 {
		if err := c.sceneService.StopScene(req.Id, userId); err != nil {
			c.Error(400, fmt.Sprintf(err.Error()))
		}
		if err := c.sceneService.StartScene(req.Id, userId); err != nil {
			c.Error(400, fmt.Sprintf(err.Error()))
		}
	}

	c.SuccessMsg()
}

// OperateScene @Title 启动/停止/重启/删除场景
// @Description 操作场景联动，restart立即启动一次用于测试
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body           body    dtos.OperateSceneReq  true  "启动请求"
// @Success 200 {object} controllers.Result
// @Failure 400 "请求出错"
// @router /operate [post]
func (c *SceneController) OperateScene() {
	var req dtos.OperateSceneReq
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)

	var err error
	var message string

	switch req.Action {
	case "start":
		err = c.sceneService.StartScene(req.SceneId, userId)
		message = "场景已启动"
	case "stop":
		err = c.sceneService.StopScene(req.SceneId, userId)
		message = "场景已停止"
	case "delete":
		err = c.sceneService.DeleteScene(req.SceneId, userId)
		message = "场景已删除"
	case "restart":
		err = c.sceneService.RestartScene(req.SceneId, userId)
		message = "场景已执行"
	default:
		c.Error(400, "无效的操作类型，仅支持 start、stop、restart、delete")
	}

	if err != nil {
		c.Error(400, fmt.Sprintf("%s失败: %v", message, err))
	}

	c.SuccessMsg()
}

// GetSceneStatus @Title 获取场景状态详情
// @Description 获取指定场景或全部场景的状态信息,仅查询已启动的场景
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   sceneId        query    string  false  "场景ID,不填默认查询全部"
// @Success 200 {object} controllers.SimpleResult
// @Failure 400 "请求出错"
// @router /status [get]
func (c *SceneController) GetSceneStatus() {
	sceneId := c.GetString("sceneId")
	var result []dtos.SceneStatus

	jobs := c.sceneService.Jobs()           // 假设返回 map[string]EntryID
	entries := c.sceneService.CronEntries() // 假设返回 []cron.Entry

	if sceneId != "" {
		// 查询单个场景
		if entryID, ok := jobs[sceneId]; ok {
			for _, entry := range entries {
				if entry.ID == entryID {
					result = append(result, dtos.SceneStatus{
						SceneID: sceneId,
						EntryID: int(entryID),
						Next:    entry.Next,
						Prev:    entry.Prev,
					})
					break
				}
			}
		}
	} else {
		// 查询全部
		for sceneID, entryID := range jobs {
			for _, entry := range entries {
				if entry.ID == entryID {
					result = append(result, dtos.SceneStatus{
						SceneID: sceneID,
						EntryID: int(entryID),
						Next:    entry.Next,
						Prev:    entry.Prev,
					})
					break
				}
			}
		}
	}
	c.Success(result)
}
