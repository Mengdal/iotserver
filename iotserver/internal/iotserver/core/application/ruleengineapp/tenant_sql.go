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
	// 存量规则的 filter.sql 里可能已经带有租户条件（旧版本把编译结果存成了原始 SQL），
	// 导入/多次保存也可能叠加。这里先剥掉再注入，保证无论调用多少次都只有一个租户条件。
	if stripped, changed := stripTenantPredicates(sql); changed {
		sql = stripped
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

// stripTenantPredicates 移除 enforceTenantFilter 注入过的顶层租户条件。
//
// 旧版本把注入后的 SQL 直接存进 filter.sql，启动时又会再注入一次；
// 导出→导入到其他租户时也会叠加。这里把形如
//
//	WHERE tenantId = 't' AND (原条件)
//	WHERE tenantId = 't'
//
// 的前缀逐层剥掉，返回清理后的 SQL 和是否发生修改。
// 只处理顶层 WHERE，括号/引号内的条件不会被误删。
func stripTenantPredicates(sql string) (string, bool) {
	sql = strings.TrimSpace(sql)
	changed := false
	// 叠加层数很少；限制循环次数，避免异常输入导致死循环
	for i := 0; i < 16; i++ {
		clauses, err := scanTopLevel(sql)
		if err != nil || clauses.where < 0 {
			break
		}
		whereEnd := clauses.where + len("WHERE")
		end := len(sql)
		for _, p := range []int{clauses.group, clauses.having, clauses.order, clauses.limit} {
			if p > whereEnd && p < end {
				end = p
			}
		}
		if whereEnd > end {
			break
		}
		rest, ok := stripLeadingTenantPredicate(sql[whereEnd:end])
		if !ok {
			break
		}
		changed = true
		if rest == "" {
			// 整个 WHERE 只剩租户条件：直接去掉 WHERE 子句
			sql = strings.TrimRight(sql[:clauses.where], " \t\r\n") + " " + strings.TrimLeft(sql[end:], " \t\r\n")
		} else {
			sql = strings.TrimRight(sql[:whereEnd], " \t\r\n") + " " + rest + " " + strings.TrimLeft(sql[end:], " \t\r\n")
		}
		sql = strings.TrimSpace(sql)
	}
	return sql, changed
}

// stripLeadingTenantPredicate 尝试剥掉 WHERE 条件开头的 tenantId = 'x'。
// 返回剩余条件；rest 为空表示租户条件是 WHERE 的全部内容。
func stripLeadingTenantPredicate(cond string) (string, bool) {
	s := strings.TrimSpace(cond)
	const keyword = "tenantId"
	if len(s) < len(keyword) || !strings.EqualFold(s[:len(keyword)], keyword) {
		return "", false
	}
	rest := s[len(keyword):]
	if rest != "" && isWordPart(rest[0]) {
		return "", false
	}
	rest = strings.TrimLeft(rest, " \t\r\n")
	if rest == "" || rest[0] != '=' {
		return "", false
	}
	rest = strings.TrimLeft(rest[1:], " \t\r\n")
	if rest == "" || rest[0] != '\'' {
		return "", false
	}
	endQuote := strings.IndexByte(rest[1:], '\'')
	if endQuote < 0 {
		return "", false
	}
	if !tenantIDPattern.MatchString(rest[1 : 1+endQuote]) {
		return "", false
	}
	rest = strings.TrimSpace(rest[1+endQuote+1:])
	if rest == "" {
		return "", true
	}
	if len(rest) < len("AND") || !strings.EqualFold(rest[:len("AND")], "AND") {
		return "", false
	}
	if len(rest) > len("AND") && isWordPart(rest[len("AND")]) {
		return "", false
	}
	rest = strings.TrimSpace(rest[len("AND"):])
	if rest == "" {
		return "", false
	}
	// 注入时原条件一定被括号包住；能完整匹配就解一层括号
	if rest[0] == '(' {
		if inner, ok := unwrapParens(rest); ok {
			return strings.TrimSpace(inner), true
		}
	}
	return rest, true
}

// unwrapParens 若 s 恰好是一个括号包裹的表达式，返回括号内的内容。
func unwrapParens(s string) (string, bool) {
	depth := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '\'', '"', '`':
			quote = ch
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				if strings.TrimSpace(s[i+1:]) != "" {
					return "", false
				}
				return s[1:i], true
			}
			if depth < 0 {
				return "", false
			}
		}
	}
	return "", false
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
			// 只拒绝 FROM 子句里的逗号（多数据源）；GROUP BY / ORDER BY 等
			// 子句内的逗号是合法表达式分隔符。
			if c.where < 0 && c.group < 0 && c.having < 0 && c.order < 0 && c.limit < 0 {
				return c, errort.NewCommonErr(errort.DefaultReqParamsError,
					errort.NewMessageErr("规则 SQL 不支持多数据源", nil))
			}
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
