package accessUtils

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/workspace/workspaceModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/tracing"
)

// System 复核系统管理员身份，在工作空间锁内执行管理操作并同事务写审计。
func System(
	ctx context.Context,
	action string,
	event auditModels.Event,
	operation func(*gorm.DB, accessModels.Principal, *auditModels.Event) error,
) error {
	return run(ctx, action, accessConstants.ScopeSystem, event, operation)
}

// Session 仅用于代码明确登记的当前账号自助查询，不授予租户业务权限。
func Session(
	ctx context.Context,
	action string,
	event auditModels.Event,
	operation func(*gorm.DB, accessModels.Principal, *auditModels.Event) error,
) error {
	return run(ctx, action, accessConstants.ScopeSession, event, operation)
}

// Workspace 在业务事务内锁定工作空间，实时复核账号、用户权限及唯一指定的 action。
func Workspace(
	ctx context.Context,
	workspaceID string,
	action string,
	event auditModels.Event,
	operation func(*gorm.DB, accessModels.Principal, *auditModels.Event) error,
) error {
	event.TenantID = &workspaceID
	return run(ctx, action, accessConstants.ScopeWorkspace, event, operation)
}

func run(
	ctx context.Context,
	action, scope string,
	event auditModels.Event,
	operation func(*gorm.DB, accessModels.Principal, *auditModels.Event) error,
) error {
	// 调用方不能通过选择辅助函数绕过操作目录所声明的范围。
	definition, ok := accessConstants.Lookup(action)
	if !ok || definition.Scope != scope {
		return Denied()
	}
	ctx, span := tracing.Tracer.Start(ctx, action)
	defer span.End()
	if db.DB() == nil {
		return unavailable()
	}
	err := db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 查询共用读锁，写操作保持排他锁；顺序统一为账号、空间、权限。
		user, err := userUtils.Require(tx, action)
		if err != nil {
			return err
		}
		principal := accessModels.Principal{User: user}
		if event.TenantID != nil {
			principal.Workspace, err = lockWorkspace(
				tx,
				*event.TenantID,
				accessConstants.ReadOnly(action),
			)
			if err != nil {
				return err
			}
		}
		if scope == accessConstants.ScopeWorkspace {
			if event.TenantID == nil {
				return Denied()
			}
			if err := requireAction(tx, &principal, action); err != nil {
				return err
			}
		}

		// 业务与成功审计共用此事务；审计失败时连同业务一并回滚。
		if err := operation(tx, principal, &event); err != nil {
			return err
		}
		event.Actor = auditUtils.Actor(ctx)
		event.Action = action
		event.Result = auditConstants.Success
		return auditSvc.Write(tx, event)
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Reject(ctx, action, event, err)
	}
	return nil
}

// Check 供 HTTP 入口默认拒绝未授权操作；业务仍须使用 Workspace 在事务内再次复核。
func Check(ctx context.Context, action string, workspaceID string) error {
	definition, ok := accessConstants.Lookup(action)
	if !ok {
		return Denied()
	}
	if definition.Scope == accessConstants.ScopePublic {
		return nil
	}
	if db.DB() == nil {
		return unavailable()
	}
	event := auditModels.Event{ObjectType: definition.Group.Value}
	if workspaceID != "" {
		event.TenantID = &workspaceID
	}
	// 中间件复用刚确认的身份，系统和会话接口不再次查询账号。
	identity, err := userUtils.Identity(ctx)
	if err == nil && identity.Student != nil {
		// 学生服务只依赖登录身份及绑定空间，不读取或创建 action 授权。
		if action == accessConstants.UserInfo || action == accessConstants.UserLogout ||
			(accessConstants.StudentServiceAction(action) &&
				workspaceID == identity.Student.WorkspaceID) {
			return nil
		}
		return Reject(ctx, action, event, Denied())
	}
	if err == nil {
		if accessConstants.StudentServiceAction(action) {
			return Reject(ctx, action, event, Denied())
		}
		err = userUtils.CheckAction(identity, action)
	}
	if err != nil {
		return Reject(ctx, action, event, err)
	}
	if definition.Scope != accessConstants.ScopeWorkspace {
		if definition.Scope != accessConstants.ScopeSystem &&
			definition.Scope != accessConstants.ScopeSession {
			return Denied()
		}
		return nil
	}
	// 入口使用单条一致性读取预检，避免重复开启锁事务；业务事务仍实时复核。
	err = checkWorkspace(db.DB().WithContext(ctx), identity.User.ID, workspaceID, action)
	if err != nil {
		return Reject(ctx, action, event, err)
	}
	return nil
}

