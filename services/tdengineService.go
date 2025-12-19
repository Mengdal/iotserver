package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
	beego "github.com/beego/beego/v2/server/web"
	_ "github.com/taosdata/driver-go/v3/taosRestful"
	"iotServer/models"
	"iotServer/utils"
	"strings"
	"sync"
	"time"
)

// 声明全局变量
var Label string = "productId BIGINT"
var SubLabel string = "productId"
var DBName string = "power"

type TDengineService struct {
	db *sql.DB
}
type MqttMessage struct {
	Dn         string                 `json:"dn"`         // 超级表名
	Desc       string                 `json:"desc"`       // 设备描述
	Properties map[string]interface{} `json:"properties"` // 列和值
	Time       int64                  `json:"time"`       // 时间戳
}
type Tag struct {
	Key   string
	Value interface{}
}

func NewTDengineService() (*TDengineService, error) {
	taosUri, _ := beego.AppConfig.String("tdEngine")
	db, err := sql.Open("taosRestful", taosUri)
	if err != nil {
		return nil, err
	}

	// 测试连接
	if err = db.Ping(); err != nil {
		return nil, err
	}

	return &TDengineService{db: db}, nil
}

type TDengineWriter struct {
	db            *sql.DB
	buffer        []MqttMessage // 改为统一缓冲
	mu            sync.Mutex
	flushInterval time.Duration
	maxBatchSize  int
}

// Writer
func (t *TDengineService) NewTDengineWriter(flushInterval time.Duration, maxBatch int) *TDengineWriter {
	w := &TDengineWriter{
		db:            t.db,
		buffer:        make([]MqttMessage, 0, maxBatch),
		flushInterval: flushInterval,
		maxBatchSize:  maxBatch,
	}
	go w.startFlushLoop()
	return w
}

// 创建数据库
func (t *TDengineService) CreateDatabase(dbName string) error {
	query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s KEEP 3650", dbName)
	_, err := t.db.Exec(query)
	return err
}

// 创建超级表
func (t *TDengineService) CreateStable(dbName, stableName, schema string, tags string) error {
	var query string
	if tags == "" {
		// 无标签的超级表
		query = fmt.Sprintf("CREATE STABLE IF NOT EXISTS %s.`%s` (%s) TAGS (`tag` BINARY(64))",
			dbName, stableName, schema)
	} else {
		// 有标签的超级表
		query = fmt.Sprintf("CREATE STABLE IF NOT EXISTS %s.`%s` (%s) TAGS (%s)",
			dbName, stableName, schema, tags)
	}
	_, err := t.db.Exec(query)
	return err
}

// 创建子表
func (t *TDengineService) CreateTable(dbName, stableName, tableName string, tags []Tag) error {
	if len(tags) == 0 {
		return fmt.Errorf("tags cannot be empty")
	}

	// 构建标签值
	tagValues := make([]string, 0, len(tags))
	for _, tag := range tags {
		switch v := tag.Value.(type) {
		case string:
			tagValues = append(tagValues, fmt.Sprintf("'%s'", v))
		default:
			tagValues = append(tagValues, fmt.Sprintf("%v", v))
		}
	}

	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s USING %s.%s TAGS (%s)",
		dbName, tableName, dbName, stableName, strings.Join(tagValues, ", "))
	logs.Info(query)
	_, err := t.db.Exec(query)
	return err
}

// 添加一条消息到缓冲

func (w *TDengineWriter) Add(msg MqttMessage) {
	w.mu.Lock()
	w.buffer = append(w.buffer, msg)
	needFlush := len(w.buffer) >= w.maxBatchSize
	w.mu.Unlock()

	if needFlush {
		// 在锁外执行 flush（避免死锁）
		w.flush()
	}
}

// 定时器自动落库
func (w *TDengineWriter) startFlushLoop() {
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	for range ticker.C {
		w.flush()
	}
}

