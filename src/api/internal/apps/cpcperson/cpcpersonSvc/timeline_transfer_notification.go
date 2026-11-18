package cpcpersonSvc

import (
	"fmt"
	"strconv"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/notification/notificationUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/workspace/workspaceModels"
)

func timelineTransferReviewers(tx *gorm.DB, workspaceID string, branchID uint64) ([]uint64, error) {
	// 审核权限与编辑支部范围分别检查，系统管理员身份不会成为隐含审批资格。
	manage := tx.Model(&accessModels.Permission{}).Select("1").
		Where("workspace_id = ? AND user_id = users.id AND action = ?",
			workspaceID, accessConstants.CPCPartyBranchUpdate)
	secretary := tx.Table("cpc_party_branches").Select("1").
		Where("workspace_id = ? AND id = ? AND secretary_id = users.id", workspaceID, branchID)
	var ids []uint64
	err := tx.Model(&userModels.User{}).
		Joins("JOIN access_permissions AS reviewer ON reviewer.user_id = users.id "+
			"AND reviewer.workspace_id = ? AND reviewer.action = ?",
			workspaceID, accessConstants.CPCPersonTimelineTransferReview).
		Where("users.is_enabled = ?", true).
		Where("EXISTS (?) OR EXISTS (?)", manage, secretary).
		Order("users.id ASC").Pluck("users.id", &ids).Error
	return ids, err
}

func notifyTimelineTransferReview(
	tx *gorm.DB, workspaceID string, review cpcpersonModels.TimelineTransferReview,
	person cpcpersonModels.Info, sourceName, targetName string,
) ([]notificationModels.Notification, error) {
	// 在发起申请的空间锁内确定全部可审批账号，避免通知其他空间或其他支部的账号。
	ids, err := timelineTransferReviewers(tx, workspaceID, person.PartyBranchID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, invalid("暂无可审核此时间线转移的启用账号，请先配置审核权限")
	}
	var workspace workspaceModels.Workspace
	if err := tx.Select("name").Where("id = ?", workspaceID).Take(&workspace).Error; err != nil {
		return nil, err
	}
	actor := auditUtils.Actor(tx.Statement.Context)
	content := fmt.Sprintf(
		"%s发起了工作空间“%s”中人员“%s(%s)”的时间线转移申请。\n"+
			"来源模板：%s\n目标模板：%s\n请前往党建人员名单的人员时间线审核。",
		actor.DisplayName, workspace.Name, person.Name, person.StudentNo, sourceName, targetName,
	)
	notifications := make([]notificationModels.Notification, 0, len(ids))
	for _, id := range ids {
		notification, created, err := notificationUtils.Create(tx, notificationModels.Delivery{
			UserID:   id,
			SourceID: fmt.Sprintf("cpc-person-timeline-transfer:%s:%d", workspaceID, review.ID),
			Title:    "时间线转移待审核",
			Content:  content,
			URL: notificationUtils.WorkspaceURL(
				cpcpersonConstants.PeoplePagePath,
				workspaceID,
				map[string]string{
					"person_id":     strconv.FormatUint(person.ID, 10),
					"timeline_mode": "review",
				},
			),
		})
		if err != nil {
			return nil, err
		}
		if created {
			notifications = append(notifications, notification)
		}
	}
	return notifications, nil
}

func notifyTimelineTransferResult(
	tx *gorm.DB, workspaceID string, review cpcpersonModels.TimelineTransferReview,
	person cpcpersonModels.Info, targetName, result, reason string,
) ([]notificationModels.Notification, error) {
	// 接收人使用申请中保存的提交人，不根据当前审核人或后续授权变化重新推导。
	var workspace workspaceModels.Workspace
	if err := tx.Select("name").Where("id = ?", workspaceID).Take(&workspace).Error; err != nil {
		return nil, err
	}
	label, description := "审核通过", "人员时间线已完成转移。"
	if result == cpcpersonConstants.TimelineTransferRejected {
		label = "已驳回"
		description = "审批意见：" + reason + "\n已保留原时间线，可调整后重新发起转移。"
	}
	actor := auditUtils.Actor(tx.Statement.Context)
	notification, created, err := notificationUtils.Create(tx, notificationModels.Delivery{
		UserID:   review.RequestedByID,
		SourceID: fmt.Sprintf("cpc-person-timeline-transfer-result:%s:%d", workspaceID, review.ID),
		Title:    "时间线转移" + label,
		URL: notificationUtils.WorkspaceURL(
			cpcpersonConstants.PeoplePagePath,
			workspaceID,
			map[string]string{"person_id": strconv.FormatUint(person.ID, 10)},
		),
		Content: fmt.Sprintf(
			"你提交的工作空间“%s”中人员“%s(%s)”的时间线转移申请%s。\n"+
				"目标模板：%s\n审核人：%s\n%s",
			workspace.Name, person.Name, person.StudentNo, label, targetName,
			actor.DisplayName, description,
		),
	})
	if err != nil {
		return nil, err
	}
	if !created {
		return nil, nil
	}
	return []notificationModels.Notification{notification}, nil
}
