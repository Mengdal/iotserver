package sqlite

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	sqldb "iotserver/internal/tools/sqldb/sqlite"
)

// batchUpsert 批量写入，主键冲突时更新全部字段。
func batchUpsert[T any](c *Client, rows []T) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	tx := c.Pool.Session(&gorm.Session{FullSaveAssociations: true}).Clauses(
		clause.OnConflict{UpdateAll: true},
	).CreateInBatches(rows, sqldb.CreateBatchSize)
	return tx.RowsAffected, tx.Error
}
