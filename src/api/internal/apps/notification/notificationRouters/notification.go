package notificationRouters

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/notification/notificationSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// List 返回当前账号的个人通知，不能指定接收人或工作空间。
// @Summary 查看我的通知
// @Description 未读优先，组内按编号倒序；读取不会自动标为已读。
// @Tags 通知
// @Produce json
// @Param page query int false "页码，从 1 开始"
// @Param page_size query int false "每页条数，不超过 YAML 配置上限"
// @Param status query string false "通知范围，默认仅查询未读通知" Enums(unread,all) default(unread)
// @Success 200 {object} notificationModels.ListResponse
// @Failure 400 {object} notificationModels.EmptyResponse "分页参数或通知范围无效"
// @Router /v1/notifications [get]
func List(c echo.Context) error {
	// 校验分页，账号及接收人范围由业务事务复核。
	query, err := pagination.Parse(c)
	if err != nil {
		return err
	}
	data, err := notificationSvc.List(
		c.Request().
			Context(),
		notificationModels.ListQuery{Query: query, Status: c.QueryParam("status")},
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Unread 返回当前账号的未读通知总数。
// @Summary 查看我的未读通知数量
// @Tags 通知
// @Produce json
// @Success 200 {object} notificationModels.UnreadResponse
// @Router /v1/notifications/unread [get]
func Unread(c echo.Context) error {
	// 接收人来自会话，不接受请求指定的账号编号。
	data, err := notificationSvc.Unread(c.Request().Context())
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// MarkRead 将指定的个人通知标为已读。
// @Summary 标记我的通知为已读
// @Description 整批通知必须全部属于当前账号，任一编号无效时不修改任何通知；重复提交幂等。
// @Tags 通知
// @Accept json
// @Produce json
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Param body body notificationModels.ReadRequest true "待标记的通知编号"
// @Success 200 {object} notificationModels.ReadResponse
// @Failure 400 {object} notificationModels.EmptyResponse "编号为空、重复或超过单次上限"
// @Failure 404 {object} notificationModels.EmptyResponse "通知不存在或不可访问，整批不修改"
// @Router /v1/notifications/read [post]
func MarkRead(c echo.Context) error {
	// 提取并校验请求字段，数量及整批归属由业务事务校验。
	var request notificationModels.ReadRequest
	if err := accessUtils.Decode(c, &request); err != nil {
		return echo.ErrBadRequest
	}
	data, err := notificationSvc.MarkRead(
		c.Request().Context(), request,
	)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}
