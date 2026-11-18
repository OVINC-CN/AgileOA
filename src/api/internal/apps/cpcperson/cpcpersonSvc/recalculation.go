package cpcpersonSvc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
)

// ErrUnmatched 表示至少一名人员无法分配，整批保留原归属，等待规则修正后重试。
var ErrUnmatched = errors.New("people have no matching party branch")

// branchRecalculation 保存空间锁内读取的人员及调整前后党支部简称快照。
type branchRecalculation struct {
	ID, PartyBranchID    uint64
	StudentNo, Name      string
	PartyBranchShortName string
	NewBranchID          *uint64
	NewBranchShortName   string
	Revision             uint64
}

// Recalculate 由后台任务原子重算当前空间在编人员，不因触发账号的后续权限变更跳过处理。
func Recalculate(
	ctx context.Context,
	workspaceID string,
	recipientID uint64,
	notificationSource string,
) error {
	if db.DB() == nil {
		return errors.New("database is unavailable")
	}
	var changes []branchRecalculation
	var notifications []notificationModels.Notification
	err := db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		changes, notifications, err = recalculateBranches(
			tx, workspaceID, recipientID, notificationSource,
		)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}

	// 事务提交后逐人记录真实变更，保留当时的学号、姓名和党支部简称。
	for _, change := range changes {
		logger.Logger.WithContext(ctx).Infof(
			"student: %s %s; workspace: %s; person: %d; operation: recalculate_branches\n"+
				"before: %d %s\nafter: %d %s",
			change.StudentNo,
			change.Name,
			workspaceID,
			change.ID,
			change.PartyBranchID,
			change.PartyBranchShortName,
			*change.NewBranchID,
			change.NewBranchShortName,
		)
	}
	notificationUtils.LogCreated(ctx, notifications)
	return nil
}

func recalculateBranches(
	tx *gorm.DB,
	workspaceID string,
	recipientID uint64,
	notificationSource string,
) ([]branchRecalculation, []notificationModels.Notification, error) {
	// 与规则、学生资料、书记和授权修改共用工作空间锁。
	workspace, err := accessUtils.LockWorkspace(tx, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	if !workspace.IsEnabled {
		return nil, nil, errors.New("workspace is disabled")
	}
	expression, err := matchingExpression(tx, workspaceID)
	if err != nil {
		return nil, nil, err
	}

	// 仅重排党建人员状态为在编的记录，左连接保留未匹配人员供整批校验。
	matched := tx.Table("cpc_people AS p").
		Select("p.id, p.workspace_id, p.party_branch_id, p.revision, s.student_no, s.name, "+
			"? AS new_branch_id", expression).
		Joins("JOIN students AS s ON s.workspace_id = p.workspace_id AND s.id = p.student_id").
		Where("p.workspace_id = ?", workspaceID).
		Where("p.status = ?", cpcpersonConstants.DefaultStatus)
	var rows []branchRecalculation
	if err := tx.Table("(?) AS matched", matched).
		Select("matched.id, matched.party_branch_id, matched.revision, " +
			"matched.student_no, matched.name, matched.new_branch_id, " +
			"COALESCE(previous_branch.short_name, '-') AS party_branch_short_name, " +
			"next_branch.short_name AS new_branch_short_name").
		Joins("LEFT JOIN cpc_party_branches AS previous_branch " +
			"ON previous_branch.workspace_id = matched.workspace_id " +
			"AND previous_branch.id = matched.party_branch_id").
		Joins("LEFT JOIN cpc_party_branches AS next_branch " +
			"ON next_branch.workspace_id = matched.workspace_id " +
			"AND next_branch.id = matched.new_branch_id").
		Order("matched.id ASC").Find(&rows).Error; err != nil {
		return nil, nil, err
	}

	// 先核对全部在编人员，不允许在发现未匹配人员之前部分写入。
	unmatched := []uint64{}
	for _, row := range rows {
		if row.NewBranchID == nil {
			unmatched = append(unmatched, row.ID)
		}
	}
	if len(unmatched) > 0 {
		return nil, nil, fmt.Errorf(
			"%w; workspace: %s; people: %v", ErrUnmatched, workspaceID, unmatched,
		)
	}
	var changes []branchRecalculation
	var notifications []notificationModels.Notification
	for _, row := range rows {
		if row.PartyBranchID == *row.NewBranchID {
			continue
		}
		changed := tx.Model(&cpcpersonModels.Person{}).
			Where("workspace_id = ? AND id = ? AND revision = ?", workspaceID, row.ID, row.Revision).
			Updates(map[string]any{
				"party_branch_id": *row.NewBranchID, "revision": row.Revision + 1,
			})
		if changed.Error != nil {
			return nil, nil, changed.Error
		}
		if changed.RowsAffected != 1 {
			return nil, nil, errors.New("person revision changed during branch recalculation")
		}
		changes = append(changes, row)

		// 明细通知与归属变更同事务保存，接收人不作为后台用户审计身份。
		// 历史任务缺少接收人或来源时仍完成重算，不推导接收人。
		if recipientID == 0 || notificationSource == "" {
			continue
		}
		notification, created, err := notificationUtils.Create(tx, notificationModels.Delivery{
			UserID:   recipientID,
			SourceID: fmt.Sprintf("%s:person:%d", notificationSource, row.ID),
			Title:    "人员党支部调整",
			URL: notificationUtils.WorkspaceURL(
				cpcpersonConstants.PeoplePagePath, workspaceID, nil,
			),
			Content: fmt.Sprintf(
				"工作空间“%s”的人员党支部已调整。\n学号：%s\n姓名：%s"+
					"\n调整前党支部：%s\n调整后党支部：%s",
				workspace.Name,
				row.StudentNo,
				row.Name,
				row.PartyBranchShortName,
				row.NewBranchShortName,
			),
		})
		if err != nil {
			return nil, nil, err
		}
		if created {
			notifications = append(notifications, notification)
		}
	}
	return changes, notifications, nil
}
