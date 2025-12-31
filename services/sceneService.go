package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"github.com/robfig/cron/v3"
	"iotServer/common"
	"iotServer/iotp"
	"iotServer/models"
	"iotServer/models/constants"
	"iotServer/models/dtos"
	"iotServer/utils"
	"log"
	"strconv"
	"sync"
	"time"
)

// SceneService 场景服务
type SceneService struct {
	cron *cron.Cron
	jobs map[int64]cron.EntryID // 场景ID -> 定时任务ID
	mu   sync.RWMutex
}

// NewSceneService 创建场景服务
func NewSceneService() *SceneService {
	service := SceneService{
		cron: cron.New(),
		jobs: make(map[int64]cron.EntryID),
	}
	service.cron.Start()
	return &service
}

// SceneList 获取场景列表
func (s *SceneService) SceneList(req dtos.SceneQueryRequest) (*dtos.SceneListResponse, error) {
	o := orm.NewOrm()

	// 构建查询条件
	qs := o.QueryTable("scene")
	if req.Name != "" {
		qs = qs.Filter("name__icontains", req.Name)
	}
	if req.Status != "" {
		qs = qs.Filter("status", req.Status)
	}

	// 获取总数
	total, err := qs.Count()
	if err != nil {
		return nil, err
	}

	// 分页查询
	var scenes []models.Scene
	offset := (req.Page - 1) * req.PageSize
	_, err = qs.Limit(req.PageSize, offset).OrderBy("-created").All(&scenes)
	if err != nil {
		return nil, err
	}

	// 转换为响应格式
	var list []dtos.SceneResponse
	for _, scene := range scenes {
		var conditions []dtos.Condition
		var actions []dtos.Action

		if scene.Condition != "" {
			json.Unmarshal([]byte(scene.Condition), &conditions)
		}
		if scene.Action != "" {
			json.Unmarshal([]byte(scene.Action), &actions)
		}

		response := dtos.SceneResponse{
			Id:          scene.Id,
			Name:        scene.Name,
			Description: scene.Description,
			Status:      scene.Status,
			Created:     scene.Created,
			Modified:    scene.Modified,
			Conditions:  conditions,
			Actions:     actions,
		}
		list = append(list, response)
	}

	return &dtos.SceneListResponse{
		Total: total,
		List:  list,
	}, nil
}

// StartScene 启动场景
func (s *SceneService) StartScene(id, userId int64) error {
	o := orm.NewOrm()
	scene := models.Scene{Id: id}
	tenantId, _ := models.GetUserTenantId(userId)
	if err := o.Read(&scene); err != nil || scene.Department == nil || scene.Department.Id != tenantId {
		return fmt.Errorf("scene not found or no permission")
	}

	// 检查是否已经在运行
	if scene.Status == string(constants.RuleStart) {
		return fmt.Errorf("场景已在运行中，请停止后编辑")
	}

	// 解析条件，加载定时场景
	if err := s.loadSceneToCron(scene, userId); err != nil {
		return fmt.Errorf("请重新配置该场景，Reason：" + err.Error())
	}
	// 更新状态
	scene.Status = "running"
	scene.BeforeUpdate()

	_, err := o.Update(&scene)
	return err
}

// StopScene 停止场景
func (s *SceneService) StopScene(id, userId int64) error {
	o := orm.NewOrm()
	scene := models.Scene{Id: id}
	tenantId, _ := models.GetUserTenantId(userId)
	if err := o.Read(&scene); err != nil || scene.Department == nil || scene.Department.Id != tenantId {
		return fmt.Errorf("scene not found or no permission")
	}

	// 移除定时任务
	if entryID, exists := s.jobs[id]; exists {
		s.cron.Remove(entryID)
		delete(s.jobs, id) // 清理映射，确保场景不再关联任务
	}

	// 停止ekuiper规则
	go func(name string) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		if err := common.Ekuiper.StopRule(ctx, name); err != nil {
			// 这里可以选择打印日志，不影响主流程
			log.Printf("StopRule 异步失败: %v", err)
		}
	}(scene.Name)

	// 更新状态
	scene.Status = "stopped"
	scene.BeforeUpdate()
	_, err := o.Update(&scene)
	return err
}

