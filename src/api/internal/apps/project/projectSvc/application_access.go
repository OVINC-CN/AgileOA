package projectSvc

import (
	"context"
	"slices"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userUtils"
)

type applicationActor struct {
	StudentID uint64
	UserID    uint64
	Name      string
	ClassName string
	Positions []string
	Actions   []string
}

func applicationAction(student bool, mode, operation string) string {
	if !student {
		switch operation {
		case projectConstants.ApplicationUpdate:
			return accessConstants.ProjectReviewUpdate
		case projectConstants.ApplicationApprove:
			return accessConstants.ProjectReviewApprove
		case projectConstants.ApplicationReject:
			return accessConstants.ProjectReviewReject
		default:
			if mode == projectConstants.ApplicationManage {
				return accessConstants.ProjectApplicationList
			}
			return accessConstants.ProjectApplicationList
		}
	}
	if mode == projectConstants.ApplicationMine {
		switch operation {
		case projectConstants.ApplicationUpdate:
			return accessConstants.StudentApplicationUpdate
		case projectConstants.ApplicationSubmit:
			return accessConstants.StudentApplicationSubmit
		default:
			return accessConstants.StudentApplicationList
		}
	}
	switch operation {
	case projectConstants.ApplicationUpdate:
		return accessConstants.StudentReviewUpdate
	case projectConstants.ApplicationApprove:
		return accessConstants.StudentReviewApprove
	case projectConstants.ApplicationReject:
		return accessConstants.StudentReviewReject
	default:
		return accessConstants.StudentReviewList
	}
}

func applicationAccess(ctx context.Context, workspaceID, action string, audit auditModels.Event,
	operation func(*gorm.DB, applicationActor, *auditModels.Event) error,
) error {
	identity, err := userUtils.Identity(ctx)
	if err != nil {
		return accessUtils.Reject(ctx, action, audit, err)
	}
	if identity.Student != nil {
		return accessUtils.StudentWorkspace(ctx, workspaceID, action, audit,
			func(tx *gorm.DB, student userModels.StudentIdentity, event *auditModels.Event) error {
				var record struct{ ClassName, ClassPosition string }
				if err := tx.Table("students").Select("class_name", "class_position").
					Where("workspace_id = ? AND id = ?", workspaceID, student.StudentID).
					Take(&record).Error; err != nil {
					return err
				}
				return operation(
					tx,
					applicationActor{
						StudentID: student.StudentID,
						Name:      student.Name,
						ClassName: record.ClassName,
						Positions: projectUtils.Positions(record.ClassPosition),
					},
					event,
				)
			})
	}
	run := accessUtils.Workspace
	definition, ok := accessConstants.Lookup(action)
	if ok && definition.Scope == accessConstants.ScopeSession {
		run = func(ctx context.Context, _ string, action string, event auditModels.Event,
			operation func(*gorm.DB, accessModels.Principal, *auditModels.Event) error) error {
			return accessUtils.Session(ctx, action, event, operation)
		}
	}
	return run(ctx, workspaceID, action, audit,
		func(tx *gorm.DB, principal accessModels.Principal, event *auditModels.Event) error {
			if !principal.Workspace.IsEnabled {
				return accessUtils.Denied()
			}
			actions, err := accessUtils.Actions(tx, principal.User.ID, workspaceID)
			if err != nil {
				return err
			}
			// 组织账号进入填报管理必须显式授权，节点身份只决定可执行的审批操作。
			if !slices.Contains(actions, accessConstants.ProjectApplicationList) {
				return accessUtils.Denied()
			}
			return operation(tx, applicationActor{UserID: principal.User.ID,
				Name: principal.User.DisplayName, Actions: actions}, event)
		})
}

func isStudentActor(ctx context.Context) bool {
	identity, err := userUtils.Identity(ctx)
	return err == nil && identity.Student != nil
}

