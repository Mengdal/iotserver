package iotp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// AuditLogs 读取 iotp 控制台「变更记录」页同源的最近审计记录。
//
// 不同版本的 iotp 可能直接返回数组，或包在 data / entries / result 等字段里，
// 这里做兼容解析，调用方只需要拿到一组对象。
func (c *Client) AuditLogs(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	var raw json.RawMessage
	path := fmt.Sprintf("/api/v1/meta/auth/audit?limit=%d", limit)
	if err := c.do(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return nil, err
	}
	return decodeAuditEntries(raw)
}

func decodeAuditEntries(raw json.RawMessage) ([]map[string]any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var entries []map[string]any
	if err := json.Unmarshal(raw, &entries); err == nil {
		return entries, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("iotp audit response: %w", err)
	}
	for _, key := range []string{"data", "entries", "items", "logs", "audit", "result", "list"} {
		value, ok := obj[key]
		if !ok {
			continue
		}
		if err := json.Unmarshal(value, &entries); err == nil {
			return entries, nil
		}
		var nested map[string]json.RawMessage
		if json.Unmarshal(value, &nested) != nil {
			continue
		}
		for _, nestedKey := range []string{"list", "items", "entries", "data"} {
			if nestedValue, ok := nested[nestedKey]; ok {
				if json.Unmarshal(nestedValue, &entries) == nil {
					return entries, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("iotp audit response shape is not recognized")
}