// DeleteScene 删除场景
func (s *SceneService) DeleteScene(id, userId int64) error {
	o := orm.NewOrm()
	scene := models.Scene{Id: id}
	tenantId, _ := models.GetUserTenantId(userId)
	if err := o.Read(&scene); err != nil || scene.Department == nil || scene.Department.Id != tenantId {
		return fmt.Errorf("scene not found or no permission")
	}

	// 先停止场景
	s.StopScene(id, tenantId)
	// 删除ekuiper中的同名场景规则
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	common.Ekuiper.DeleteRule(ctx, scene.Name)

	_, err := o.Delete(&scene)
	return err
}

// ExecuteScene 执行场景动作
func ExecuteScene(id, userId int64) error {
	o := orm.NewOrm()
	scene := models.Scene{Id: id}
	tenantId, _ := models.GetUserTenantId(userId)
	if err := o.Read(&scene); err != nil || scene.Department == nil || scene.Department.Id != tenantId {
		return fmt.Errorf("scene not found or no permission")
	}

	// 解析动作
	var actions []dtos.Action
	err := json.Unmarshal([]byte(scene.Action), &actions)
	if err != nil {
		return fmt.Errorf("解析动作失败: %v", err)
	}

	// 执行动作
	var results []string
	var writeLog models.WriteLog
	for _, action := range actions {
		// 调用设备控制接口
		seq, err := Processor.Deal(action.DeviceName, action.Code, action.Value, "场景控制", userId, tenantId)
		if err != nil {
			//更新SEQ失败状态
			writeLog.Seq = seq
			writeLog.Status = "FAIL"
			o.Update(&writeLog, "Status")
			return fmt.Errorf("设备控制失败: %v", err)
		}
		result := fmt.Sprintf("SEQ:[%s] ; 执行动作: 设备[%s] 属性[%s] 值[%s]", seq,
			action.DeviceName, action.Code, action.Value)
		results = append(results, result)
	}
	log.Println(results)
	return nil
}

// RestartScene 手动执行场景
func (s *SceneService) RestartScene(id, userId int64) error {
	err := ExecuteScene(id, userId)
	if err != nil {
		return err
	}
	return nil
}

func (s *SceneService) Jobs() map[string]cron.EntryID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobsCopy := make(map[string]cron.EntryID)
	for k, v := range s.jobs {
		jobsCopy[strconv.FormatInt(k, 10)] = v
	}
	return jobsCopy
}

func (s *SceneService) CronEntries() []cron.Entry {
	return s.cron.Entries()
}

// LoadScenesFromDatabase 项目启动时将robfig_cron加载至内存
func (s *SceneService) LoadScenesFromDatabase() error {
	o := orm.NewOrm()
	var scenes []models.Scene

	// 查询所有状态为运行中的场景
	_, err := o.QueryTable(new(models.Scene)).Filter("status", string(constants.RuleStart)).All(&scenes)
	if err != nil {
		return fmt.Errorf("查询运行中的场景失败: %v", err)
	}

	log.Printf("发现 %d 个运行中的场景，开始加载定时任务...", len(scenes))

	loadedCount := 0
	for _, scene := range scenes {
		if err := s.loadSceneToCron(scene, scene.UserId); err != nil {
			log.Printf("加载场景 %d 失败: %v", scene.Id, err)
			continue
		}
		loadedCount++
	}

	log.Printf("定时任务加载完成，共加载 %d 个场景", loadedCount)

	//启动设备离线检测
	_, err = s.cron.AddFunc("@every 10m", func() {
		err = s.DeviceStatusCron()
	})
	if err != nil {
		return fmt.Errorf("规则设备离线检测启动失败: %v", err)
	}
	_, err = s.cron.AddFunc("@every 1h", func() {
		err = s.AllDeviceStatusCron()
	})
	if err != nil {
		return fmt.Errorf("全局设备离线检测启动失败: %v", err)
	}
	return nil
}

