package workspaceRouters

import (
	"strings"
	"unicode/utf8"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/workspace/workspaceModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/workspace/workspaceSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileOA/src/api/internal/queryfilter"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// List 校验系统管理员工作空间列表参数。
// @Summary 工作空间管理列表
// @Description 仅系统管理员可查询全部工作空间，读取在审计提交后返回。
// @Tags 工作空间管理
// @Produce json
// @Param page query int false "页码，从 1 开始" default(1)
// @Param page_size query int false "每页数量，默认与上限来自 YAML"
// @Param keyword query string false "工作空间名称关键词，最多 100 个字符"
// @Param status query []string false "状态：enabled、disabled 或留空；支持多选，同一参数可重复传递" collectionFormat(multi)
// @Success 200 {object} workspaceModels.ListResponse "工作空间分页目录"
// @Router /v1/system/workspaces [get]
func List(c echo.Context) error {
	page, err := pagination.Parse(c)
	if err != nil {
		return err
	}
	query := workspaceModels.ListQuery{
		Query:    page,
		Keyword:  strings.TrimSpace(c.QueryParam("keyword")),
		Statuses: queryfilter.Values(c.QueryParams()["status"], false),
	}
	if utf8.RuneCountInString(query.Keyword) > 100 ||
		!queryfilter.ContainsOnly(query.Statuses, "enabled", "disabled") {
		return echo.ErrBadRequest
	}
	data, err := workspaceSvc.List(c.Request().Context(), query)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Enum 返回供本地搜索选择器使用的完整工作空间枚举。
// @Summary 工作空间枚举
// @Description 仅系统管理员可查询，一次返回全部空间的编号、名称和状态，不分页、不远程搜索；审计提交后才返回。
// @Tags 工作空间管理
// @Produce json
// @Success 200 {object} workspaceModels.EnumResponse "完整工作空间枚举"
// @Router /v1/system/workspaces/enum [get]
func Enum(c echo.Context) error {
	data, err := workspaceSvc.Enum(c.Request().Context())
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Create 校验工作空间初始化参数。
// @Summary 创建工作空间
// @Description 仅系统管理员可初始化工作空间，不自动分配 action。名称全局唯一，重名返回 409。
// @Tags 工作空间管理
// @Accept json
// @Produce json
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Param body body workspaceModels.CreateRequest true "工作空间基本信息"
// @Success 200 {object} workspaceModels.InfoResponse "已创建工作空间"
// @Failure 409 {object} accessModels.EmptyResponse "工作空间名称已存在"
// @Router /v1/system/workspaces [post]
func Create(c echo.Context) error {
	var request workspaceModels.CreateRequest
	if err := accessUtils.Decode(c, &request); err != nil {
		return echo.ErrBadRequest
	}
	data, err := workspaceSvc.Create(c.Request().Context(), request)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Detail 读取工作空间管理信息。
// @Summary 工作空间管理详情
// @Description 仅系统管理员可读取，工作空间业务使用独立的进入接口。
// @Tags 工作空间管理
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Success 200 {object} workspaceModels.InfoResponse "工作空间详情"
// @Router /v1/system/workspaces/{workspace_id} [get]
func Detail(c echo.Context) error {
	id, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := workspaceSvc.Detail(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Update 校验工作空间基本信息、状态与并发版本。
// @Summary 更新工作空间
// @Description 仅系统管理员可操作；停用立即阻断所有工作空间业务，不自动清理引用。名称全局唯一，重名或版本冲突返回 409。
// @Tags 工作空间管理
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Param body body workspaceModels.UpdateRequest true "工作空间更新参数"
// @Success 200 {object} workspaceModels.InfoResponse "已更新工作空间"
// @Failure 409 {object} accessModels.EmptyResponse "名称已存在或工作空间已被修改"
// @Router /v1/system/workspaces/{workspace_id} [put]
func Update(c echo.Context) error {
	id, err := accessUtils.WorkspaceID(c)
	var request workspaceModels.UpdateRequest
	if err != nil || accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	data, err := workspaceSvc.Update(c.Request().Context(), id, request)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Delete 显式硬删除没有引用的工作空间。
// @Summary 删除工作空间
// @Description 仅系统管理员可操作；存在权限或业务引用时拒绝，不级联清理，保留审计。
// @Tags 工作空间管理
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} accessModels.EmptyResponse "工作空间已删除"
// @Router /v1/system/workspaces/{workspace_id} [delete]
func Delete(c echo.Context) error {
	id, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	if err := workspaceSvc.Delete(
		c.Request().Context(),
		id,
	); err != nil {
		return err
	}
	return response.Success[*accessModels.EmptyData](c, nil)
}

// Enter 校验指定工作空间的实时用户授权。
// @Summary 进入工作空间
// @Description 要求启用账号、启用工作空间及显式授予的 access_workspace；节点指派和系统管理员身份均不能绕过。
// @Tags 工作空间
// @Produce json
// @Param workspace_id path string true "当前工作空间编号，不从会话推导"
// @Success 200 {object} accessModels.ContextResponse "当前工作空间与显式权限"
// @Router /v1/workspaces/{workspace_id} [get]
func Enter(c echo.Context) error {
	id, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := workspaceSvc.Enter(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}
