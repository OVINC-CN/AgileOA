package attachmentSvc

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpctimeline/cpctimelineConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/monthlywork/monthlyworkConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

var timelineNodeID = regexp.MustCompile(cpctimelineConstants.StableIDPattern)

// attachmentPolicy 只描述代码固定的业务映射，不接受请求指定表名或列名。
type attachmentPolicy struct {
	ownerColumn    string
	table          string
	readAction     string
	uploadAction   string
	deleteAction   string
	maxAttachments int
	workCategory   string
}

func policyFor(category string) (attachmentPolicy, error) {
	switch category {
	case attachmentConstants.CategoryProject:
		return attachmentPolicy{
			ownerColumn:    "project_id",
			table:          (attachmentModels.ProjectAttachment{}).TableName(),
			readAction:     accessConstants.ProjectAttachmentRead,
			uploadAction:   accessConstants.ProjectAttachmentUpload,
			deleteAction:   accessConstants.ProjectAttachmentDelete,
			maxAttachments: config.Config.Project.MaxAttachments,
		}, nil
	case attachmentConstants.CategoryMonthlyWork:
		return attachmentPolicy{
			ownerColumn:    "monthly_work_id",
			table:          (attachmentModels.MonthlyWorkAttachment{}).TableName(),
			readAction:     accessConstants.MonthlyWorkAttachmentRead,
			uploadAction:   accessConstants.MonthlyWorkAttachmentUpload,
			deleteAction:   accessConstants.MonthlyWorkAttachmentDelete,
			maxAttachments: config.Config.MonthlyWork.MaxAttachments,
			workCategory:   monthlyworkConstants.CategoryStudent,
		}, nil
	case attachmentConstants.CategoryCPCMonthlyWork:
		return attachmentPolicy{
			ownerColumn:    "monthly_work_id",
			table:          (attachmentModels.MonthlyWorkAttachment{}).TableName(),
			readAction:     accessConstants.CPCMonthlyWorkAttachmentRead,
			uploadAction:   accessConstants.CPCMonthlyWorkAttachmentUpload,
			deleteAction:   accessConstants.CPCMonthlyWorkAttachmentDelete,
			maxAttachments: config.Config.MonthlyWork.MaxAttachments,
			workCategory:   monthlyworkConstants.CategoryCPC,
		}, nil
	case attachmentConstants.CategoryCPCPersonTimelineNode:
		return attachmentPolicy{
			ownerColumn:    "person_id",
			table:          (attachmentModels.PersonTimelineAttachment{}).TableName(),
			readAction:     accessConstants.CPCPersonDetail,
			uploadAction:   accessConstants.CPCPersonUpdate,
			deleteAction:   accessConstants.CPCPersonUpdate,
			maxAttachments: config.Config.CPCTimeline.MaxAttachments,
		}, nil
	case attachmentConstants.CategoryCPCTimelineNode:
		return attachmentPolicy{
			ownerColumn:    "timeline_template_id",
			table:          (attachmentModels.TimelineNodeAttachment{}).TableName(),
			readAction:     accessConstants.CPCTimelineAttachmentRead,
			uploadAction:   accessConstants.CPCTimelineAttachmentUpload,
			deleteAction:   accessConstants.CPCTimelineAttachmentDelete,
			maxAttachments: config.Config.CPCTimeline.MaxAttachments,
		}, nil
	default:
		return attachmentPolicy{}, failure(http.StatusBadRequest, "附件类别无效")
	}
}

func (policy attachmentPolicy) scope(tx *gorm.DB, workspaceID string) *gorm.DB {
	scope := tx.Model(&attachmentModels.Record{}).
		Table(policy.table).
		Where("workspace_id = ?", workspaceID)
	// 月度附件通过所属记录的类别隔离，沿用原有保护型复合外键。
	if policy.workCategory != "" {
		owners := tx.Table("monthly_work_records").Select("id").
			Where("workspace_id = ? AND category = ?", workspaceID, policy.workCategory)
		scope = scope.Where("monthly_work_id IN (?)", owners)
	}
	return scope
}

func (policy attachmentPolicy) ownerSelection() string {
	return "*, " + policy.ownerColumn + " AS owner_id"
}

