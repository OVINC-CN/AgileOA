package cpcpersonSvc

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

const studentColumns = "s.student_no, s.name, s.gender, s.ethnicity, s.grade, " +
	"s.status AS student_status, s.education_level, s.class_name, s.major, s.advisor, s.phone, " +
	"s.arrival_month, s.previous_school_or_employer, s.previous_graduation_month"

const personColumns = "p.id, p.student_id, p.stage, p.status, p.party_branch_id, " +
	"p.party_position, p.revision, p.timeline_template_id, p.timeline_status, " +
	"tr.target_template_id AS timeline_transfer_target_id, " +
	"COALESCE(t.name, '') AS timeline_template_name, " +
	"(SELECT COUNT(*) FROM cpc_person_timeline_nodes AS n WHERE " +
	"n.workspace_id = p.workspace_id AND n.person_id = p.id AND n.status = 'pending') AS timeline_pending, " +
	"(SELECT COUNT(*) FROM cpc_person_timeline_nodes AS n WHERE " +
	"n.workspace_id = p.workspace_id AND n.person_id = p.id AND n.status = 'completed') AS timeline_completed, " +
	"(SELECT COUNT(*) FROM cpc_person_timeline_nodes AS n WHERE " +
	"n.workspace_id = p.workspace_id AND n.person_id = p.id AND n.status = 'skipped') AS timeline_skipped, " +
	"p.created_at, p.updated_at, " + studentColumns +
	", COALESCE(b.full_name, '') AS party_branch_name, " +
	"COALESCE(b.short_name, '') AS party_branch_short_name"

// Save 在空间锁内复核细粒度权限、书记范围、学生和支部准入规则后保存。
func Save(
	ctx context.Context, workspaceID string, id uint64, request cpcpersonModels.Request,
) (cpcpersonModels.Info, error) {
	var result cpcpersonModels.Info
	saved := false
	action := accessConstants.CPCPersonCreate
	if id != 0 {
		action = accessConstants.CPCPersonUpdate
	}
	err := accessUtils.Workspace(ctx, workspaceID, action, event(workspaceID, id),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 先校验当前授权范围，再读取对象，避免通过编号探测其他支部。
			access, err := loadAccess(tx, principal)
			if err != nil {
				return err
			}
			if err := access.require(action); err != nil {
				return err
			}
			var branchID *uint64
			var previous *cpcpersonModels.Info
			if id != 0 {
				before, err := detail(tx, workspaceID, id, access, action)
				if err != nil {
					return err
				}
				if err := cpcpersonUtils.RequireTimelineWritable(
					before.TimelineStatus,
				); err != nil {
					return err
				}
				if before.Revision != request.Revision {
					return stale()
				}
				if before.StudentID != request.StudentID {
					return invalid("编辑时不能更换关联学生")
				}
				audit.Before = snapshot(before)
				branchID = &before.PartyBranchID
				previous = &before
				if strings.TrimSpace(request.Status) == "" {
					request.Status = before.Status
				}
			}
			input, err := clean(request.Input)
			if err != nil {
				return err
			}
			if id == 0 {
				branchID, err = determineBranch(tx, workspaceID, input.StudentID)
				if err != nil {
					return err
				}
			}
			if !access.permits(action, branchID) {
				return accessUtils.Denied()
			}
			// 新增必选模板；编辑只保留现有绑定，存量绑定通过独立入口完成。
			binding := id == 0
			if previous != nil {
				if input.TimelineTemplateID != nil && (previous.TimelineTemplateID == nil ||
					*input.TimelineTemplateID != *previous.TimelineTemplateID) {
					return invalid("编辑人员时不能修改时间线模板，请通过时间线入口绑定未绑定人员")
				}
				input.TimelineTemplateID = previous.TimelineTemplateID
			}
			if id == 0 && input.TimelineTemplateID == nil {
				return invalid("新增人员必须绑定时间线模板")
			}
			if binding {
				if _, err := timelineTemplate(
					tx,
					workspaceID,
					*input.TimelineTemplateID,
				); err != nil {
					return err
				}
			}
			if previous != nil && previous.Stage == input.Stage &&
				previous.Status == input.Status &&
				previous.PartyPosition == input.PartyPosition &&
				!binding {
				result = *previous
				audit.After = snapshot(result)
				return nil
			}

			// 学生基础信息只读；新增与更新都不写入学生表。
			normal := cpcpersonUtils.TimelineNormal
			record := cpcpersonModels.Person{
				WorkspaceID: workspaceID, StudentID: input.StudentID, Stage: input.Stage,
				Status:        input.Status,
				PartyBranchID: branchID, PartyPosition: input.PartyPosition,
				TimelineTemplateID: input.TimelineTemplateID, TimelineStatus: &normal,
				Revision: 1,
			}
			if id == 0 {
				if err := tx.Create(&record).Error; err != nil {
					return writeError(err)
				}
				id = record.ID
			} else {
				changed := tx.Model(&cpcpersonModels.Person{}).
					Where("workspace_id = ? AND id = ? AND revision = ?", workspaceID, id, request.Revision).
					Updates(map[string]any{
						"stage":                input.Stage,
						"status":               input.Status,
						"party_position":       input.PartyPosition,
						"timeline_template_id": input.TimelineTemplateID,
						"revision":             request.Revision + 1,
					})
				if changed.Error != nil {
					return writeError(changed.Error)
				}
				if changed.RowsAffected != 1 {
					return stale()
				}
			}
			if binding {
				if err := createTimelineNodes(
					tx,
					workspaceID,
					action,
					id,
					*input.TimelineTemplateID,
				); err != nil {
					return err
				}
			}

			// 响应沿用本次写入范围，操作成功及快照在提交后才对外可见。
			result, err = detail(tx, workspaceID, id, access, action)
			if err != nil {
				return err
			}
			audit.ObjectID = strconv.FormatUint(id, 10)
			audit.After = snapshot(result)
			saved = true
			return nil
		})
	if err == nil && saved {
		logger.Logger.WithContext(ctx).Infof(
			"CPC person saved; action: %s; workspace: %s; person: %d", action, workspaceID, id,
		)
	}
	return result, err
}

