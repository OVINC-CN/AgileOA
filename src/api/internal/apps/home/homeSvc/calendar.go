package homeSvc

import (
	"context"
	"net/http"
	"time"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/home/homeModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
	"github.com/OVINC-CN/AgileOA/src/api/internal/tracing"
)

// Calendar 只读公开节日，不读取账号、租户或法定调休信息。
func Calendar(ctx context.Context, date time.Time) (homeModels.CalendarData, error) {
	// 节日日期由请求显式指定，避免服务器时区与首页展示时区不一致。
	ctx, span := tracing.Tracer.Start(ctx, "homeSvc.Calendar")
	defer span.End()
	key := date.Format(time.DateOnly)
	data := homeModels.CalendarData{
		CalendarDay: homeModels.CalendarDay{Date: key, Holidays: []string{}},
	}
	if db.DB() == nil {
		return data, &response.BusinessError{
			Status: http.StatusServiceUnavailable, Message: "节日信息暂时不可用",
		}
	}

	// 连续同名日期视为一段假期，只对假期首日倒计时，避免假期中每天倒计时到明天。
	database := db.DB().WithContext(ctx)
	previousDay := database.Table("calendar_holidays AS previous").Select("1").
		Where("previous.date = DATE_SUB(calendar_holidays.date, INTERVAL 1 DAY)").
		Where("previous.holiday = calendar_holidays.holiday")
	nextDate := database.Model(&homeModels.CalendarHoliday{}).
		Select("MIN(`date`)").Where("`date` > ?", key).
		Where("NOT EXISTS (?)", previousDay)
	var records []homeModels.CalendarHoliday
	if err := database.Select("date", "holiday").
		Where("(`date` = ? OR `date` = (?))", key, nextDate).
		Where("(`date` = ? OR NOT EXISTS (?))", key, previousDay).
		Order("date ASC").Order("holiday ASC").Find(&records).Error; err != nil {
		return data, err
	}

	// 无记录保持空数组或 null，不推算农历节日，也不自动重复上一年的日期。
	for _, record := range records {
		recordDate := record.Date.Format(time.DateOnly)
		if recordDate == key {
			data.Holidays = append(data.Holidays, record.Holiday)
			continue
		}
		if data.Next == nil {
			data.Next = &homeModels.CalendarDay{Date: recordDate, Holidays: []string{}}
		}
		data.Next.Holidays = append(data.Next.Holidays, record.Holiday)
	}
	return data, nil
}
