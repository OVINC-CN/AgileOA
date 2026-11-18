package accessSvc

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// Users 由系统管理员按当前空间的有效显式授权筛选账号，并在分页前统计数量。
func Users(
	ctx context.Context,
	workspaceID string,
	query accessModels.UserQuery,
) (accessModels.UserListData, error) {
	// 省略状态时仅列出已授权账号，候选查询显式选择未授权状态。
	if query.PermissionStatus == "" {
		query.PermissionStatus = accessConstants.PermissionStatusGranted
	}
	result := accessModels.UserListData{
		Items: []accessModels.UserInfo{}, Page: query.Page, PageSize: query.PageSize,
	}
	event := permissionEvent(workspaceID, 0)
	event.Conditions["keyword"] = query.Keyword
	event.Conditions["permission_status"] = query.PermissionStatus
	event.Conditions["page"] = query.Page
	event.Conditions["page_size"] = query.PageSize
	err := accessUtils.System(ctx, accessConstants.AccessUserList, event,
		func(tx *gorm.DB, _ accessModels.Principal, event *auditModels.Event) error {
			// 仅统计仍在目录中的工作空间操作，其他空间及失效标识不参与归类。
			actions := make([]string, 0)
			for _, action := range accessConstants.Catalog() {
				if action.Scope == accessConstants.ScopeWorkspace {
					actions = append(actions, action.Value)
				}
			}
			permissions := tx.Model(&accessModels.Permission{}).
				Where("workspace_id = ? AND action IN ?", workspaceID, actions)
			scope := tx.Model(&userModels.User{})
			countField := "grants.permission_count"
			if query.PermissionStatus == accessConstants.PermissionStatusUngranted {
				scope = scope.Where("users.id NOT IN (?)", permissions.Select("user_id"))
				countField = "0 AS permission_count"
			} else {
				scope = scope.Joins(
					"JOIN (?) AS grants ON grants.user_id = users.id",
					permissions.Select("user_id, COUNT(*) AS permission_count").Group("user_id"),
				)
			}
			if query.Keyword != "" {
				scope = scope.Where(
					"users.username LIKE ? OR users.display_name LIKE ?",
					"%"+query.Keyword+"%",
					"%"+query.Keyword+"%",
				)
			}
			if err := scope.Count(&result.Total).Error; err != nil {
				return err
			}
			// 先从覆盖索引选取本页编号，再按主键回读固定字段，避免候选列表扫描整张账号表。
			page := scope.Select("users.id, " + countField).Order("users.id DESC").
				Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize)
			if err := tx.Table("(?) AS matched", page).
				Joins("JOIN users ON users.id = matched.id").
				Select(
					"users.id, users.username, users.display_name, users.is_enabled, " +
						"users.created_at, users.updated_at, matched.permission_count",
				).Order("users.id DESC").
				Scan(&result.Items).Error; err != nil {
				return err
			}
			// 分页结果仅包含固定账号目录与同一查询计算的授权数量。
			for _, user := range result.Items {
				event.ReturnedIDs = append(event.ReturnedIDs, strconv.FormatUint(user.ID, 10))
			}
			event.ReturnedCount = int64(len(result.Items))
			return nil
		})
	return result, err
}

func targetUser(tx *gorm.DB, id uint64) error {
	var user userModels.User
	if err := tx.Select("id").Where("id = ?", id).Take(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &response.BusinessError{Status: http.StatusNotFound, Message: "用户不存在"}
		}
		return err
	}
	return nil
}

func permissionEvent(workspaceID string, userID uint64) auditModels.Event {
	return auditModels.Event{
		TenantID: &workspaceID, ObjectType: auditConstants.ObjectPermission,
		ObjectID:   strconv.FormatUint(userID, 10),
		Conditions: auditModels.Snapshot{"workspace_id": workspaceID, "user_id": userID},
	}
}

func invalid() error {
	return &response.BusinessError{Status: http.StatusBadRequest, Message: "用户或授权参数无效"}
}

func stale() error {
	return &response.BusinessError{Status: http.StatusConflict, Message: "工作空间或权限已被修改，请刷新后重试"}
}