// Detail 仅返回查看范围内的人员，读取审计持久化后返回。
func Detail(
	ctx context.Context,
	workspaceID string,
	id uint64,
) (cpcpersonModels.Info, error) {
	var result cpcpersonModels.Info
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.CPCPersonDetail,
		event(workspaceID, id),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			access, err := loadAccess(tx, principal)
			if err != nil {
				return err
			}
			if err := access.require(accessConstants.CPCPersonDetail); err != nil {
				return err
			}
			result, err = detail(tx, workspaceID, id, access, accessConstants.CPCPersonDetail)
			audit.ReturnedIDs = []string{strconv.FormatUint(id, 10)}
			audit.ReturnedCount = 1
			return err
		})
	return result, err
}

// Delete 按独立删除范围硬删除党建记录，不删除关联学生。
func Delete(ctx context.Context, workspaceID string, id, revision uint64) error {
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.CPCPersonDelete,
		event(workspaceID, id),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			access, err := loadAccess(tx, principal)
			if err != nil {
				return err
			}
			if err := access.require(accessConstants.CPCPersonDelete); err != nil {
				return err
			}
			before, err := detail(tx, workspaceID, id, access, accessConstants.CPCPersonDelete)
			if err != nil {
				return err
			}
			if before.Revision != revision {
				return stale()
			}
			if err := cpcpersonUtils.RequireTimelineWritable(before.TimelineStatus); err != nil {
				return err
			}
			audit.Before = snapshot(before)

			// 级联删除前逐条保留人员节点审计。
			var nodes []cpcpersonModels.TimelineNode
			if err := tx.Where("workspace_id = ? AND person_id = ?", workspaceID, id).
				Find(&nodes).Error; err != nil {
				return err
			}
			for _, node := range nodes {
				item := event(workspaceID, id)
				item.Action, item.Result = accessConstants.CPCPersonDelete, auditConstants.Success
				item.ObjectType = "cpc_person_timeline_node"
				item.Before = cpcpersonUtils.TimelineSnapshot(node)
				if err := auditSvc.Write(tx, item); err != nil {
					return err
				}
			}

			// 人员节点、转移申请及附件记录随人员级联删除，学生和模板保留。
			return writeError(tx.Where("workspace_id = ? AND id = ?", workspaceID, id).
				Delete(&cpcpersonModels.Person{}).Error)
		})
	if err == nil {
		logger.Logger.WithContext(ctx).Infof(
			"CPC person deleted; action: %s; workspace: %s; person: %d",
			accessConstants.CPCPersonDelete, workspaceID, id,
		)
	}
	return err
}

func infoScope(tx *gorm.DB, workspaceID string) *gorm.DB {
	return tx.Table("cpc_people AS p").
		Select(personColumns).
		Joins("JOIN students AS s ON s.workspace_id = p.workspace_id AND s.id = p.student_id").
		Joins("LEFT JOIN cpc_party_branches AS b ON b.workspace_id = p.workspace_id AND b.id = p.party_branch_id").
		Joins("LEFT JOIN cpc_timeline_templates AS t ON t.workspace_id = p.workspace_id AND t.id = p.timeline_template_id").
		Joins("LEFT JOIN cpc_person_timeline_transfer_reviews AS tr "+
			"ON tr.workspace_id = p.workspace_id AND tr.person_id = p.id").
		Where("p.workspace_id = ?", workspaceID)
}

