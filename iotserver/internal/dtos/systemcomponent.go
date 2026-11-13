package dtos

// SystemComponentDTO 内置系统组件（Mosquitto / eKuiper / IotEdgeDB 等）的展示信息。
type SystemComponentDTO struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Runtime     string `json:"runtime"`
	Essential   bool   `json:"essential"`
	// Endpoint 组件实际监听地址（来自平台配置，仅用于展示与探活）
	Endpoint string `json:"endpoint"`
	// Status running / stopped / unknown
	Status string `json:"status"`
	// LogConfigured 是否配置了可通过平台查看的日志文件。
	LogConfigured bool `json:"logConfigured"`
	// LogPath 日志文件路径（仅平台管理员可见的运维信息）。
	LogPath string `json:"logPath"`
	Updated int64  `json:"updated"`
}

// SystemComponentLogsDTO 系统组件日志读取结果。
type SystemComponentLogsDTO struct {
	ComponentId string   `json:"componentId"`
	Path        string   `json:"path"`
	Exists      bool     `json:"exists"`
	Size        int64    `json:"size"`
	Modified    int64    `json:"modified"`
	Lines       []string `json:"lines"`
}
