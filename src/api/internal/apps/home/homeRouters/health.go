package homeRouters

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/home/homeModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/home/homeSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// Health 返回进程存活结果。
// @Summary 进程存活检查
// @Description 仅通过 HTTP 状态表达结果，data 固定为 null，不提供内部状态。
// @Tags 健康检查
// @Produce json
// @Success 200 {object} homeModels.HealthResponse "进程存活，error_msg 为空，data 为 null"
// @Router /health [get]
func Health(c echo.Context) error {
	// 此接口没有请求参数，后续处理交给服务层。
	if err := homeSvc.Health(c.Request().Context()); err != nil {
		return err
	}

	// 成功响应也不提供业务数据。
	return response.Success[*homeModels.EmptyData](c, nil)
}

// Ready 返回服务就绪结果。
// @Summary 服务就绪检查
// @Description 仅通过 HTTP 状态表达结果，data 固定为 null，不公开依赖资源明细。
// @Tags 健康检查
// @Produce json
// @Success 200 {object} homeModels.HealthResponse "服务就绪，error_msg 为空，data 为 null"
// @Router /ready [get]
func Ready(c echo.Context) error {
	// 此接口没有请求参数，依赖检查全部由服务层负责。
	if err := homeSvc.Ready(c.Request().Context()); err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable).SetInternal(err)
	}

	// 失败详情由统一错误处理隐藏，对外只返回通用错误说明。
	return response.Success[*homeModels.EmptyData](c, nil)
}
