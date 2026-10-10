package interfaces

import (
	"iotserver/internal/dtos"
	"iotserver/internal/models"
)

// SceneDB 高级能力-场景联动仓储（含场景日志）
type SceneDB interface {
	AddScene(s models.Scene) (models.Scene, error)
	SceneById(id string) (models.Scene, error)
	SceneSearch(offset, limit int, req dtos.SceneSearchQueryRequest) ([]models.Scene, uint32, error)
	UpdateScene(s models.Scene) error
	DeleteScene(id string) error
	UpdateSceneStatus(id, status string) error
	// SceneRunningTimers 运行中的定时场景（服务重启后恢复调度）
	SceneRunningTimers() ([]models.Scene, error)

	AddSceneLog(log models.SceneLog) error
	SceneLogSearch(offset, limit int, req dtos.SceneLogSearchQueryRequest) ([]models.SceneLog, uint32, error)
	DeleteSceneLogsBySceneId(sceneId string) error
}
