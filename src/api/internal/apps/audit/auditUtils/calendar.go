package auditUtils

import (
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
)

// Location 保留管理统计的上海时区；标准审计使用独立 UTC 窗口。
var Location *time.Location

func init() {
	var err error
	Location, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		logger.Logger.Fatalf("Audit timezone initialization failed: %v", err)
	}
}

// Window 返回包含今天的连续日期窗口，边界为上海自然日。
func Window(days int) (time.Time, time.Time) {
	now := time.Now().In(Location)
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, Location).AddDate(0, 0, 1)
	return end.AddDate(0, 0, -days), end
}

// UTCWindow 仅供标准审计使用，不改变其他管理统计的展示时区。
func UTCWindow(days int) (time.Time, time.Time) {
	now := time.Now().UTC()
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	return end.AddDate(0, 0, -days), end
}
