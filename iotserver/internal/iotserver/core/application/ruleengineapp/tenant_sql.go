package ruleengineapp

import (
	"fmt"
	"regexp"
	"strings"

	"iotserver/internal/pkg/errort"
)

// 规则引擎的 mqtt_stream 是所有租户共享的（订阅 eventbus/in）。
// 规则 SQL 由前端/API 直接提交，如果原样下发，租户可以去掉租户条件读取其他租户的数据。
// 这里在保存/启动规则时强制在外层条件里加上 tenantId。

var tenantIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type sqlClauses struct {
	from   int
	where  int
	group  int
	having int
	order  int
	limit  int
}

// enforceTenantFilter 给规则 SQL 加上租户过滤条件。
//
// 只处理顶层查询（括号/引号内的子查询不受影响）：
//   - 有 WHERE：WHERE tenantId = 'x' AND (原有条件)
//   - 无 WHERE：在 GROUP/HAVING/ORDER/LIMIT 之前插入 WHERE tenantId = 'x'
//
// 同时拒绝多数据源/多语句/注释，避免绕过过滤条件。
func enforceTenantFilter(sql, tenantID string) (string, error) {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return "", errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("规则 SQL 不能为空", nil))
	}
	// 平台作用域创建的规则不限制
	if tenantID == "" {
		return sql, nil
	}
	if !tenantIDPattern.MatchString(tenantID) {
		return "", errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("租户标识非法", nil))
	}

	clauses, err := scanTopLevel(sql)
	if err != nil {
		return "", err
	}
	if clauses.from < 0 {
		return "", errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("规则 SQL 必须从 mqtt_stream 取数", nil))
	}
	// FROM 后必须是 mqtt_stream（允许别名），不能是 mqtt_stream_xxx 这类同前缀的其他流
	rest := strings.TrimSpace(sql[clauses.from+len("FROM"):])
	if len(rest) < len("mqtt_stream") ||
		!strings.EqualFold(rest[:len("mqtt_stream")], "mqtt_stream") ||
		(len(rest) > len("mqtt_stream") && isWordPart(rest[len("mqtt_stream")])) {
		return "", errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("规则 SQL 必须从 mqtt_stream 取数", nil))
	}

	filter := fmt.Sprintf("tenantId = '%s'", tenantID)

	if clauses.where < 0 {
		pos := len(sql)
		for _, p := range []int{clauses.group, clauses.having, clauses.order, clauses.limit} {
			if p >= 0 && p < pos {
				pos = p
			}
		}
		return strings.TrimRight(sql[:pos], " \t\r\n") + " WHERE " + filter + " " + sql[pos:], nil
	}

	whereEnd := clauses.where + len("WHERE")
	end := len(sql)
	for _, p := range []int{clauses.group, clauses.having, clauses.order, clauses.limit} {
		if p > whereEnd && p < end {
			end = p
		}
	}
	cond := strings.TrimSpace(sql[whereEnd:end])
	if cond == "" {
		return "", errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("规则 SQL 的 WHERE 条件为空", nil))
	}
	return sql[:whereEnd] + " " + filter + " AND (" + cond + ") " + sql[end:], nil
}

// scanTopLevel 扫描顶层 SQL 结构，忽略引号/括号内的内容。
func scanTopLevel(sql string) (sqlClauses, error) {
	c := sqlClauses{from: -1, where: -1, group: -1, having: -1, order: -1, limit: -1}
	depth := 0
	var quote byte
	for i := 0; i < len(sql); i++ {
		ch := sql[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '\'', '"', '`':
			quote = ch
			continue
		case '(':
			depth++
			continue
		case ')':
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth == 0 && ch == ';' {
			return c, errort.NewCommonErr(errort.DefaultReqParamsError,
				errort.NewMessageErr("规则 SQL 不支持多条语句", nil))
		}
		if depth == 0 && ch == '-' && i+1 < len(sql) && sql[i+1] == '-' {
			return c, errort.NewCommonErr(errort.DefaultReqParamsError,
				errort.NewMessageErr("规则 SQL 不支持注释", nil))
		}
		if depth == 0 && ch == '/' && i+1 < len(sql) && sql[i+1] == '*' {
			return c, errort.NewCommonErr(errort.DefaultReqParamsError,
				errort.NewMessageErr("规则 SQL 不支持注释", nil))
		}
		if depth == 0 && ch == ',' && c.from >= 0 {
			return c, errort.NewCommonErr(errort.DefaultReqParamsError,
				errort.NewMessageErr("规则 SQL 不支持多数据源", nil))
		}
		if depth != 0 || !isWordStart(ch) {
			continue
		}
		j := i + 1
		for j < len(sql) && isWordPart(sql[j]) {
			j++
		}
		switch strings.ToUpper(sql[i:j]) {
		case "FROM":
			if c.from < 0 {
				c.from = i
			}
		case "WHERE":
			if c.from >= 0 && c.where < 0 {
				c.where = i
			}
		case "GROUP":
			if c.from >= 0 && c.group < 0 {
				c.group = i
			}
		case "HAVING":
			if c.from >= 0 && c.having < 0 {
				c.having = i
			}
		case "ORDER":
			if c.from >= 0 && c.order < 0 {
				c.order = i
			}
		case "LIMIT":
			if c.from >= 0 && c.limit < 0 {
				c.limit = i
			}
		case "JOIN", "UNION":
			return c, errort.NewCommonErr(errort.DefaultReqParamsError,
				errort.NewMessageErr("规则 SQL 不支持多数据源", nil))
		}
		i = j - 1
	}
	return c, nil
}

func isWordStart(ch byte) bool {
	return ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isWordPart(ch byte) bool {
	return isWordStart(ch) || (ch >= '0' && ch <= '9')
}
