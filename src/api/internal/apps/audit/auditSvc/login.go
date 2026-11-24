package auditSvc

import (
	"time"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/displaytime"
)

// LatestLogins 在现有业务事务中读取最近登录，业务无需了解审计存储字段。
func LatestLogins(tx *gorm.DB, ids []uint64) ([]auditModels.LatestLogin, error) {
	// 登录投影和整数编号提供独立覆盖查询，空目录不访问数据库。
	result := make([]auditModels.LatestLogin, 0, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		UserID       uint64
		Milliseconds int64
	}
	if err := tx.Model(&auditModels.Record{}).
		Select("user_id, MAX(end_time) AS milliseconds").
		Where("is_login_event = 1 AND user_id IN ?", ids).Group("user_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result = append(result, auditModels.LatestLogin{
			UserID: row.UserID, Latest: time.UnixMilli(row.Milliseconds).UTC(),
		})
	}
	return result, nil
}

// LoginTrend 按展示时区汇总成功登录的结束日期。
func LoginTrend(tx *gorm.DB, start, end time.Time) ([]auditModels.LoginDay, error) {
	// 查询只读取成功登录投影；日期来自结束时刻，不套用开始日期的下界。
	result := []auditModels.LoginDay{}
	date := displaytime.UnixMilliDateSQL("end_time", start, end)
	err := tx.Model(&auditModels.Record{}).
		Select("? AS date, COUNT(*) AS count", date).
		Where("is_login_event = 1 AND end_time >= ? AND end_time < ?", start.UnixMilli(), end.UnixMilli()).
		Group("date").Scan(&result).Error
	return result, err
}