func withBranchSecretaries(scope *gorm.DB) *gorm.DB {
	return scope.Select(
		personColumns+", COALESCE((SELECT JSON_ARRAYAGG(ss.name) FROM cpc_people AS secretary "+
			"JOIN students AS ss ON ss.workspace_id = secretary.workspace_id "+
			"AND ss.id = secretary.student_id "+
			"WHERE secretary.workspace_id = p.workspace_id "+
			"AND secretary.party_branch_id = p.party_branch_id "+
			"AND CAST(secretary.party_position AS BINARY) = ?), JSON_ARRAY()) "+
			"AS party_branch_secretaries",
		"党支部书记",
	)
}

func detail(
	tx *gorm.DB, workspaceID string, id uint64, access personAccess, action string,
) (cpcpersonModels.Info, error) {
	var result cpcpersonModels.Info
	scope := access.limit(infoScope(tx, workspaceID), action, "p.party_branch_id")
	err := withBranchSecretaries(scope).Where("p.id = ?", id).Take(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, missing()
	}
	setCapabilities(&result, access)
	return result, err
}

func setCapabilities(info *cpcpersonModels.Info, access personAccess) {
	info.CanUpdate = access.permits(accessConstants.CPCPersonUpdate, &info.PartyBranchID)
	info.CanDelete = access.permits(accessConstants.CPCPersonDelete, &info.PartyBranchID)
	info.CanReviewTimelineTransfer = info.TimelineTransferTargetID != nil && access.permits(
		accessConstants.CPCPersonTimelineTransferReview,
		&info.PartyBranchID,
	)
}

func clean(input cpcpersonModels.Input) (cpcpersonModels.Input, error) {
	input.Stage = strings.TrimSpace(input.Stage)
	input.Status = strings.TrimSpace(input.Status)
	input.PartyPosition = strings.TrimSpace(input.PartyPosition)
	stage, ok := cpcpersonConstants.StageValue(input.Stage)
	if !ok || stage != input.Stage || input.StudentID == 0 {
		return input, invalid("请选择有效学生及发展阶段")
	}
	if input.Status == "" {
		input.Status = cpcpersonConstants.DefaultStatus
	}
	if !validText(input.Status, cpcpersonConstants.StatusLength) {
		return input, invalid("状态最多 8 字符，且不能包含控制字符")
	}
	if !validText(input.PartyPosition, cpcpersonConstants.PositionLength) {
		return input, invalid("党内职务最多 64 字符，且不能包含控制字符")
	}
	return input, nil
}

func snapshot(info cpcpersonModels.Info) auditModels.Snapshot {
	return auditModels.Snapshot{
		"id": info.ID, "student_id": info.StudentID, "student_no": info.StudentNo,
		"name": info.Name, "stage": info.Stage, "status": info.Status,
		"party_branch_id":   info.PartyBranchID,
		"party_branch_name": info.PartyBranchName, "party_position": info.PartyPosition,
		"revision":                    info.Revision,
		"timeline_template_id":        info.TimelineTemplateID,
		"timeline_template_name":      info.TimelineTemplateName,
		"timeline_status":             info.TimelineStatus,
		"timeline_transfer_target_id": info.TimelineTransferTargetID,
	}
}

func event(workspaceID string, id uint64) auditModels.Event {
	result := auditModels.Event{
		TenantID: &workspaceID, ObjectType: cpcpersonConstants.Object,
		Conditions: auditModels.Snapshot{"workspace_id": workspaceID},
	}
	if id != 0 {
		result.ObjectID = strconv.FormatUint(id, 10)
	}
	return result
}

func validText(value string, limit int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= limit &&
		strings.IndexFunc(value, unicode.IsControl) < 0
}

func invalid(message string) error {
	return &response.BusinessError{Status: http.StatusBadRequest, Message: message}
}

func missing() error {
	return &response.BusinessError{Status: http.StatusNotFound, Message: "人员不存在或不可操作"}
}

func stale() error {
	return &response.BusinessError{Status: http.StatusConflict, Message: "人员信息已被修改，请刷新后重试"}
}

func writeError(err error) error {
	var database *mysql.MySQLError
	if errors.As(err, &database) {
		switch database.Number {
		case 1062:
			return &response.BusinessError{Status: http.StatusConflict, Message: "学生已在名单中或不可操作"}
		case 1451:
			return &response.BusinessError{
				Status:  http.StatusConflict,
				Message: "删除党建人员失败，请检查关联业务后重试",
			}
		case 1452:
			return missing()
		}
	}
	return err
}
