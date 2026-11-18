package monthlyworkSvc

import (
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/monthlywork/monthlyworkModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
)

func syncParticipants(
	tx *gorm.DB,
	workspaceID string,
	workID uint64,
	current, previous []uint64,
) error {
	// 只校验新指派的账号，保留后续停用或撤权的原有参与人。
	retained := make(map[uint64]bool, len(previous))
	for _, id := range previous {
		retained[id] = true
	}
	added := make([]uint64, 0, len(current))
	for _, id := range current {
		if !retained[id] {
			added = append(added, id)
		}
		delete(retained, id)
	}
	if len(added) > 0 {
		var count int64
		if err := accessUtils.WorkspaceUsers(tx, workspaceID).Where("users.id IN ?", added).
			Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(added)) {
			return invalid("参与人须为当前工作空间可用的用户")
		}
	}

	// 参与人是记录自有字段，增删明细与工作记录及审计共用事务。
	removed := make([]uint64, 0, len(retained))
	for id := range retained {
		removed = append(removed, id)
	}
	if len(removed) > 0 {
		if err := tx.Where(
			"workspace_id = ? AND monthly_work_id = ? AND user_id IN ?",
			workspaceID,
			workID,
			removed,
		).Delete(&monthlyworkModels.Participant{}).Error; err != nil {
			return writeError(err)
		}
	}
	if len(added) == 0 {
		return nil
	}
	participants := make([]monthlyworkModels.Participant, 0, len(added))
	for _, id := range added {
		participants = append(participants, monthlyworkModels.Participant{
			WorkspaceID: workspaceID, MonthlyWorkID: workID, UserID: id,
		})
	}
	return writeError(tx.Create(&participants).Error)
}

func importParticipants(tx *gorm.DB, workspaceID, value string) ([]uint64, error) {
	// 模板使用英文逗号分隔账号；不接受空项，重复账号只保存一次。
	ids := []uint64{}
	if strings.TrimSpace(value) == "" {
		return ids, nil
	}
	usernames := strings.Split(value, ",")
	if len(usernames) > config.Config.MonthlyWork.MaxOptions {
		return nil, invalid("参与人数量超过限制")
	}
	unique := make(map[string]bool, len(usernames))
	for _, username := range usernames {
		username = strings.TrimSpace(username)
		if username == "" {
			return nil, invalid("参与人账号须用英文逗号分隔，且不能包含空账号")
		}
		unique[username] = true
	}
	names := make([]string, 0, len(unique))
	for name := range unique {
		names = append(names, name)
	}
	if err := accessUtils.WorkspaceUsers(tx, workspaceID).Where("users.username IN ?", names).
		Pluck("users.id", &ids).Error; err != nil {
		return nil, err
	}
	if len(ids) != len(names) {
		return nil, invalid("参与人账号不存在、已停用或不能访问当前工作空间")
	}
	return ids, nil
}
