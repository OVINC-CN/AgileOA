package cpcpersonRouters

import (
	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonSvc"
)

// TimelineTransferOptions 校验当前时间线并查询允许转移的模板。
// @Summary 人员时间线转移选项
// @Description 复用编辑党建人员权限和支部范围，当前时间线存在异常节点时拒绝转移。
// @Tags 党建人员时间线
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "人员编号"
// @Success 200 {object} cpcpersonModels.TimelineTransferOptionsResponse
// @Failure 400 {object} accessModels.EmptyResponse "未绑定时间线或当前节点存在时间异常"
// @Failure 409 {object} accessModels.EmptyResponse "时间线已在转移中"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/transfer-options [get]
func TimelineTransferOptions(c echo.Context) error {
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.TimelineTransferOptions(
			c.Request().Context(), workspaceID, id,
		)
	})
}

// PreviewTimelineTransfer 按已配置的节点和字段映射生成完整预览。
// @Summary 预览人员时间线转移
// @Description 复用编辑权限；要求来源时间线无时间异常，展示目标节点、引用值、规则、人员附件及未映射字段。目标时间异常仅提示，不写入时间线。
// @Tags 党建人员时间线
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "人员编号"
// @Param body body cpcpersonModels.TimelineTransferRequest true "目标模板编号"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} cpcpersonModels.TimelineTransferPreviewResponse
// @Failure 400 {object} accessModels.EmptyResponse "来源节点异常、目标未配置或映射无效"
// @Failure 409 {object} accessModels.EmptyResponse "时间线已在转移中"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/transfer-preview [post]
func PreviewTimelineTransfer(c echo.Context) error { return transferTimeline(c, false) }

// TransferTimeline 发起待审核转移，保留原时间线并锁定人员、节点和附件编辑。
// @Summary 发起人员时间线转移
// @Description 要求编辑权限及来源时间线无时间异常，目标时间异常不阻止发起，复核预览版本。
// @Description 保存独立审核申请、置为 transferring 并通知有权限及支部范围的启用审批人，业务与通知及审计同事务。
// @Tags 党建人员时间线
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "人员编号"
// @Param body body cpcpersonModels.TimelineTransferRequest true "目标模板编号及预览版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} cpcpersonModels.TimelineResponse
// @Failure 400 {object} accessModels.EmptyResponse "来源节点异常、目标未配置、映射无效或无可用审批人"
// @Failure 409 {object} accessModels.EmptyResponse "预览后数据或模板发生变化"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/transfer [post]
func TransferTimeline(c echo.Context) error { return transferTimeline(c, true) }

// ReviewTimeline 按独立审核权限读取人员时间线。
// @Summary 人员时间线审核详情
// @Description 不要求人员编辑权限，按审核支部范围读取时间线、节点及附件元数据，完成或驳回后可读取最新状态。
// @Tags 党建人员时间线
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "人员编号"
// @Success 200 {object} cpcpersonModels.TimelineResponse
// @Router /v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/review [get]
func ReviewTimeline(c echo.Context) error {
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.TimelineForReview(
			c.Request().Context(), workspaceID, id,
		)
	})
}

// PreviewTimelineTransferReview 读取待审核目标的最新完整预览。
// @Summary 预览待审核时间线转移
// @Description 仅允许审核时间线转移权限，固定使用已发起的目标模板；重新校验来源时间线无时间异常，目标时间异常仅提示。
// @Tags 党建人员时间线
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "人员编号"
// @Success 200 {object} cpcpersonModels.TimelineTransferPreviewResponse
// @Failure 400 {object} accessModels.EmptyResponse "无待审核转移、来源节点异常或映射无效"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/transfer-review [get]
func PreviewTimelineTransferReview(c echo.Context) error {
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.PreviewTimelineTransferReview(
			c.Request().Context(), workspaceID, id,
		)
	})
}

// ReviewTimelineTransfer 审核通过后原子重建人员时间线并解除转移锁定。
// @Summary 审核并完成人员时间线转移
// @Description 要求独立审核权限、来源时间线无时间异常且目标与申请一致，复核预览版本；目标时间异常不阻止通过，完成后保存实际状态，转移、结果通知与审计同事务。
// @Tags 党建人员时间线
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "人员编号"
// @Param body body cpcpersonModels.TimelineTransferRequest true "待审核目标及最新预览版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} cpcpersonModels.TimelineResponse
// @Failure 400 {object} accessModels.EmptyResponse "无待审核转移、来源节点异常或映射无效"
// @Failure 409 {object} accessModels.EmptyResponse "预览后数据发生变化"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/transfer-review [post]
func ReviewTimelineTransfer(c echo.Context) error {
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	var request cpcpersonModels.TimelineTransferRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.ReviewTimelineTransfer(
			c.Request().Context(), workspaceID, id, request,
		)
	})
}

// CancelTimelineTransfer 取消待审核转移并恢复原时间线状态。
// @Summary 取消人员时间线转移
// @Description 要求人员编辑权限及支部范围，校验当前人员版本，不改变原节点、填写值或附件。
// @Tags 党建人员时间线
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "人员编号"
// @Param body body cpcpersonModels.TimelineTransferRevisionRequest true "当前人员版本"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} cpcpersonModels.TimelineResponse
// @Failure 400 {object} accessModels.EmptyResponse "无待取消的转移"
// @Failure 409 {object} accessModels.EmptyResponse "人员版本已发生变化"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/transfer-cancel [post]
func CancelTimelineTransfer(c echo.Context) error {
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	var request cpcpersonModels.TimelineTransferRevisionRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.CancelTimelineTransfer(
			c.Request().Context(), workspaceID, id, request,
		)
	})
}

// RejectTimelineTransfer 按独立审核权限驳回申请，保留原时间线并通知提交人。
// @Summary 驳回人员时间线转移
// @Description 校验待审核申请及人员版本，解除转移锁定，结果通知、业务及审计同事务。
// @Description 必须填写审批意见，去除首尾空白后最多 1000 字符，随结果通知正文发送给提交人。
// @Tags 党建人员时间线
// @Accept json
// @Produce json
// @Param workspace_id path string true "工作空间编号"
// @Param id path int true "人员编号"
// @Param body body cpcpersonModels.TimelineTransferRejectRequest true "当前人员版本及审批意见"
// @Param X-CSRF-Protection header string true "固定为 1" default(1)
// @Success 200 {object} cpcpersonModels.TimelineResponse
// @Failure 400 {object} accessModels.EmptyResponse "无待审核的转移或审批意见无效"
// @Failure 409 {object} accessModels.EmptyResponse "人员版本已发生变化"
// @Router /v1/workspaces/{workspace_id}/cpc-people/{id}/timeline/transfer-reject [post]
func RejectTimelineTransfer(c echo.Context) error {
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	var request cpcpersonModels.TimelineTransferRejectRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	return handle(c, func(workspaceID string) (any, error) {
		return cpcpersonSvc.RejectTimelineTransfer(
			c.Request().Context(), workspaceID, id, request,
		)
	})
}

func transferTimeline(c echo.Context, confirmed bool) error {
	id, err := accessUtils.PathID(c, "id")
	if err != nil {
		return err
	}
	var request cpcpersonModels.TimelineTransferRequest
	if accessUtils.Decode(c, &request) != nil {
		return echo.ErrBadRequest
	}
	return handle(c, func(workspaceID string) (any, error) {
		if confirmed {
			return cpcpersonSvc.TransferTimeline(
				c.Request().Context(), workspaceID, id, request,
			)
		}
		return cpcpersonSvc.PreviewTimelineTransfer(
			c.Request().Context(), workspaceID, id, request,
		)
	})
}
