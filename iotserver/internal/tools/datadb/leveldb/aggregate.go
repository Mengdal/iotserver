package leveldb

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"

	"iotserver/internal/dtos"
	"iotserver/internal/tools/datadb/aggregate"
)

// 本文件是 LevelDB 侧的聚合查询实现：
// LevelDB 是 KV 存储，无法在库内聚合，因此先扫出指定范围内的全部原始行，
// 再用 aggregate.Bucketize 在内存里按时间分桶，最后对「桶」分页。
//
// 注意：聚合查询会遍历整个时间范围（与既有 IsAll 查询同一量级），
// 不再走「只读取需要的那一页」的短路逻辑。

// aggregateDeviceProperty 按时间分桶聚合属性历史
func (c *Client) aggregateDeviceProperty(req dtos.ThingModelPropertyDataRequest, baseKey string) ([]dtos.ReportData, int, error) {
	var response []dtos.ReportData
	if len(req.Range) != 2 {
		return response, 0, fmt.Errorf("聚合查询必须指定时间范围")
	}

	startTime := minInt64(req.Range[0], req.Range[1])
	endTime := maxInt64(req.Range[0], req.Range[1])

	rows := make([]dtos.ReportData, 0, 256)
	iter := c.client.NewIterator(&util.Range{
		Start: []byte(baseKey + strconv.Itoa(int(startTime))),
		Limit: []byte(baseKey + strconv.Itoa(int(endTime))),
	}, &opt.ReadOptions{DontFillCache: true})
	if iter.Last() {
		for {
			var dbvalue dtos.ReportData
			_ = json.Unmarshal(iter.Value(), &dbvalue)
			rows = append(rows, dbvalue)
			if !iter.Prev() {
				break
			}
		}
	}
	iter.Release()

	buckets := aggregate.Bucketize(rows, req.AggInterval, req.AggMethod)
	page, total := aggregate.Paginate(buckets, req.Page, req.PageSize)
	return page, total, nil
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
