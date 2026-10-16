package gateway

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	resourceContainer "iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
)

// SystemBackupRequest 在线备份请求
type SystemBackupRequest struct {
	// Dir 导出目录；留空则落到数据库文件同级的 backup/<时间戳>/
	Dir string `json:"dir"`
	// Keep 保留最近多少份（按目录名倒序清理），0 表示不清理
	Keep int `json:"keep"`
}

// SystemBackup 在线导出一致快照（不停服）。
//
// 数据库文件被运行中的进程独占，直接拷文件会拿到 WAL 未合并的状态；
// 这里调用数据层的 DatabaseExporter——先 CHECKPOINT 再由引擎导出
// Parquet + schema.sql，导出结果可用 IMPORT DATABASE 恢复。
//
// 平台管理员专属（路由挂在 PlatformOnly 组），权限码复用 system:config:edit。
func (ctl *controller) SystemBackup(c *gin.Context) {
	lc := ctl.lc
	var req SystemBackupRequest
	// 允许不传 body（全部用默认值）
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}

	exporter, ok := resourceContainer.DataDBClientFrom(ctl.dic.Get).(interfaces.DatabaseExporter)
	if !ok {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError,
			fmt.Errorf("当前数据层不支持在线导出")), c.Writer, lc)
		return
	}

	dir := strings.TrimSpace(req.Dir)
	if dir == "" {
		// 默认落到数据库文件同级的 backup/<时间戳>/
		dbPath := ctl.cfg.GetDataDatabaseInfo()["Primary"].DataSource
		base := filepath.Dir(dbPath)
		dir = filepath.Join(base, "backup", time.Now().Format("20060102-150405"))
	}

	if err := exporter.ExportDatabase(c.Request.Context(), dir); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	lc.Infof("在线备份完成: %s", dir)

	removed := 0
	if req.Keep > 0 {
		removed = pruneBackups(filepath.Dir(dir), req.Keep, lc)
	}
	httphelper.ResultSuccess(map[string]interface{}{
		"dir":     dir,
		"removed": removed,
	}, c.Writer, lc)
}

// pruneBackups 只保留最近 keep 份备份（按目录名倒序，目录名为时间戳）。
// 仅清理 parent 下形如 <时间戳> 的子目录，避免误删无关目录。
func pruneBackups(parent string, keep int, lc interface{ Warnf(string, ...interface{}) }) int {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return 0
	}
	dirs := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// 目录名必须是 20060102-150405 形式才认为是备份
		if _, err := time.Parse("20060102-150405", e.Name()); err != nil {
			continue
		}
		dirs = append(dirs, e.Name())
	}
	if len(dirs) <= keep {
		return 0
	}
	// 名称即时间戳，字典序倒序 = 从新到旧
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	removed := 0
	for _, name := range dirs[keep:] {
		if err := os.RemoveAll(filepath.Join(parent, name)); err != nil {
			lc.Warnf("清理旧备份 %s 失败: %v", name, err)
			continue
		}
		removed++
	}
	return removed
}
