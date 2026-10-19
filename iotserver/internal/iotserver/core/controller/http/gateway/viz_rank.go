//
// Copyright (c) 2024 IOTServer
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package gateway

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	resourceContainer "iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
	"iotserver/internal/pkg/tenant"
)

// 首页看板「设备消息量排名」：按设备统计时间窗内的消息数，取前 N 名。
const (
	vizRankDefaultLimit = 5
	vizRankMaxLimit     = 20
	vizRankTimeout      = 20 * time.Second
)

type vizRankItem struct {
	DeviceId   string `json:"deviceId"`
	DeviceName string `json:"deviceName"`
	// Count 时间窗内的消息数（属性 + 事件 + 服务）
	Count int `json:"count"`
}

type vizRankResponse struct {
	Items []vizRankItem `json:"items"`
	// Total 时间窗内全部设备的消息总数（前端算占比用）
	Total     int    `json:"total"`
	TimeRange string `json:"timeRange"`
}

// VizDeviceMessageRank 设备消息量排行
//
// GET /api/v1/viz/device-message-rank?timeRange=7d&limit=5
//
// 数据层一次 GROUP BY 拿到每个设备的消息数（属性 + 事件 + 服务），这里只做排序与设备名补全；
// 租户视图按 tenant_id 过滤，平台视图统计全部租户。
func (ctl *controller) VizDeviceMessageRank(c *gin.Context) {
	timeRange := c.DefaultQuery("timeRange", "7d")
	limit := vizRankDefaultLimit
	if v, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(vizRankDefaultLimit))); err == nil && v > 0 && v <= vizRankMaxLimit {
		limit = v
	}
	start, end := vizTimeWindow(nil, timeRange)

	resp := vizRankResponse{Items: []vizRankItem{}, TimeRange: timeRange}
	counter, ok := resourceContainer.DataDBClientFrom(ctl.dic.Get).(interfaces.DeviceMsgCounter)
	if !ok {
		// 数据层不支持按设备分组统计：返回空列表，前端展示空态
		httphelper.ResultSuccess(resp, c.Writer, ctl.lc)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), vizRankTimeout)
	defer cancel()

	tenantID := ""
	if !tenant.IsPlatformScope(ctx) {
		tenantID = tenant.FromContext(ctx).TenantID
	}
	counts, err := counter.CountMsgGroupByDevice(tenantID, start, end)
	if err != nil {
		ctl.lc.Errorf("viz rank: 统计设备消息量失败: %v", err)
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError,
			errort.NewMessageErr("统计设备消息量失败：{{.p1}}", map[string]interface{}{"p1": err.Error()})), c.Writer, ctl.lc)
		return
	}
	if len(counts) == 0 {
		httphelper.ResultSuccess(resp, c.Writer, ctl.lc)
		return
	}

	type deviceCount struct {
		id string
		n  int
	}
	all := make([]deviceCount, 0, len(counts))
	total := 0
	for id, n := range counts {
		total += n
		if n > 0 {
			all = append(all, deviceCount{id: id, n: n})
		}
	}
	// 消息数倒序；数量相同按设备 ID 稳定排序，避免每次顺序抖动
	sort.Slice(all, func(i, j int) bool {
		if all[i].n != all[j].n {
			return all[i].n > all[j].n
		}
		return all[i].id < all[j].id
	})
	if len(all) > limit {
		all = all[:limit]
	}

	items := make([]vizRankItem, 0, len(all))
	for _, d := range all {
		name := d.id
		// 设备可能已删除（数据层里还留着历史数据）：取不到名字就回落到设备 ID
		if device, dErr := ctl.getDeviceApp().DeviceModelById(ctx, d.id); dErr == nil && device.Name != "" {
			name = device.Name
		}
		items = append(items, vizRankItem{DeviceId: d.id, DeviceName: name, Count: d.n})
	}
	resp.Items = items
	resp.Total = total
	httphelper.ResultSuccess(resp, c.Writer, ctl.lc)
}
