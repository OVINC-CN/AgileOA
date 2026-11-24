package displaytime

import (
	"strings"
	"time"

	"gorm.io/gorm/clause"

	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
)

// DateSQL 将 UTC 时间列转成展示日期。查询范围仍直接约束原时间列。
func DateSQL(column string, from, until time.Time) clause.Expr {
	value := clause.Column{Name: column}
	return clause.Expr{
		SQL:  "DATE_FORMAT(TIMESTAMPADD(SECOND, ?, ?), '%Y-%m-%d')",
		Vars: []any{offsetSQL(value, from, until, false), value},
	}
}

// UnixMilliDateSQL 将 Unix 毫秒列转成展示日期，计算时不依赖 MySQL 时区表。
func UnixMilliDateSQL(column string, from, until time.Time) clause.Expr {
	value := clause.Column{Name: column}
	return clause.Expr{
		SQL: "DATE_FORMAT(TIMESTAMPADD(SECOND, ?, " +
			"TIMESTAMPADD(MICROSECOND, ? * 1000, CAST('1970-01-01' AS DATETIME(6)))), '%Y-%m-%d')",
		Vars: []any{offsetSQL(value, from, until, true), value},
	}
}

func offsetSQL(column clause.Column, from, until time.Time, unixMilli bool) clause.Expr {
	// 从 Go 时区库取得窗口内的偏移边界，值与时间边界均通过占位符传递。
	var sql strings.Builder
	var values []any
	for instant := from.In(config.Config.App.Location); ; {
		_, offset := instant.Zone()
		next := nextZoneBoundary(instant)
		if next.IsZero() || !next.Before(until) {
			if len(values) == 0 {
				return clause.Expr{SQL: "?", Vars: []any{offset}}
			}
			sql.WriteString(" ELSE ? END")
			return clause.Expr{SQL: sql.String(), Vars: append(values, offset)}
		}
		if sql.Len() == 0 {
			sql.WriteString("CASE")
		}
		sql.WriteString(" WHEN ? < ? THEN ?")
		var boundary any = next.UTC()
		if unixMilli {
			boundary = next.UnixMilli()
		}
		values = append(values, column, boundary, offset)
		instant = next
	}
}
