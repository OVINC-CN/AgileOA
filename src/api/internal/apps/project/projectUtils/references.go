package projectUtils

import (
	"net/http"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// GuardStudentDeletion 显式保护申报人及流程指派，不能借级联删除使审批跳级。
func GuardStudentDeletion(tx *gorm.DB, workspaceID string, studentID uint64) error {
	queries := []*gorm.DB{
		tx.Model(&projectModels.Application{}),
		tx.Model(&projectModels.ApprovalStep{}),
		tx.Model(&projectModels.ApplicationNode{}),
		tx.Model(&projectModels.ApplicationApproval{}),
	}
	for _, query := range queries {
		var count int64
		if err := query.Where("workspace_id = ? AND student_id = ?", workspaceID, studentID).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return referenced("该学生仍有关联申报或审批节点，不能删除")
		}
	}
	return nil
}

// GuardUserDeletion 系统删除账号前检查全部空间的流程引用，历史身份快照不阻止删除。
func GuardUserDeletion(tx *gorm.DB, userID uint64) error {
	for _, query := range []*gorm.DB{
		tx.Model(&projectModels.ApprovalStep{}), tx.Model(&projectModels.ApplicationNode{}),
		tx.Model(&projectModels.ApplicationApproval{}),
	} {
		var count int64
		if err := query.Where("user_id = ?", userID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return referenced("该用户仍被评奖评优审批流引用，不能删除")
		}
	}
	return nil
}

func referenced(message string) error {
	return &response.BusinessError{Status: http.StatusConflict, Message: message}
}
