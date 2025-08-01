package services

import (
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"github.com/robfig/cron/v3"
	"iotServer/models"
	"iotServer/models/constants"
	"iotServer/models/dtos"
	"log"
	"strconv"
	"sync"
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
func (s *SceneService) StartScene(id int64) error {
	o := orm.NewOrm()
	scene := models.Scene{Id: id}

	if err := o.Read(&scene); err != nil {
		return fmt.Errorf("场景不存在: %v", err)
	}

	// 检查是否已经在运行
	if scene.Status == string(constants.RuleStart) {
		return fmt.Errorf("场景已在运行中")
	}

	// 解析条件，查找定时条件
	if err := s.loadSceneToCron(scene); err != nil {
		return fmt.Errorf("添加场景失败：" + err.Error())
	}
	// 更新状态
	scene.Status = "running"
	scene.BeforeUpdate()
	_, err := o.Update(&scene)
	return err
}

// StopScene 停止场景
func (s *SceneService) StopScene(id int64) error {
	o := orm.NewOrm()
	scene := models.Scene{Id: id}

	if err := o.Read(&scene); err != nil {
		return fmt.Errorf("场景不存在: %v", err)
	}

	// 移除定时任务
	if entryID, exists := s.jobs[id]; exists {
		s.cron.Remove(entryID)
		delete(s.jobs, id) // 清理映射，确保场景不再关联任务
	}

	// 更新状态
	scene.Status = "stopped"
	scene.BeforeUpdate()
	_, err := o.Update(&scene)
	return err
}

// DeleteScene 删除场景
func (s *SceneService) DeleteScene(id int64) error {
	// 先停止场景
	s.StopScene(id)

	o := orm.NewOrm()
	scene := models.Scene{Id: id}
	_, err := o.Delete(&scene)
	return err
}

// executeScene 执行场景动作
func executeScene(id int64) error {
	o := orm.NewOrm()
	scene := models.Scene{Id: id}

	if err := o.Read(&scene); err != nil {
		return fmt.Errorf("场景不存在: %v", err)
	}

	// 解析动作
	var actions []dtos.Action
	err := json.Unmarshal([]byte(scene.Action), &actions)
	if err != nil {
		return fmt.Errorf("解析动作失败: %v", err)
	}

	// 执行动作
	var results []string
	for _, action := range actions {
		// 调用设备控制接口
		seq, _ := Processor.Deal(action.DeviceName, action.Code, action.Value, "定时任务", 0)
		result := fmt.Sprintf("SEQ:[%s] ; 执行动作: 设备[%s] 属性[%s] 值[%s]", seq,
			action.DeviceName, action.Code, action.Value)
		results = append(results, result)
	}
	log.Println(results)
	return nil
}

// RestartScene 手动执行场景
func (s *SceneService) RestartScene(id int64) error {
	err := executeScene(id)
	if err != nil {
		return fmt.Errorf("执行失败：%v" + err.Error())
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
		if err := s.loadSceneToCron(scene); err != nil {
			log.Printf("加载场景 %d 失败: %v", scene.Id, err)
			continue
		}
		loadedCount++
	}

	log.Printf("定时任务加载完成，共加载 %d 个场景", loadedCount)
	return nil
}

// loadSceneToCron 将单个场景加载到cron中
func (s *SceneService) loadSceneToCron(scene models.Scene) error {
	// 解析条件，查找定时条件
	var conditions []dtos.Condition
	err := json.Unmarshal([]byte(scene.Condition), &conditions)
	if err != nil {
		return fmt.Errorf("解析条件失败: %v", err)
	}

	// 查找定时条件
	for _, condition := range conditions {
		if condition.ConditionType == "timer" {
			if cronExpr, ok := condition.Option["cron_expression"]; ok {
				// 添加定时任务
				entryID, err := s.cron.AddFunc(cronExpr, func() {
					log.Printf("定时任务触发: 场景ID=%d, 名称=%s", scene.Id, scene.Name)
					executeScene(scene.Id)
				})
				if err != nil {
					return fmt.Errorf("添加定时任务失败: %v", err)
				}

				s.mu.Lock()
				s.jobs[scene.Id] = entryID
				s.mu.Unlock()

				return nil
			}
		}
		// TODO 设备触发
	}

	return fmt.Errorf("场景 %d 没有找到有效的定时条件", scene.Id)
}
