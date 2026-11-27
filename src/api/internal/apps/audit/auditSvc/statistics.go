package auditSvc

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/displaytime"
)

const trendTopLimit = 10

type statisticsRow struct {
	AuditDate  string
	TrendValue string
	Total      int64
	Success    int64
	Failure    int64
	Denied     int64
}

// Statistics 用同一组筛选记录生成总计和展示时区内的按日趋势。
func Statistics(ctx context.Context, query auditModels.Query) (auditModels.StatisticsData, error) {
	result := auditModels.StatisticsData{Days: []auditModels.Day{}}
	event := queryEvent()
	event.ObjectID = "statistics"
	err := read(
		ctx,
		accessConstants.AuditStatistics,
		event,
		func(tx *gorm.DB) (auditModels.Event, error) {
			var rows []statisticsRow
			if err := statisticsQuery(tx, query).Scan(&rows).Error; err != nil {
				return event, err
			}

			// 总计和成功失败趋势包含前十项及其他中的全部记录。
			byDate := make(map[string]auditModels.Day)
			for _, row := range rows {
				result.Total += row.Total
				result.Success += row.Success
				result.Failure += row.Failure
				result.Denied += row.Denied
				day := byDate[row.AuditDate]
				day.Success += row.Success
				day.Failure += row.Failure
				byDate[row.AuditDate] = day
			}
			last := displaytime.Date(query.Until.Add(-time.Millisecond))
			for day := displaytime.Date(query.From); !day.After(last); day = day.AddDate(0, 0, 1) {
				date := day.Format(time.DateOnly)
				item := byDate[date]
				item.Date = date
				result.Days = append(result.Days, item)
			}
			if query.Dimension == auditConstants.DimensionWorkspace ||
				query.Dimension == auditConstants.DimensionAction {
				var err error
				result.Trend, err = buildTrend(tx, query.Dimension, result.Days, rows)
				return event, err
			}
			return event, nil
		},
	)
	return result, err
}

func statisticsQuery(tx *gorm.DB, query auditModels.Query) *gorm.DB {
	const counts = "COUNT(*) AS total, SUM(result_code = 0) AS success, " +
		"SUM(result_code <> 0) AS failure, SUM(result_code = 403) AS denied"
	// 派生查询各自创建语句，沿用当前事务和上下文。
	tx = tx.Session(&gorm.Session{NewDB: true})
	date := displaytime.UnixMilliDateSQL("start_time", query.From, query.Until)
	daily := queryScope(tx, query).Group("audit_date")
	var column string
	switch query.Dimension {
	case auditConstants.DimensionWorkspace:
		column = "workspace_id"
	case auditConstants.DimensionAction:
		column = "action_id"
	default:
		return daily.Select("? AS audit_date, "+counts, date)
	}

	// 先按日和目录字段分组，再按整个范围内的总数和编号确定唯一名次。
	daily.Select("? AS audit_date, ? AS series_value, "+counts, date, clause.Column{Name: column}).
		Group("series_value")
	totals := tx.Table("(?) AS audit_daily", daily).
		Select("audit_daily.*, SUM(total) OVER (PARTITION BY series_value) AS series_total")
	ranked := tx.Table("(?) AS audit_totals", totals).
		Select("audit_totals.*, DENSE_RANK() OVER (ORDER BY series_total DESC, series_value) AS series_rank")

	// 数据库按日合并所有超出名次的项，其他始终排在前十项之后。
	return tx.Table("(?) AS audit_ranked", ranked).Select(
		"audit_date, CASE WHEN series_rank <= ? THEN series_value ELSE '' END AS trend_value, "+
			"SUM(total) AS total, SUM(success) AS success, SUM(failure) AS failure, "+
			"SUM(denied) AS denied, MIN(series_rank) AS series_position",
		trendTopLimit,
	).Group("audit_date").Group("trend_value").Order("series_position")
}

func buildTrend(
	tx *gorm.DB,
	dimension string,
	days []auditModels.Day,
	rows []statisticsRow,
) (*auditModels.Trend, error) {
	positions := make(map[string]int)
	values := make([]string, 0, trendTopLimit+1)
	for _, row := range rows {
		if _, seen := positions[row.TrendValue]; !seen {
			positions[row.TrendValue] = len(values)
			values = append(values, row.TrendValue)
		}
	}
	series, err := trendSeries(tx, dimension, values)
	if err != nil {
		return nil, err
	}

	// 每天使用相同的统计项顺序，缺少的日期或统计项为零。
	trend := &auditModels.Trend{Series: series, Days: make([]auditModels.TrendDay, 0, len(days))}
	counts := make(map[string][]int64, len(days))
	for _, day := range days {
		point := auditModels.TrendDay{Date: day.Date, Counts: make([]int64, len(series))}
		trend.Days = append(trend.Days, point)
		counts[day.Date] = point.Counts
	}
	for _, row := range rows {
		if dayCounts, exists := counts[row.AuditDate]; exists {
			dayCounts[positions[row.TrendValue]] = row.Total
		}
	}
	return trend, nil
}

func trendSeries(tx *gorm.DB, dimension string, values []string) ([]auditModels.Option, error) {
	var labels map[string]string
	if dimension == auditConstants.DimensionWorkspace {
		var ids []string
		for _, value := range values {
			if value != "" && value != auditConstants.SystemWorkspace {
				ids = append(ids, value)
			}
		}
		var err error
		labels, err = workspaceNames(tx, ids)
		if err != nil {
			return nil, err
		}
	}
	series := make([]auditModels.Option, 0, len(values))
	for _, value := range values {
		option := auditModels.Option{Value: value, Label: value}
		switch {
		case value == "":
			option = auditModels.Option{Value: "other", Label: "其他"}
		case dimension == auditConstants.DimensionWorkspace:
			option.Label = workspaceLabel(value, labels)
		default:
			if action, found := accessConstants.Lookup(value); found {
				option.Label = action.Label
			}
		}
		series = append(series, option)
	}
	return series, nil
}
