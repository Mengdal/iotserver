package models

// SystemConfig 系统配置（键值对）
type SystemConfig struct {
	Timestamps  `gorm:"embedded"`
	Id          string `gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	ConfigKey   string `gorm:"column:config_key;uniqueIndex;type:string;size:255;comment:配置键"`
	ConfigValue string `gorm:"column:config_value;type:text;comment:配置值（图片以 data URL 存储）"`
	Name        string `gorm:"column:name;type:string;size:255;comment:配置名称"`
	GroupName   string `gorm:"column:group_name;type:string;size:128;comment:分组"`
	Type        string `gorm:"column:type;type:string;size:32;default:string;comment:值类型"`
	Sort        int    `gorm:"column:sort;type:int;default:0;comment:排序"`
	Remark      string `gorm:"column:remark;type:string;size:500;comment:备注"`
}

func (table *SystemConfig) Get() interface{} {
	return *table
}

func (table *SystemConfig) TableName() string {
	return "sys_config"
}
