package homeModels

import (
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// CalendarHoliday 按公历日期保存公开节日，同一天允许多个不同名称。
type CalendarHoliday struct {
	Date      time.Time `gorm:"primaryKey;type:date"`
	Holiday   string    `gorm:"primaryKey"`
	CreatedAt time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt time.Time `gorm:"->;autoUpdateTime:false"`
}

// CalendarQuery 仅接受首页展示时区中的公历日期。
type CalendarQuery struct {
	// Date 是查询日期，格式为 YYYY-MM-DD，年份范围为 1000 至 9999。
	Date string `query:"date" example:"2027-01-01"`
}

// CalendarDay 是节日的固定对外字段目录，不提供内部维护时间。
type CalendarDay struct {
	// Date 是节日对应的公历日期，不包含时区，格式为 YYYY-MM-DD。
	Date string `json:"date" example:"2027-01-01"`
	// Holidays 是该日期的节日或假期名称，无记录时返回空数组，不包含补班安排。
	Holidays []string `json:"holidays" example:"元旦"`
}

// CalendarData 返回指定日期与最近的后续节假日首日，连续同名日期视为一段假期。
type CalendarData struct {
	CalendarDay
	// Next 是下一个节假日的开始日期及名称，不含正在进行的连续假期，无后续记录为 null。
	Next *CalendarDay `json:"next"`
}

// CalendarResponse 返回无需登录的首页节日信息。
type CalendarResponse struct {
	response.Envelope[CalendarData]
}