// loadSceneToCron 将定时场景加载到cron中，触发场景加载ekuiper
func (s *SceneService) loadSceneToCron(scene models.Scene, userId int64) error {
	// 解析条件，查找定时条件
	var conditions []dtos.Condition
	err := json.Unmarshal([]byte(scene.Condition), &conditions)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	// 查找定时条件
	for _, condition := range conditions {
		if condition.ConditionType == "timer" {
			if cronExpr, ok := condition.Option["cron_expression"]; ok {
				// 添加定时任务
				entryID, err := s.cron.AddFunc(cronExpr, func() {
					log.Printf("定时任务触发: 场景ID=%d, 名称=%s", scene.Id, scene.Name)
					ExecuteScene(scene.Id, userId)
				})
				if err != nil {
					return fmt.Errorf("添加定时任务失败: %v", err)
				}

				s.mu.Lock()
				s.jobs[scene.Id] = entryID
				s.mu.Unlock()

				return nil
			}
		} else if condition.ConditionType == "notify" {
			// 设备触发 启动ekuiper规则
			go func(name string) {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()
				if err := common.Ekuiper.StartRule(ctx, name); err != nil {
					// 这里可以选择打印日志，不影响主流程
					log.Printf("StartRule 异步失败: %v", err)
				}
			}(scene.Name)
			return nil
		}
	}
	return fmt.Errorf("场景 %d 没有找到有效的定时条件", scene.Id)
}

