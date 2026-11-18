package accessRouters

import (
	"strings"
	"unicode/utf8"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// Catalog 读取代码维护的操作权限目录。
// @Summary 操作权限目录
// @Description 仅系统管理员可查看稳定标识、名称、说明、分组、范围与接口；仅 workspace 可勾选授权。
// @Tags 权限管理
// @Produce json
// @Success 200 {object} accessModels.CatalogResponse "只读操作目录"
// @Router /v1/system/access/actions [get]
func Catalog(c echo.Context) error {
	data, err := accessSvc.Catalog(c.Request().Context())
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Users 校验指定工作空间的用户列表条件。
// @Summary 工作空间用户列表
// @Description 仅系统管理员可查询，默认仅返回当前空间至少有一项有效显式权限的账号；ungranted 返回未授权候选，停用账号及其他空间授权不改变本空间归类。
// @Tags 权限管理
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param page query int false "页码，从 1 开始" default(1)
// @Param page_size query int false "每页数量，默认与上限来自 YAML"
// @Param keyword query string false "用户名或显示名称关键词，最多 100 个字符"
// @Param permission_status query string false "授权状态：granted 已授权，ungranted 未授权" Enums(granted,ungranted) default(granted)
// @Success 200 {object} accessModels.UserListResponse "用户分页目录"
// @Router /v1/system/access/workspaces/{workspace_id}/users [get]
func Users(c echo.Context) error {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	page, err := pagination.Parse(c)
	if err != nil {
		return err
	}
	query := accessModels.UserQuery{
		Query:            page,
		Keyword:          strings.TrimSpace(c.QueryParam("keyword")),
		PermissionStatus: accessConstants.PermissionStatusGranted,
	}
	if values, supplied := c.QueryParams()["permission_status"]; supplied {
		if len(values) != 1 || (values[0] != accessConstants.PermissionStatusGranted &&
			values[0] != accessConstants.PermissionStatusUngranted) {
			return echo.ErrBadRequest
		}
		query.PermissionStatus = values[0]
	}
	if utf8.RuneCountInString(query.Keyword) > 100 {
		return echo.ErrBadRequest
	}
	data, err := accessSvc.Users(
		c.Request().Context(),
		workspaceID,
		query,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// UserActions 读取本空间内指定用户的显式权限。
// @Summary 查询用户操作权限
// @Description 仅系统管理员可操作；不从其他工作空间或系统身份推导权限。
// @Tags 权限管理
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param user_id path int true "全局账号编号"
// @Success 200 {object} accessModels.ActionsResponse "用户操作权限及版本"
// @Router /v1/system/access/workspaces/{workspace_id}/users/{user_id}/actions [get]
func UserActions(c echo.Context) error {
	workspaceID, userID, err := permissionIDs(c)
	if err != nil {
		return err
	}
	data, err := accessSvc.UserActions(
		c.Request().Context(),
		workspaceID,
		userID,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// UpdateActions 以勾选列表完整替换本空间内指定用户的权限。
// @Summary 设置用户操作权限
// @Description 仅系统管理员可操作；仅接受登记的 workspace action，空数组撤销全部权限，未知、重复和系统操作均拒绝。
// @Tags 权限管理
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param user_id path int true "全局账号编号"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Param body body accessModels.UpdateActionsRequest true "完整 action 列表与最近读取的版本"
// @Success 200 {object} accessModels.ActionsResponse "已更新操作权限"
// @Router /v1/system/access/workspaces/{workspace_id}/users/{user_id}/actions [put]
func UpdateActions(c echo.Context) error {
	workspaceID, userID, err := permissionIDs(c)
	var request accessModels.UpdateActionsRequest
	if err != nil || accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	data, err := accessSvc.UpdateActions(
		c.Request().Context(),
		workspaceID,
		userID,
		request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// MyWorkspaces 查询当前账号获得授权的工作空间。
// @Summary 我的工作空间
// @Description 仅返回当前启用账号被显式授予 access_workspace 的启用工作空间；节点指派不授予空间发现或进入权限。
// @Tags 工作空间
// @Produce json
// @Param page query int false "页码，从 1 开始" default(1)
// @Param page_size query int false "每页数量，默认与上限来自 YAML"
// @Success 200 {object} workspaceModels.ListResponse "我的工作空间分页目录"
// @Router /v1/workspaces [get]
func MyWorkspaces(c echo.Context) error {
	page, err := pagination.Parse(c)
	if err != nil {
		return err
	}
	data, err := accessSvc.MyWorkspaces(c.Request().Context(), page)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

func permissionIDs(c echo.Context) (string, uint64, error) {
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return "", 0, err
	}
	userID, err := accessUtils.PathID(c, "user_id")
	return workspaceID, userID, err
}
