package homeRouters

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/home/homeModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/home/homeSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// Calendar 返回公开的首页节日信息。
// @Summary 查看首页节日
// @Description 无需登录。返回当日节假日与后续假期首日；连续同名日期视为一段假期，不包含补班安排。
// @Tags 首页
// @Produce json
// @Param date query string true "展示时区中的公历日期，YYYY-MM-DD，年份为 1000 至 9999" example(2027-01-01)
// @Success 200 {object} homeModels.CalendarResponse
// @Failure 400 {object} homeModels.HealthResponse "日期缺失、重复或格式无效"
// @Router /v1/home/calendar [get]
func Calendar(c echo.Context) error {
	// 严格校验唯一日期参数，不使用服务器本地时间作为隐式默认值。
	query := homeModels.CalendarQuery{Date: c.QueryParam("date")}
	date, err := time.Parse(time.DateOnly, query.Date)
	if err != nil || date.Year() < 1000 || len(c.QueryParams()["date"]) != 1 {
		return &response.BusinessError{
			Status: http.StatusBadRequest, Message: "请提供有效日期，格式为 YYYY-MM-DD",
		}
	}

	// 业务读取完成后沿用统一响应，不要求会话或工作空间。
	data, err := homeSvc.Calendar(c.Request().Context(), date)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}
