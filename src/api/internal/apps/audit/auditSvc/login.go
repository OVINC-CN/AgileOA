package auditSvc

import (
	"time"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
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

// LoginTrend 保留用户管理原上海日期口径，结束时刻范围不会漏掉跨日长请求。
func LoginTrend(tx *gorm.DB, start, end time.Time) ([]auditModels.LoginDay, error) {
	// 查询只读取成功登录投影；日期来自结束时刻，不套用开始日期的下界。
	result := []auditModels.LoginDay{}
	err := tx.Model(&auditModels.Record{}).
		Select("DATE_FORMAT(TIMESTAMPADD(HOUR,8,TIMESTAMPADD(MICROSECOND,end_time * 1000,CAST('1970-01-01 00:00:00.000000' AS DATETIME(6)))),'%Y-%m-%d') AS date, COUNT(*) AS count").
		Where("is_login_event = 1 AND end_time >= ? AND end_time < ?", start.UnixMilli(), end.UnixMilli()).
		Group("date").Scan(&result).Error
	return result, err
}
