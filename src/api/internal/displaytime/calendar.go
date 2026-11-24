package displaytime

import (
	"time"

	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
)

const dayDuration = 24 * time.Hour

// Date 返回展示时区的公历日期，以 UTC 午夜承载日期运算。
func Date(instant time.Time) time.Time {
	return wallTime(instant).Truncate(dayDuration)
}

// Window 返回包含当天的自然日窗口，起止边界为 UTC 时刻。
func Window(now time.Time, days int) (time.Time, time.Time) {
	today := Date(now)
	return dayStart(today.AddDate(0, 0, 1-days)), dayStart(today.AddDate(0, 0, 1))
}

// WithinDays 按展示时区的日历时间比较查询跨度，兼容夏令时的长短日。
func WithinDays(from, until time.Time, days int) bool {
	return !wallTime(until).After(wallTime(from).AddDate(0, 0, days))
}

func wallTime(instant time.Time) time.Time {
	local := instant.In(config.Config.App.Location)
	year, month, day := local.Date()
	hour, minute, second := local.Clock()
	return time.Date(year, month, day, hour, minute, second, local.Nanosecond(), time.UTC)
}

func dayStart(date time.Time) time.Time {
	// 逐段查找首个达到指定公历日期的时刻，兼容午夜跳过或出现两次。
	location := config.Config.App.Location
	for instant := date.Add(-dayDuration).In(location); ; {
		_, offset := instant.Zone()
		candidate := date.Add(-time.Duration(offset) * time.Second)
		next := nextZoneBoundary(instant)
		if next.IsZero() || candidate.Before(next) {
			if candidate.Before(instant) {
				return instant.UTC()
			}
			return candidate
		}
		instant = next
	}
}

func nextZoneBoundary(instant time.Time) time.Time {
	_, next := instant.ZoneBounds()
	if !next.IsZero() && !next.After(instant) {
		// Go 推算未来时区时可能将闰年末的区间边界返回为 12 月 31 日。
		newYear := time.Date(instant.UTC().Year()+1, time.January, 1, 0, 0, 0, 0, time.UTC)
		return newYear.In(instant.Location())
	}
	return next
}