func applicationScope(tx *gorm.DB, workspaceID, mode string, actor applicationActor) *gorm.DB {
	// 所有列表、详情及变更均从同一租户和处理人范围开始，不接受外部身份编号。
	scope := tx.Table("project_applications AS a").
		Joins("JOIN projects AS p ON p.workspace_id = a.workspace_id AND p.id = a.project_id").
		Joins("JOIN students AS s ON s.workspace_id = a.workspace_id AND s.id = a.student_id").
		Where("a.workspace_id = ?", workspaceID)
	switch mode {
	case projectConstants.ApplicationManage:
		scope = scope.Where("a.status <> ?", projectConstants.ApplicationDraft)
		if actor.UserID == 0 {
			return scope.Where("1 = 0")
		}
		if !slices.Contains(actor.Actions, accessConstants.ProjectApplicationList) {
			return scope.Where("1 = 0")
		}
	case projectConstants.ApplicationMine:
		if actor.StudentID == 0 {
			return scope.Where("1 = 0")
		}
		scope = scope.Where("a.student_id = ?", actor.StudentID)
	case projectConstants.ApplicationReviews:
		scope = scope.Where("a.status <> ?", projectConstants.ApplicationDraft)
		nodes := tx.Table("project_application_nodes AS n").Select("1").
			Where("n.workspace_id = ? AND n.application_id = a.id AND n.position > 0", workspaceID)
		nodes = nodes.Where(applicationNodeActorScope(tx, actor))
		scope = scope.Where("EXISTS (?)", nodes)
	default:
		scope = scope.Where("1 = 0")
	}
	return scope
}

func actorMatches(actor applicationActor, applicantID uint64, className string,
	node projectModels.ApplicationNode,
) bool {
	switch node.Kind {
	case projectConstants.ApproverApplicant:
		return actor.StudentID != 0 && actor.StudentID == applicantID
	case projectConstants.ApproverStudent:
		return actor.StudentID != 0 && node.StudentID != nil && actor.StudentID == *node.StudentID
	case projectConstants.ApproverUser:
		return actor.UserID != 0 && node.UserID != nil && actor.UserID == *node.UserID
	case projectConstants.ApproverClassPosition:
		return actor.StudentID != 0 && className != "" && actor.ClassName == className &&
			slices.Contains(actor.Positions, node.ClassPosition)
	default:
		return false
	}
}

func applicationPermissions(info *projectModels.ApplicationSummary, actor applicationActor,
	mode string, node projectModels.ApplicationNode,
) {
	// 填报管理必须有查看权限，再按当前节点确定修改、通过或驳回资格。
	if mode == projectConstants.ApplicationManage &&
		!slices.Contains(actor.Actions, accessConstants.ProjectApplicationList) {
		return
	}
	if info.Status == projectConstants.ApplicationApproved ||
		!actorMatches(actor, info.StudentID, info.ClassName, node) {
		return
	}
	if mode == projectConstants.ApplicationMine &&
		(node.Kind != projectConstants.ApproverApplicant || info.Status != projectConstants.ApplicationRejected && info.Status != projectConstants.ApplicationDraft) {
		return
	}
	if mode == projectConstants.ApplicationReviews && node.Position == 0 {
		return
	}
	info.CanEdit = node.Position == 0 || node.AllowEdit
	info.CanApprove, info.CanReject = true, node.Position > 0
}

func applicationNodeActorScope(tx *gorm.DB, actor applicationActor) *gorm.DB {
	if actor.StudentID == 0 {
		return tx.Where("n.kind = ? AND n.user_id = ?", projectConstants.ApproverUser, actor.UserID)
	}
	match := tx.Where("n.kind = ? AND a.student_id = ?", projectConstants.ApproverApplicant, actor.StudentID).
		Or("n.kind = ? AND n.student_id = ?", projectConstants.ApproverStudent, actor.StudentID)
	if actor.ClassName != "" && len(actor.Positions) > 0 {
		match = match.Or("n.kind = ? AND n.class_position IN ? AND s.class_name = ?",
			projectConstants.ApproverClassPosition, actor.Positions, actor.ClassName)
	}
	return match
}