func (w *TDengineWriter) flush() {
	// 先把要处理的数据取出来（通过交换 slice）
	w.mu.Lock()
	if len(w.buffer) == 0 {
		w.mu.Unlock()
		return
	}
	// 交换：old := w.buffer; w.buffer = make([]MqttMessage, 0, w.maxBatchSize)
	old := w.buffer
	w.buffer = make([]MqttMessage, 0, w.maxBatchSize)
	w.mu.Unlock()

	// 下面在锁外处理 old，不会阻塞 Add
	// 按表名分组
	tableMessages := make(map[string][]MqttMessage)
	validMsgCount := 0
	for _, msg := range old {
		// 检查超级表是否存在
		_, ok := GetDeviceCategoryKeyFromCache(msg.Dn)
		if !ok {
			fmt.Printf("超级表 [%s] 不存在，跳过插入\n", msg.Dn)
			continue
		}
		if len(msg.Properties) == 0 {
			continue
		}
		tableMessages[msg.Dn] = append(tableMessages[msg.Dn], msg)
		validMsgCount++
	}
	if validMsgCount == 0 {
		return
	}

	// 构建跨表批量插入SQL
	var sqlBuilder strings.Builder
	sqlBuilder.WriteString("INSERT INTO ")
	firstTable := true
	insertedCount := 0

	for tableName, tableMsgs := range tableMessages {
		if len(tableMsgs) == 0 {
			continue
		}
		if !firstTable {
			sqlBuilder.WriteString(" ")
		}
		firstTable = false

		var cols []string
		var values []string

		for _, msg := range tableMsgs {
			if len(msg.Properties) == 0 {
				continue
			}
			var valParts []string
			valParts = append(valParts, fmt.Sprintf("%d", msg.Time*1000)) // ts

			if len(cols) == 0 {
				cols = append(cols, "`ts`")
				for k := range msg.Properties {
					cols = append(cols, fmt.Sprintf("`%s`", k))
				}
			}

			for _, k := range cols[1:] {
				key := strings.Trim(k, "`")
				v, ok := msg.Properties[key]
				if !ok {
					valParts = append(valParts, "NULL")
					continue
				}
				switch val := v.(type) {
				case string:
					valSafe := strings.ReplaceAll(val, "'", "''")
					valParts = append(valParts, fmt.Sprintf("'%s'", valSafe))
				case nil:
					valParts = append(valParts, "NULL")
				default:
					valParts = append(valParts, fmt.Sprintf("%v", val))
				}
			}
			values = append(values, fmt.Sprintf("(%s)", strings.Join(valParts, ",")))
		}
		if len(values) == 0 {
			continue
		}
		fmt.Fprintf(&sqlBuilder, "%s.`%s` (%s) VALUES %s",
			DBName, tableName, strings.Join(cols, ","), strings.Join(values, ","))
		insertedCount += len(tableMsgs)
	}

	if insertedCount == 0 {
		return
	}

	sqlStr := sqlBuilder.String()
	if _, err := w.db.Exec(sqlStr); err != nil {
		fmt.Printf("批量插入失败: %v\nSQL: %s\n", err, sqlStr)
	} else {
		utils.DebugLog("批量插入成功: %d 条记录\n", insertedCount)
	}
}

// 查询数据
func (t *TDengineService) QueryData(query string, args ...interface{}) (*sql.Rows, error) {
	return t.db.Query(query, args...)
}

// 查询单行数据
func (t *TDengineService) QueryRow(query string, args ...interface{}) *sql.Row {
	return t.db.QueryRow(query, args...)
}

// 关闭连接
func (t *TDengineService) Close() error {
	if t.db != nil {
		return t.db.Close()
	}
	return nil
}

// 获取超级表现有列
func (t *TDengineService) getExistingColumns(dbName, stableName string) ([]string, error) {
	query := fmt.Sprintf("DESCRIBE %s.`%s`", dbName, stableName)
	rows, err := t.QueryData(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var field, fieldType, nullable, key, defaultValue, note, extra string
		if err := rows.Scan(&field, &fieldType, &nullable, &key, &defaultValue, &note, &extra); err != nil {
			return nil, err
		}
		columns = append(columns, field)
	}

	return columns, nil
}

// 检查列是否存在
func containsColumn(columns []string, columnName string) bool {
	for _, col := range columns {
		if col == columnName {
			return true
		}
	}
	return false
}

