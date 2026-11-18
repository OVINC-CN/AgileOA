package notificationRouters

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// StudentList 查看当前学生在绑定空间内的通知，不接受指定接收人。
// @Summary 查看我的学生通知
// @Description 仅限绑定空间的学生会话，未读优先、组内编号倒序，读取不自动标为已读。
// @Tags 学生通知
// @Produce json
// @Param workspace_id path string true "绑定的工作空间编号"
// @Param page query int false "页码，从 1 开始" default(1)
// @Param page_size query int false "每页条数，不超过 YAML 配置上限"
// @Param status query string false "通知范围" Enums(unread,all) default(unread)
// @Success 200 {object} notificationModels.ListResponse "审计提交后返回"
// @Failure 400 {object} notificationModels.EmptyResponse "分页参数或通知范围无效"
// @Failure 401 {object} notificationModels.EmptyResponse "学生登录已失效"
// @Failure 403 {object} notificationModels.EmptyResponse "不是绑定空间的学生会话"
// @Failure 503 {object} notificationModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/notifications [get]
func StudentList(c echo.Context) error {
	// 校验空间及分页，接收人从事务内复核的学生身份读取。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	query, err := pagination.Parse(c)
	if err != nil {
		return err
	}
	data, err := notificationSvc.StudentList(c.Request().Context(), workspaceID,
		notificationModels.ListQuery{Query: query, Status: c.QueryParam("status")})
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// StudentUnread 返回绑定空间内当前学生的未读数量。
// @Summary 查看我的学生未读通知数量
// @Tags 学生通知
// @Produce json
// @Param workspace_id path string true "绑定的工作空间编号"
// @Success 200 {object} notificationModels.UnreadResponse
// @Failure 400 {object} notificationModels.EmptyResponse "空间参数无效"
// @Failure 401 {object} notificationModels.EmptyResponse "学生登录已失效"
// @Failure 403 {object} notificationModels.EmptyResponse "不是绑定空间的学生会话"
// @Failure 503 {object} notificationModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/notifications/unread [get]
func StudentUnread(c echo.Context) error {
	// 只接收空间参数，不接收学生编号。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	data, err := notificationSvc.StudentUnread(c.Request().Context(), workspaceID)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// StudentMarkRead 将当前学生明确指定的通知整批标为已读。
// @Summary 标记我的学生通知为已读
// @Description 任一通知不属于绑定空间内的当前学生时整批不修改，重复提交保留首次已读时间。
// @Tags 学生通知
// @Accept json
// @Produce json
// @Param workspace_id path string true "绑定的工作空间编号"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Param body body notificationModels.ReadRequest true "待标记的通知编号"
// @Success 200 {object} notificationModels.ReadResponse "变更与审计同事务提交"
// @Failure 400 {object} notificationModels.EmptyResponse "编号为空、重复或超过单次上限"
// @Failure 401 {object} notificationModels.EmptyResponse "学生登录已失效"
// @Failure 403 {object} notificationModels.EmptyResponse "不是绑定空间的学生会话"
// @Failure 404 {object} notificationModels.EmptyResponse "通知不存在或不可访问"
// @Failure 503 {object} notificationModels.EmptyResponse "服务暂时不可用"
// @Router /v1/workspaces/{workspace_id}/student-service/notifications/read [post]
func StudentMarkRead(c echo.Context) error {
	// 提取请求字段，数量及整批归属由业务事务校验。
	workspaceID, err := accessUtils.WorkspaceID(c)
	if err != nil {
		return err
	}
	var request notificationModels.ReadRequest
	if err := accessUtils.Decode(c, &request); err != nil {
		return echo.ErrBadRequest
	}
	data, err := notificationSvc.StudentMarkRead(c.Request().Context(), workspaceID, request)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}