func BuildEkuiperRule(ctx context.Context, params dtos.SceneUpdateRequest, sceneName string) error {
	var req dtos.RuleUpdateRequest
	var deviceArray []string

	req.Name = sceneName
	deviceIDs := append(deviceArray, params.Condition[0].Option["device_id"])
	productId, _ := strconv.ParseInt(params.Condition[0].Option["product_id"], 10, 64)
	req.SubRule = make([]models.SubRule, 1)
	req.SubRule[0].ProductId = productId
	req.SubRule[0].DeviceId = deviceIDs
	req.SubRule[0].Trigger = params.Condition[0].Option["trigger"]
	req.SubRule[0].Option = make(map[string]string)
	req.SubRule[0].Option["code"] = params.Condition[0].Option["code"]
	req.SubRule[0].Option["name"] = params.Condition[0].Option["name"]
	req.SubRule[0].Option["value_type"] = params.Condition[0].Option["value_type"]
	req.SubRule[0].Option["value_cycle"] = params.Condition[0].Option["value_cycle"]
	req.SubRule[0].Option["decide_condition"] = params.Condition[0].Option["decide_condition"]

	var sql string

	o := orm.NewOrm()
	switch req.SubRule[0].Trigger {
	case "设备数据触发":
		code := req.SubRule[0].Option["code"]
		productId := req.SubRule[0].ProductId
		var property models.Properties
		err := o.QueryTable(new(models.Properties)).Filter("code", code).Filter("product_id", productId).One(&property)
		if err != nil {
			return fmt.Errorf(err.Error())
		}
		var specs map[string]string
		err = json.Unmarshal([]byte(property.TypeSpec), &specs)
		if err != nil {
			return fmt.Errorf(err.Error())
		}
		typeStyle := specs["type"] // int float text ..
		sql = req.BuildMultiDeviceDataSql(deviceIDs, typeStyle)
	case "设备事件触发":
		sql = req.BuildMultiDeviceEventSql(deviceIDs)
	case "设备状态触发":
		sql = req.BuildMultiDeviceStatusSql(deviceIDs)
	}

	actions := common.GetRuleAlertEkuiperActions(common.CallBackUrl + "/api/ekuiper/callback2")

	if err := common.Ekuiper.RuleExist(ctx, req.Name); err == nil {
		err = common.Ekuiper.UpdateRule(ctx, actions, req.Name, sql)
		if err != nil {
			return fmt.Errorf(err.Error())
		}
	} else {
		err = common.Ekuiper.CreateRule(ctx, actions, req.Name, sql)
		if err != nil {
			return fmt.Errorf(err.Error())
		}
	}

	return nil
}
func (s *SceneService) DeviceStatusCron() error {
	// 查询所有正在运行的规则
	o := orm.NewOrm()
	var rules []models.AlertRule

	_, err := o.QueryTable(new(models.AlertRule)).Filter("status", string(constants.RuleStart)).All(&rules)
	if err != nil {
		return fmt.Errorf("查询运行中的规则失败: %v", err)
	}

	deviceMap := make(map[string]struct{})
	// 查找定时条件
	for _, rule := range rules {
		var subRule []models.SubRule
		if err := json.Unmarshal([]byte(rule.SubRule), &subRule); err != nil {
			continue
		}
		if subRule[0].Trigger != string(constants.DeviceStatusTrigger) {
			continue
		}
		var deviceIds []string
		if err := json.Unmarshal([]byte(rule.DeviceId), &deviceIds); err != nil {
			continue
		}

		// 遍历 deviceId 数组，将每个 ID 添加到 map 中去重
		for _, deviceId := range deviceIds {
			deviceMap[deviceId] = struct{}{}
		}
	}
	// 当前时间戳
	currentTime := time.Now().Unix()

	// 遍历 deviceMap 查询最新数据
	for deviceId, _ := range deviceMap {
		fmt.Printf("处理设备: %s\n", deviceId)
		startTime := time.Unix(currentTime-10*60, 0).Format("2006-01-02 15:04:05")
		endTime := time.Unix(currentTime, 0).Format("2006-01-02 15:04:05")

		tags, err := iotp.GetDeviceTags(deviceId)
		if err != nil {
			log.Printf("查询设备 %s 失败: %v", deviceId, err)
			continue
		}

		query := models.HistoryObject{
			Count:     1,
			EndTime:   endTime,
			StartTime: startTime,
			IDs:       tags,
		}

		// 查询最近数据
		data, err := iotp.HistoryQuery(query)
		if err != nil {
			log.Printf("查询设备 %s 失败: %v", deviceId, err)
			continue
		}

		if len(data) == 0 {
			log.Printf("设备 %s 在过去 %d 分钟无数据，触发告警", deviceId, 10)
			// 发送流数据告警
			err := Processor.SendOffline(deviceId)
			if err != nil {
				log.Printf("发送设备 %s 失败: %v", deviceId, err)
			}
		}
	}
	utils.DebugLog("设备离线检测执行完成，共检测 %d 个规则\n", len(deviceMap))
	return nil
}

// 全局设备离线检测
func (s *SceneService) AllDeviceStatusCron() error {
	// 查询所有在线设备
	devices, err := tagService.ListDevicesByTag("status", "1", nil)
	if err != nil {
		return fmt.Errorf(err.Error())
	}

	// 当前时间戳
	currentTime := time.Now().Unix()

	// 查询设备上次在线时间
	for _, device := range devices {
		lastOnline := deviceStatusCache[device]
		// 如果最后在线时间距离现在超过1小时，则标记为离线
		if currentTime-lastOnline > 3600 { // 3600秒 = 1小时
			// 更新设备状态为离线(假设"0"表示离线，"1"表示在线)
			if err := tagService.AddTag(device, "status", "0"); err != nil {
				log.Printf("更新设备 %s 状态失败: %v", device, err)
			} else {
				log.Printf("设备 %s 已标记为离线", device)
			}
		}
	}
	return nil
}

func ExecCallBack(req map[string]interface{}) error {
	o := orm.NewOrm()

	var scene models.Scene
	sceneName := req["rule_id"].(string)
	scene.Name = sceneName
	if err := o.Read(&scene, "Name"); err != nil {
		return fmt.Errorf(err.Error())
	}
	if err := ExecuteScene(scene.Id, scene.UserId); err != nil {
		return fmt.Errorf(err.Error())
	}
	return nil
}