func checkWorkspace(tx *gorm.DB, userID uint64, workspaceID, action string) error {
	permission := tx.Model(&accessModels.Permission{}).Select("1").
		Where("user_id = ? AND action = ? AND workspace_id = ?", userID, action, workspaceID)
	var current struct {
		IsEnabled bool
		Permitted bool
	}
	err := tx.Model(&workspaceModels.Workspace{}).
		Select("is_enabled, EXISTS (?) AS permitted", permission).
		Where("id = ?", workspaceID).
		Take(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return workspaceMissing()
	}
	if err != nil {
		return err
	}
	if !accessConstants.Grantable(action) || !current.IsEnabled || !current.Permitted {
		return Denied()
	}
	return nil
}

// LockWorkspace 在账号锁之后锁定租户边界，管理变更和业务操作共用该锁。
func LockWorkspace(tx *gorm.DB, id string) (workspaceModels.Workspace, error) {
	return lockWorkspace(tx, id, false)
}

// ReadWorkspace 共享锁定租户边界，允许并行查询并阻止管理和业务写入。
func ReadWorkspace(tx *gorm.DB, id string) (workspaceModels.Workspace, error) {
	return lockWorkspace(tx, id, true)
}

func lockWorkspace(tx *gorm.DB, id string, readOnly bool) (workspaceModels.Workspace, error) {
	strength := "UPDATE"
	if readOnly {
		strength = "SHARE"
	}
	var workspace workspaceModels.Workspace
	err := tx.Clauses(clause.Locking{Strength: strength}).
		Where("id = ?", id).
		Take(&workspace).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return workspace, workspaceMissing()
	}
	return workspace, err
}

func workspaceMissing() error {
	return &response.BusinessError{
		Status:  http.StatusNotFound,
		Message: "工作空间不存在或不可访问",
	}
}

// Require 在现有工作空间锁内检查附加的隐私操作，不推导其他权限。
func Require(tx *gorm.DB, principal accessModels.Principal, actions ...string) error {
	for _, action := range actions {
		if err := requireAction(tx, &principal, action); err != nil {
			return err
		}
	}
	return nil
}

func requireAction(tx *gorm.DB, principal *accessModels.Principal, action string) error {
	// 系统管理员必须具备与普通用户相同的显式授权。
	if !accessConstants.Grantable(action) || !principal.Workspace.IsEnabled {
		return Denied()
	}
	var permission accessModels.Permission
	if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).
		Where(
			"user_id = ? AND action = ? AND workspace_id = ?",
			principal.User.ID,
			action,
			principal.Workspace.ID,
		).
		Take(&permission).
		Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Denied()
		}
		return err
	}
	return nil
}

// Actions 只返回用户在当前空间内有效且已登记的权限。
func Actions(tx *gorm.DB, userID uint64, workspaceID string) ([]string, error) {
	var permissions []accessModels.Permission
	if err := tx.Where("user_id = ? AND workspace_id = ?", userID, workspaceID).
		Order("action ASC").Find(&permissions).Error; err != nil {
		return nil, err
	}
	actions := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		if accessConstants.Grantable(permission.Action) {
			actions = append(actions, permission.Action)
		}
	}
	return actions, nil
}

// Reject 对已确认身份的失败操作独立审计，不保存失败事务的业务快照。
func Reject(ctx context.Context, action string, event auditModels.Event, cause error) error {
	// 在转换公开错误和尝试失败审计前保留原始异常。
	response.RecordError(ctx, cause)
	failure := &response.BusinessError{
		Status:  http.StatusServiceUnavailable,
		Message: "暂时无法完成操作，请稍后重试",
	}
	var business *response.BusinessError
	if errors.As(cause, &business) {
		failure = business
	}
	if errors.Is(cause, auditSvc.ErrUnavailable) {
		failure = &response.BusinessError{
			Status:  http.StatusServiceUnavailable,
			Message: "暂时无法完成操作，请稍后重试",
		}
	}
	if event.ObjectType == "" {
		event.ObjectType = auditConstants.ObjectWorkspace
	}
	event.Actor = auditUtils.Actor(ctx)
	event.Action = action
	event.Before, event.After = nil, nil
	event.SensitiveBefore, event.SensitiveAfter = nil, nil
	event.ReturnedIDs, event.ReturnedCount = nil, 0
	event.ErrorCode = "http_" + strconv.Itoa(failure.Status)
	event.ErrorMessage = failure.Message
	if err := auditSvc.Failure(ctx, event); err != nil {
		response.RecordError(ctx, err)
		return &response.BusinessError{
			Status:  http.StatusServiceUnavailable,
			Message: "暂时无法完成操作，请稍后重试",
		}
	}
	return failure
}

// Denied 返回固定的简体中文拒绝说明，不暴露内部授权状态。
func Denied() error {
	return &response.BusinessError{Status: http.StatusForbidden, Message: "没有此工作空间的操作权限"}
}

func unavailable() error {
	return &response.BusinessError{Status: http.StatusServiceUnavailable, Message: "暂时无法完成操作，请稍后重试"}
}
