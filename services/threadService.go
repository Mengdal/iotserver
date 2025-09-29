package services

import (
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"iotServer/common"
	"iotServer/iotp"
	"iotServer/models"
	"iotServer/models/constants"
	"iotServer/utils"
	"log"
	"strings"
	"sync"
	"time"
)

// 定义全局的工作池参数
var (
	workerPoolSize    = 50                           // 工作协程数量
	jobQueue          = make(chan Job, 10000)        // 任务队列
	workerOnce        sync.Once                      // 确保工作池只初始化一次
	forwardWorkerPool = 50                           // 专门用于转发的工作协程数量
	forwardJobQueue   = make(chan ForwardJob, 10000) // 转发任务队列
	forwardWorkerOnce sync.Once                      // 确保转发工作池只初始化一次
	// 设备状态缓存相关
	deviceStatusCache = make(map[string]int64) // 设备ID -> 最后更新时间戳
	cacheMutex        sync.RWMutex             // 保护缓存的读写锁
	cacheTTL          = 24 * time.Hour         // 缓存有效期
	maxCacheSize      = 10000                  // 缓存最大条目数
	// 事件处理相关
	eventWorkerOnce sync.Once
	eventJobQueue   = make(chan EventJob, 1000) // 事件处理任务队列
	eventWorkerSize = 20                        // 事件处理工作协程数量
	eventCache      = sync.Map{}
)

// 定义消息处理任务结构
type Job struct {
	Topic   string
	Payload string
	Handler func(string, string) error
}

// 定义转发任务结构
type ForwardJob struct {
	Topic   string
	Payload []byte
	Client  common.MqttConnector
}

// EventJob 事件处理任务结构
type EventJob struct {
	Topic string
	Data  map[string]interface{}
}

// 初始化消息处理工作池
func initWorkerPool() {
	for i := 0; i < workerPoolSize; i++ {
		go worker(jobQueue)
	}
	log.Printf("已启动 %d 个工作协程处理MQTT消息", workerPoolSize)
}

// 初始化转发工作池
func initForwardWorkerPool(client common.MqttConnector) {
	for i := 0; i < forwardWorkerPool; i++ {
		go forwardWorker(forwardJobQueue, client)
	}
	log.Printf("已启动 %d 个工作协程处理转发任务", forwardWorkerPool)
}

// 初始化事件处理工作池
func initEventWorkerPool() {
	for i := 0; i < eventWorkerSize; i++ {
		go eventWorker(eventJobQueue)
	}
	log.Printf("已启动 %d 个事件处理工作协程", eventWorkerSize)
}

// 消息处理工作协程
func worker(jobQueue <-chan Job) {
	for job := range jobQueue {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Worker处理任务时发生panic: %v", r)
				}
			}()

			start := time.Now()
			log.Printf("开始处理消息: %s", job.Topic)

			if err := job.Handler(job.Topic, job.Payload); err != nil {
				log.Printf("消息处理失败: %s -- %v", job.Topic, err)
			} else {
				log.Printf("消息处理完成: %s (耗时: %v)", job.Topic, time.Since(start))
			}
		}()
	}
}

// 转发工作协程
func forwardWorker(jobQueue <-chan ForwardJob, client common.MqttConnector) {
	for job := range jobQueue {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("ForwardWorker处理任务时发生panic: %v", r)
				}
			}()

			start := time.Now()
			log.Printf("开始转发消息: %s", job.Topic)

			if err := client.Publish(job.Topic, 0, job.Payload); err != nil {
				log.Printf("消息转发失败: %s -- %v", job.Topic, err)
			} else {
				log.Printf("消息转发完成: %s (耗时: %v)", job.Topic, time.Since(start))
			}
		}()
	}
}

// UpdateDeviceStatus 更新设备状态，避免频繁的重复更新
func UpdateDeviceStatus(deviceId string, tagService iotp.TagService) bool {
	currentTime := time.Now().Unix()

	// 检查缓存，避免重复处理
	cacheMutex.RLock()
	lastUpdateTime, exists := deviceStatusCache[deviceId]
	cacheMutex.RUnlock()

	// 如果缓存存在且未过期，则跳过处理
	if exists && time.Since(time.Unix(lastUpdateTime, 0)) < cacheTTL {
		return false // 跳过更新
	}

	// 缓存未命中或已过期，检查实际设备状态
	value, _ := tagService.GetTagValue(deviceId, "status")
	if value == "1" {
		// 设备已经是在线状态，更新缓存
		updateCache(deviceId, currentTime)
		return false // 无需更新
	} else {
		// 更新设备状态为在线
		tagService.AddTag(deviceId, "status", "1")
		tagService.AddTag(deviceId, "lastOnline", utils.InterfaceToString(currentTime))

		// 更新缓存
		updateCache(deviceId, currentTime)
		return true // 已更新
	}
}

// updateCache 更新缓存，内部方法
func updateCache(deviceId string, timestamp int64) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// 检查并控制缓存大小
	if len(deviceStatusCache) >= maxCacheSize {
		// 如果缓存已满，清理过期条目
		cleanupExpiredCacheEntries()
		// 如果清理后仍然满了，跳过缓存
		if len(deviceStatusCache) >= maxCacheSize {
			return
		}
	}

	deviceStatusCache[deviceId] = timestamp
}