func create(
	tx *gorm.DB,
	category string,
	ownerID uint64,
	nodeID string,
	record attachmentModels.Record,
) (uint64, error) {
	if category == attachmentConstants.CategoryProject {
		file := attachmentModels.ProjectAttachment{Record: record, ProjectID: ownerID}
		err := tx.Create(&file).Error
		return file.ID, err
	}
	if category == attachmentConstants.CategoryCPCPersonTimelineNode {
		file := attachmentModels.PersonTimelineAttachment{
			Record:   record,
			PersonID: ownerID,
			NodeID:   nodeID,
		}
		err := tx.Create(&file).Error
		return file.ID, err
	}
	if category == attachmentConstants.CategoryCPCTimelineNode {
		file := attachmentModels.TimelineNodeAttachment{
			Record: record, TimelineTemplateID: ownerID, NodeID: nodeID,
		}
		err := tx.Create(&file).Error
		return file.ID, err
	}
	file := attachmentModels.MonthlyWorkAttachment{Record: record, MonthlyWorkID: ownerID}
	err := tx.Create(&file).Error
	return file.ID, err
}

func objectName(
	tx *gorm.DB,
	workspaceID, category string,
	id uint64,
	nodeID, filename string,
) (string, error) {
	prefix := "workspaces/" + workspaceID
	if category == attachmentConstants.CategoryCPCPersonTimelineNode {
		if id == 0 || !timelineNodeID.MatchString(nodeID) {
			return "", failure(http.StatusBadRequest, "人员编号或节点标识无效")
		}
		return randomObjectName(
			prefix+"/cpc-people/"+strconv.FormatUint(id, 10)+"/nodes/"+nodeID,
			filename,
		)
	}
	if category == attachmentConstants.CategoryCPCTimelineNode {
		// 节点必须属于当前空间及模板，禁止以其他模板的节点编号上传。
		if err := requireTimelineNode(tx, workspaceID, id, nodeID); err != nil {
			return "", err
		}
		return randomObjectName(
			prefix+"/cpc-timeline-templates/"+strconv.FormatUint(id, 10)+"/nodes/"+nodeID, filename,
		)
	}
	if category == attachmentConstants.CategoryProject {
		var count int64
		if err := tx.Table("projects").Where("workspace_id = ? AND id = ?", workspaceID, id).
			Count(&count).Error; err != nil {
			return "", err
		}
		if count != 1 {
			return "", failure(http.StatusBadRequest, "项目不存在或不可访问")
		}
		return randomObjectName(prefix+"/projects/"+strconv.FormatUint(id, 10), filename)
	}

	// 使用当前工作记录日期，每次上传独立生成十位随机目录，不检查同名对象。
	policy, err := policyFor(category)
	if err != nil {
		return "", err
	}
	var record struct{ Date string }
	err = tx.Table("monthly_work_records").Select("date").
		Where("workspace_id = ? AND category = ? AND id = ?",
			workspaceID, policy.workCategory, id).Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", failure(http.StatusBadRequest, "工作记录不存在或不可访问")
	}
	if err != nil {
		return "", err
	}
	return workObjectName(prefix, category, record.Date, filename)
}

func requireTimelineNode(tx *gorm.DB, workspaceID string, templateID uint64, nodeID string) error {
	if templateID == 0 || !timelineNodeID.MatchString(nodeID) {
		return failure(http.StatusBadRequest, "节点标识或模板编号无效")
	}
	var count int64
	if err := tx.Table("cpc_timeline_template_nodes").Where(
		"workspace_id = ? AND timeline_template_id = ? AND node_id = ?",
		workspaceID, templateID, nodeID,
	).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return failure(http.StatusNotFound, "节点不存在或不可访问，请先保存节点")
	}
	return nil
}

func workObjectName(prefix, category, dateValue, filename string) (string, error) {
	// 学生工作与党建工作使用各自的固定目录，日期和随机目录规则一致。
	directory := "work"
	if category == attachmentConstants.CategoryCPCMonthlyWork {
		directory = "cpc-work"
	}
	date, err := time.Parse(time.DateOnly, dateValue)
	if err != nil {
		return "", err
	}
	return randomObjectName(prefix+"/"+directory+"/"+date.Format("2006/01/02"), filename)
}

func randomObjectName(prefix, filename string) (string, error) {
	return attachmentUtils.RandomObjectName(prefix, filename)
}

func requirePersonTimeline(
	tx *gorm.DB, principal accessModels.Principal, category string,
	ownerID uint64, nodeID, action string,
) error {
	if category != attachmentConstants.CategoryCPCPersonTimelineNode {
		return nil
	}
	_, err := cpcpersonSvc.RequireTimelineNode(tx, principal, ownerID, nodeID, action)
	if err != nil || action != accessConstants.CPCPersonUpdate {
		return err
	}
	// 附件上传和删除的每个写入阶段都在空间锁内复核转移状态。
	var person cpcpersonModels.Person
	if err := tx.Select("timeline_status").
		Where("workspace_id = ? AND id = ?", principal.Workspace.ID, ownerID).
		Take(&person).Error; err != nil {
		return err
	}
	return cpcpersonUtils.RequireTimelineWritable(person.TimelineStatus)
}
