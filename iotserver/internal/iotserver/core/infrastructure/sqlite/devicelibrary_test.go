package sqlite

import (
	"testing"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
)

// embedded 驱动市场只保留 native/builtin，Docker 驱动必须被过滤掉。
func TestDeviceLibrariesSearchExcludeDockerRuntime(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	list, total, err := c.DeviceLibrariesSearch(0, -1, dtos.DeviceLibrarySearchQueryRequest{
		IsInternal:     "true",
		ExcludeRuntime: models.PluginRuntimeDocker,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if total == 0 || len(list) == 0 {
		t.Fatal("内置系统组件不应被过滤")
	}
	for _, item := range list {
		if item.Runtime == models.PluginRuntimeDocker {
			t.Fatalf("Docker 驱动 %s 不应出现在嵌入式市场", item.Name)
		}
	}
}
