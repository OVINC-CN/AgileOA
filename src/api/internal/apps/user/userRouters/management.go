package userRouters

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileOA/src/api/internal/queryfilter"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// List 返回管理后台的用户分页目录。
// @Summary 用户管理列表
// @Description 查询全局账号目录，读取结果在审计提交后返回，不包含口令或会话凭据。
// @Tags 用户管理
// @Produce json
// @Param page query int false "页码，从 1 开始" default(1)
// @Param page_size query int false "每页数量，默认与上限来自 YAML"
// @Param keyword query string false "按用户名、显示名称或邮箱搜索"
// @Param status query []string false "账号状态：enabled、disabled 或留空；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Success 200 {object} userModels.ListResponse "用户分页目录"
// @Router /v1/system/users [get]
func List(c echo.Context) error {
	page, err := pagination.Parse(c)
	if err != nil {
		return userSvc.Reject(
			c.Request().Context(),
			accessConstants.UserList,
			http.StatusBadRequest,
		)
	}
	query := userModels.ListQuery{
		Query:    page,
		Keyword:  strings.TrimSpace(c.QueryParam("keyword")),
		Statuses: queryfilter.Values(c.QueryParams()["status"], false),
	}
	if len([]rune(query.Keyword)) > userConstants.MaxEmailLength ||
		!queryfilter.ContainsOnly(query.Statuses, "enabled", "disabled") {
		return userSvc.Reject(
			c.Request().Context(),
			accessConstants.UserList,
			http.StatusBadRequest,
		)
	}
	data, err := userSvc.List(c.Request().Context(), query)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Statistics 返回真实用户数量与每日趋势。
// @Summary 用户管理统计
// @Description 汇总真实账号数量和状态，按配置的展示时区统计自然日内的新增和登录次数。
// @Tags 用户管理
// @Produce json
// @Success 200 {object} userModels.StatisticsResponse "用户统计"
// @Router /v1/system/users/statistics [get]
func Statistics(c echo.Context) error {
	data, err := userSvc.Statistics(c.Request().Context())
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Update 更新指定账号的显示名称、邮箱、启用状态、密码登录设置或口令。
// @Summary 更新用户
// @Description 用户名不可修改。停用、调整密码登录方式和重置密码会作废旧会话，仅停用账号会撤销 API Token。
// @Description 关闭密码登录时清除口令和待改密状态；重新启用时须设置临时密码，下次登录须修改。
// @Description 省略密码登录设置时保留原值，调整密码设置不影响已有 API Token。
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param id path int true "账号编号"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Param body body userModels.UpdateRequest true "用户更新参数"
// @Success 200 {object} userModels.InfoResponse "已更新账号"
// @Router /v1/system/users/{id} [put]
func Update(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	var request userModels.UpdateRequest
	if err != nil || id == 0 || decode(c, &request) != nil {
		return userSvc.Reject(
			c.Request().Context(),
			accessConstants.UserUpdate,
			http.StatusBadRequest,
		)
	}
	data, err := userSvc.Update(c.Request().Context(), id, request)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Delete 硬删除没有会话或其他引用的账号。
// @Summary 删除用户
// @Description 禁止删除当前登录账号；存在会话、API Token 或评奖评优审批引用时拒绝删除。其余依赖记录按外键级联删除，保留历史审计及身份快照。
// @Tags 用户管理
// @Produce json
// @Param id path int true "账号编号"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} userModels.EmptyResponse "用户已删除"
// @Router /v1/system/users/{id} [delete]
func Delete(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return userSvc.Reject(
			c.Request().Context(),
			accessConstants.UserDelete,
			http.StatusBadRequest,
		)
	}
	if err := userSvc.Delete(c.Request().Context(), id); err != nil {
		return err
	}
	return response.Success[*userModels.EmptyData](c, nil)
}

// RevokeSessions 由管理员显式撤销目标账号的全部会话。
// @Summary 撤销用户会话
// @Description 与删除账号分开执行，同时撤销全部会话、API Token 和待使用的邮件验证码，逐条保留撤销前快照及共享请求编号。
// @Tags 用户管理
// @Produce json
// @Param id path int true "账号编号"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} userModels.EmptyResponse "会话已撤销"
// @Router /v1/system/users/{id}/sessions [delete]
func RevokeSessions(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return userSvc.Reject(
			c.Request().Context(),
			accessConstants.UserRevokeSessions,
			http.StatusBadRequest,
		)
	}
	if err := userSvc.RevokeSessions(c.Request().Context(), id); err != nil {
		return err
	}
	return response.Success[*userModels.EmptyData](c, nil)
}
