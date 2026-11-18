package projectSvc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
)

// NotifyRankingFailure 独立重试并向当前空间拥有奖项编辑权限的启用账号幂等通知。
func NotifyRankingFailure(ctx context.Context, value projectModels.RankingPayload) error {
	if db.DB() == nil {
		return errors.New("database is unavailable")
	}
	var notifications []notificationModels.Notification
	err := db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 与授权变更共用空间锁，接收人使用投递时的显式权限，不依赖触发人身份。
		workspace, err := accessUtils.LockWorkspace(tx, value.WorkspaceID)
		if rankingTargetMissing(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !workspace.IsEnabled {
			return nil
		}
		var project projectModels.Project
		if err := tx.Select("name", "year").
			Where("workspace_id = ? AND id = ?", value.WorkspaceID, value.ProjectID).
			Take(&project).Error; err != nil {
			if rankingTargetMissing(err) {
				return nil
			}
			return err
		}
		var recipients []uint64
		// 固定从空间授权索引驱动，避免少量账号时优化器改为全表扫描用户。
		if err := tx.Table("access_permissions AS p").Select("p.user_id").
			Joins("STRAIGHT_JOIN users AS u FORCE INDEX (PRIMARY) ON u.id = p.user_id").
			Where("p.workspace_id = ? AND p.action = ? AND u.is_enabled = ?",
				value.WorkspaceID, accessConstants.ProjectUpdate, true).
			Order("p.user_id ASC").Scan(&recipients).Error; err != nil {
			return err
		}
		for _, userID := range recipients {
			notice, created, err := notificationUtils.Create(tx, notificationModels.Delivery{
				UserID: userID, SourceID: value.SourceID,
				Title: "奖项排名失败",
				Content: fmt.Sprintf(
					"工作空间“%s”的 %d 年“%s”奖项排名失败，已保留原排名。请检查分数计算和排名配置，重新保存奖项后重试。",
					workspace.Name, project.Year, project.Name,
				),
				URL: notificationUtils.WorkspaceURL("/stu/projects", value.WorkspaceID,
					map[string]string{"tab": "projects"}),
			})
			if err != nil {
				return err
			}
			if created {
				notifications = append(notifications, notice)
			}
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}

	// 通知与接收人快照提交后记录成功日志，不写用户操作审计。
	notificationUtils.LogCreated(ctx, notifications)
	return nil
}