// UpdateSuperTableSchema 方法中添加超级表更新逻辑
func (t *TDengineService) UpdateSuperTableSchema(dbName, stableName string, newProperties []*models.Properties) error {
	// 获取现有列名
	t.CreateDatabase(dbName)
	existingColumns, err := t.getExistingColumns(dbName, stableName)
	if err != nil {
		// 如果是表不存在的错误，则创建超级表
		if strings.Contains(err.Error(), "not exist") || strings.Contains(err.Error(), "doesn't exist") {
			schema := GenerateSchemaFromProperties(newProperties)
			// 使用全局标签变量
			err = t.CreateStable(dbName, stableName, schema, Label)
			if err != nil {
				return fmt.Errorf("创建超级表失败: %v", err)
			}
			return nil // 创建完成后直接返回
		}
		// 其他错误则返回
		return err
	}

	// 找出新增的属性
	for _, newProp := range newProperties {
		if !containsColumn(existingColumns, newProp.Code) {
			fieldType := parseFieldType(newProp.TypeSpec)
			err = t.AlterStableAddColumnIfNotExists(dbName, stableName, newProp.Code, fieldType)
			if err != nil {
				return fmt.Errorf("添加列 %s 失败: %v", newProp.Code, err)
			}
		}

	}

	return nil
}

// 添加列
func (t *TDengineService) AlterStableAddColumnIfNotExists(dbName, stableName, columnName, columnType string) error {
	query := fmt.Sprintf("ALTER STABLE %s.`%s` ADD COLUMN `%s` %s",
		dbName, stableName, columnName, columnType)
	_, err := t.db.Exec(query)
	return err
}

// 删除列
func (t *TDengineService) DeleteStableAddColumnIfNotExists(dbName, stableName, columnName, columnType string) error {
	query := fmt.Sprintf("ALTER STABLE %s.`%s` DROP COLUMN `%s` %s",
		dbName, stableName, columnName, columnType)
	_, err := t.db.Exec(query)
	return err
}

// 属性 -> schema
func GenerateSchemaFromProperties(properties []*models.Properties) string {
	var schemaParts []string
	schemaParts = append(schemaParts, "`ts` TIMESTAMP") // 时间戳字段，用反引号包裹

	for _, prop := range properties {
		fieldType := parseFieldType(prop.TypeSpec)
		// 用反引号包裹字段名，避免关键字冲突
		schemaParts = append(schemaParts, fmt.Sprintf("`%s` %s", prop.Code, fieldType))
	}

	return strings.Join(schemaParts, ", ")
}

func parseFieldType(typeSpec string) string {
	// 解析 TypeSpec JSON 来确定 TDengine 字段类型
	var spec models.TypeSpec
	if err := json.Unmarshal([]byte(typeSpec), &spec); err != nil {
		return "BINARY(255)" // 默认类型
	}

	switch spec.Type {
	case "int", "integer":
		return "INT"
	case "float", "double":
		return "FLOAT"
	case "bool", "boolean":
		return "BOOL"
	case "string", "text":
		return "BINARY(255)"
	default:
		return "BINARY(255)"
	}
}

// LoadAllDeviceCategoryKeys 查询所有设备的 categoryKey 并存入缓存
func LoadAllDeviceCategoryKeys() error {
	o := orm.NewOrm()

	// 查询所有设备的 name 和 categoryKey 字段
	var devices []models.Device
	_, err := o.QueryTable(new(models.Device)).All(&devices)
	if err != nil {
		return err
	}

	// 将结果存入缓存
	for _, device := range devices {
		StableCache.Store(device.Name, device.CategoryKey)
	}

	return nil
}

// GetDeviceCategoryKeyFromCache 从缓存中获取设备的 categoryKey
func GetDeviceCategoryKeyFromCache(deviceName string) (string, bool) {
	if value, ok := StableCache.Load(deviceName); ok {
		if categoryKey, ok := value.(string); ok {
			return categoryKey, true
		}
	}
	return "", false
}

// DemoConnect 连接示例
func DemoConnect(category string, product models.Product, properties []*models.Properties, events []*models.Events, actions []*models.Actions) {
	taosUri := "root:taosdata@http(localhost:6041)/"
	db, err := sql.Open("taosRestful", taosUri)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS power")
	if err != nil {
		fmt.Println("failed to create database:", err)
		return
	}

	_, err = db.Exec("CREATE STABLE IF NOT EXISTS power.meters (ts TIMESTAMP, current FLOAT, voltage INT, phase FLOAT) TAGS (location BINARY(64), groupId INT)")
	if err != nil {
		fmt.Println("failed to create stable:", err)
		return
	}

	fmt.Println("✅ TDengine RESTful 连接成功并建表完成")
}
