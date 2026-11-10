package dtos

// PluginPackageDTO 原生插件包。
type PluginPackageDTO struct {
	Id          string   `json:"id"`
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Runtime     string   `json:"runtime"`
	Arch        string   `json:"arch"`
	Protocols   []string `json:"protocols"`
	Essential   bool     `json:"essential"`
	Source      string   `json:"source"`
	PackagePath string   `json:"packagePath"`
	PackageHash string   `json:"packageHash"`
	Updated     int64    `json:"updated"`
}

// PluginInstanceCreateRequest 从插件包创建实例。
type PluginInstanceCreateRequest struct {
	LibraryId string                 `json:"libraryId" binding:"required"`
	Name      string                 `json:"name"`
	Config    map[string]interface{} `json:"config"`
	AutoStart bool                   `json:"autoStart"`
}

// PluginInstanceDTO 原生插件实例。
type PluginInstanceDTO struct {
	Id           string                 `json:"id"`
	Name         string                 `json:"name"`
	LibraryId    string                 `json:"libraryId"`
	LibraryName  string                 `json:"libraryName"`
	Version      string                 `json:"version"`
	Runtime      string                 `json:"runtime"`
	Status       string                 `json:"status"`
	HealthStatus string                 `json:"healthStatus"`
	Pid          int                    `json:"pid"`
	BaseAddress  string                 `json:"baseAddress"`
	InstallPath  string                 `json:"installPath"`
	LastError    string                 `json:"lastError"`
	Config       map[string]interface{} `json:"config"`
	Updated      int64                  `json:"updated"`
}

// PluginLogsDTO 插件实例日志。
type PluginLogsDTO struct {
	Lines []string `json:"lines"`
}