// cleanupExpiredCacheEntries 清理过期缓存条目
func cleanupExpiredCacheEntries() {
	expiredKeys := make([]string, 0)

	// 找出过期的条目
	for key, updateTime := range deviceStatusCache {
		if time.Since(time.Unix(updateTime, 0)) >= cacheTTL {
			expiredKeys = append(expiredKeys, key)
		}
	}

	// 删除过期条目
	for _, key := range expiredKeys {
		delete(deviceStatusCache, key)
	}

	log.Printf("清理了 %d 个过期设备状态缓存条目", len(expiredKeys))
}

// 事件处理工作协程
func eventWorker(jobQueue <-chan EventJob) {
	// 每个worker拥有自己的数据库连接
	o := orm.NewOrm()

	for job := range jobQueue {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("事件处理工作协程发生panic: %v", r)
				}
			}()

			start := time.Now()
			if err := processAlertEventJob(job.Topic, job.Data, o); err != nil {
				log.Printf("事件处理失败: %s -- %v", job.Topic, err)
			} else {
				log.Printf("事件处理完成: %s (耗时: %v)", job.Topic, time.Since(start))
			}
		}()
	}
}

// processAlertEventJob 处理单个事件任务
func processAlertEventJob(topic string, alertData map[string]interface{}, o orm.Ormer) error {
	// tag只有从网关发出才有，其他情况从实时主题转发过来，查询是否为事件后通知
	point := alertData["tag"]
	out := make(map[string]interface{})

	if point == nil {
		if data, ok := alertData["data"].(map[string]interface{}); ok {
			for key, value := range data {
				// key 是键，value 是对应的值Map(time+value)
				// 检测是否是事件类
				lastIndex := strings.LastIndex(key, "__")
				if lastIndex == -1 {
					continue
				}

				// 检测是否值变化
				currValue := utils.InterfaceToString(value.(map[string]interface{})["value"])
				cacheKey := fmt.Sprintf("%s.%s", alertData["dn"], key)

				lastValue, _ := eventCache.Load(cacheKey)
				if lastValue == currValue {
					continue // 对比上一次值没变化，直接跳过
				}
				// 更新缓存
				eventCache.Store(cacheKey, currValue)

				code := key[:lastIndex]
				params := key[lastIndex+1:]

				var event models.Events
				if err := o.QueryTable(new(models.Events)).Filter("code", code).One(&event); err != nil {
					continue
				}

				// 解析outputParams
				var outputParams models.OutPutParams
				if err := json.Unmarshal([]byte(event.OutputParams), &outputParams); err != nil {
					log.Printf("解析OutputParams失败: %v", err)
					continue
				}

				out["alert_level"] = constants.GetEventTypeDescription(event.EventType)
				out["code"] = event.Code
				out["dn"] = alertData["dn"]
				out["start_at"] = time.Now().Unix()
				out["name"] = event.Code
				out["trigger"] = "设备事件触发"
				out["type"] = "值变化"
				out["rule_name"] = event.Name

				// 检查params是否匹配outputParams中的任意一个code
				for _, param := range outputParams {
					if param.Code == params {
						var typeSpec models.TypeSpec
						if err := json.Unmarshal([]byte(param.TypeSpec), &typeSpec); err != nil {
							log.Printf("解析typeSpec失败: %v", err)
							continue
						}

						if _, ok := out["value"]; !ok {
							out["value"] = make([]map[string]interface{}, 0)
						}
						if typeSpec.Type == "enum" {
							var specs map[string]interface{}
							if err := json.Unmarshal(typeSpec.Specs, &specs); err != nil {
								log.Printf("解析specs失败: %v", err)
								continue
							}
							out["value"] = append(out["value"].([]map[string]interface{}), map[string]interface{}{param.Name: specs[currValue]})
						} else {
							out["value"] = append(out["value"].([]map[string]interface{}), map[string]interface{}{param.Name: currValue})
						}
					}
				}
			}
		}
	} else {
		split := strings.Split(point.(string), ".")
		dn := split[0]
		tag := split[1]
		eventTime := alertData["timestamp"]
		event := alertData["event"]
		value := alertData["value"]
		typeName := alertData["type"]
		if typeName == "AlarmTrigger" {
			typeName = "网关事件触发"
		} else {
			typeName = "网关事件解除"
		}
		typeR := alertData["status"]
		if typeR == "Error" && value == "0" {
			typeR = "质量不为Good"
		} else {
			typeR = "点值超出范围"
		}

		timestamp, _ := iotp.GetTimestamp(eventTime.(string))
		out = map[string]interface{}{
			"alert_level": "告警",
			"code":        tag,
			"dn":          dn,
			"start_at":    timestamp,
			"name":        tag,
			"rule_name":   event,
			"trigger":     typeName,
			"type":        typeR,
			"value":       value,
		}
	}

	if len(out) == 0 {
		return nil
	}

	newPayload, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("JSON序列化失败: %v", err)
	}

	// 构建告警记录，网关事件直接存入Alert_List
	alert := &models.AlertList{
		AlertRule:   nil,
		TriggerTime: time.Now().UnixMilli(),
		IsSend:      false,
		Status:      string(constants.Untreated),
		AlertResult: string(newPayload),
	}

	// 保存到数据库
	if err = alert.BeforeInsert(); err != nil {
		return fmt.Errorf("插入失败: %v", err)
	}
	if _, err = o.Insert(alert); err != nil {
		return fmt.Errorf("保存告警记录失败: %v", err)
	}

	return nil
}
