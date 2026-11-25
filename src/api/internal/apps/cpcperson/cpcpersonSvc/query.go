package cpcpersonSvc

import (
	"context"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/partybranch/partybranchModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
)

// List 将范围条件应用于计数与分页，关联仅使用同空间学生和党支部。
func List(
	ctx context.Context, workspaceID string, query cpcpersonModels.Query,
) (cpcpersonModels.ListData, error) {
	result := cpcpersonModels.ListData{
		Items: []cpcpersonModels.Info{}, Page: query.Page, PageSize: query.PageSize,
	}
	err := within(
		ctx,
		workspaceID,
		accessConstants.CPCPersonList,
		queryEvent(workspaceID, query),
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			result.CanCreate = access.require(accessConstants.CPCPersonCreate) == nil
			result.CanImport = access.require(accessConstants.CPCPersonImport) == nil
			result.CanExport = access.require(accessConstants.CPCPersonExport) == nil
			scope, err := filtered(tx, workspaceID, query, access, accessConstants.CPCPersonList)
			if err != nil {
				return err
			}
			if err := scope.Count(&result.Total).Error; err != nil {
				return err
			}
			if err := withBranchSecretaries(scope).Order("p.id DESC").Limit(query.PageSize).
				Offset((query.Page - 1) * query.PageSize).Find(&result.Items).Error; err != nil {
				return err
			}
			for index := range result.Items {
				setCapabilities(&result.Items[index], access)
				audit.ReturnedIDs = append(
					audit.ReturnedIDs,
					strconv.FormatUint(result.Items[index].ID, 10),
				)
			}
			audit.ReturnedCount = int64(len(result.Items))
			return nil
		},
	)
	return result, err
}

func filtered(
	tx *gorm.DB,
	workspaceID string,
	query cpcpersonModels.Query,
	access personAccess,
	action string,
) (*gorm.DB, error) {
	keyword := strings.TrimSpace(query.Keyword)
	if !validText(keyword, cpcpersonConstants.KeywordLength) {
		return nil, invalid("搜索关键词最多 200 字符，且不能包含控制字符")
	}
	scope := access.limit(infoScope(tx, workspaceID), action, "p.party_branch_id")
	if query.TemplateID != 0 {
		scope = scope.Where("p.timeline_template_id = ?", query.TemplateID)
	}
	if keyword != "" {
		scope = scope.Where(
			"(LOCATE(?, s.student_no) > 0 OR LOCATE(?, s.name) > 0)",
			keyword,
			keyword,
		)
	}
	for key, values := range query.Filters {
		column, ok := cpcpersonConstants.FilterColumns[key]
		if (!ok && key != "party_branch_id") || len(values) > config.Config.CPCPerson.MaxOptions {
			return nil, invalid("不支持的人员筛选条件")
		}
		if len(values) == 0 {
			continue
		}
		if key == "party_branch_id" {
			ids := make([]uint64, len(values))
			for index, value := range values {
				id, err := strconv.ParseUint(value, 10, 64)
				if err != nil || id == 0 {
					return nil, invalid("党支部筛选无效")
				}
				ids[index] = id
			}
			scope = scope.Where("p.party_branch_id IN ?", ids)
			continue
		}
		limit := cpcpersonConstants.KeywordLength
		if key == "status" {
			limit = cpcpersonConstants.StatusLength
		}
		for _, value := range values {
			if !validText(value, limit) {
				return nil, invalid("筛选值无效")
			}
			if key == "timeline_status" && value != "normal" && value != "abnormal" &&
				value != "transferring" {
				return nil, invalid("请选择有效的时间线状态")
			}
			if key == "stage" {
				stage, ok := cpcpersonConstants.StageValue(value)
				if !ok || stage != value {
					return nil, invalid("发展阶段筛选必须使用英文枚举代码")
				}
			}
		}
		scope = scope.Where(column+" IN ?", values)
	}
	scope, err := filterTimelineNode(tx, scope, query)
	if err != nil {
		return nil, err
	}
	return filterTimeline(tx, scope, workspaceID, query.TimelineFilters, access, action)
}

// Options 对动态状态、职务及学生维度使用与列表完全相同的读取范围。
func Options(
	ctx context.Context,
	workspaceID, field string,
) ([]cpcpersonModels.Option, error) {
	result := []cpcpersonModels.Option{}
	audit := event(workspaceID, 0)
	audit.Conditions["field"] = field
	err := within(ctx, workspaceID, accessConstants.CPCPersonList, audit,
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			column, ok := cpcpersonConstants.FilterColumns[field]
			if !ok {
				return invalid("不支持的人员选项维度")
			}
			switch field {
			case "stage":
				result = append(result, cpcpersonConstants.Stages...)
			case "timeline_status":
				result = append(result, cpcpersonConstants.TimelineStatuses...)
			default:
				values := []string{}
				if err := access.limit(infoScope(tx, workspaceID), accessConstants.CPCPersonList, "p.party_branch_id").
					Distinct(column).
					Where(column+" <> ?", "").
					Order(column+" ASC").
					Limit(config.Config.CPCPerson.MaxOptions+1).
					Pluck(column, &values).
					Error; err != nil {
					return err
				}
				if len(values) > config.Config.CPCPerson.MaxOptions {
					return invalid("人员选项超过限制，请缩小数据范围")
				}
				for _, value := range values {
					result = append(result, cpcpersonModels.Option{Value: value, Label: value})
				}
			}
			audit.ReturnedCount = int64(len(result))
			return nil
		})
	return result, err
}

// Branches 仅返回名单筛选范围内的党支部，人员归属不接受手工选择。
func Branches(
	ctx context.Context, workspaceID, operation string,
) ([]cpcpersonModels.Option, error) {
	result := []cpcpersonModels.Option{}
	err := within(ctx, workspaceID, accessConstants.CPCPersonList, event(workspaceID, 0),
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			action := accessConstants.CPCPersonList
			switch operation {
			case "", "list":
			default:
				return invalid("党支部选项用途无效")
			}
			var branches []partybranchModels.Branch
			if err := access.limit(tx.Where("workspace_id = ?", workspaceID), action, "id").
				Order("short_name ASC, id ASC").Limit(config.Config.CPCPerson.MaxOptions + 1).
				Find(&branches).Error; err != nil {
				return err
			}
			if len(branches) > config.Config.CPCPerson.MaxOptions {
				return invalid("党支部选项超过限制")
			}
			for _, branch := range branches {
				id := strconv.FormatUint(branch.ID, 10)
				result = append(result, cpcpersonModels.Option{Value: id, Label: branch.ShortName})
				audit.ReturnedIDs = append(audit.ReturnedIDs, id)
			}
			audit.Conditions["operation"] = action
			audit.ReturnedCount = int64(len(result))
			return nil
		})
	return result, err
}

// Candidates 只枚举当前空间尚未入册、状态为在籍且符合可新增支部规则的学生。
func Candidates(
	ctx context.Context,
	workspaceID, keyword string,
) ([]cpcpersonModels.Candidate, error) {
	result := []cpcpersonModels.Candidate{}
	err := within(ctx, workspaceID, accessConstants.CPCPersonCreate, event(workspaceID, 0),
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			if !validText(keyword, cpcpersonConstants.KeywordLength) {
				return invalid("学生搜索关键词无效")
			}
			audit.Conditions["keyword"] = keyword
			if !access.hasScope(accessConstants.CPCPersonCreate) {
				return nil
			}
			expression, err := matchingExpression(tx, workspaceID)
			if err != nil {
				return err
			}
			// 在籍学生按全量规则确定唯一归属，支部关联自动排除未匹配学生。
			matched := tx.Table("students AS s").Select("s.id, ? AS party_branch_id", expression).
				Where("s.workspace_id = ?", workspaceID).
				Where("s.status = ?", studentConstants.StatusEnrolled)
			scope := tx.Table("(?) AS matched", matched).
				Select("s.id AS value, "+studentColumns+", b.id AS party_branch_id, b.short_name AS party_branch_name").
				Joins("JOIN students AS s ON s.workspace_id = ? AND s.id = matched.id", workspaceID).
				Joins("JOIN cpc_party_branches AS b ON b.workspace_id = ? AND b.id = matched.party_branch_id", workspaceID).
				Where("NOT EXISTS (SELECT 1 FROM cpc_people AS p WHERE p.workspace_id = s.workspace_id AND p.student_id = s.id)")
			scope = access.limit(scope, accessConstants.CPCPersonCreate, "b.id")
			if keyword != "" {
				scope = scope.Where(
					"(LOCATE(?, s.student_no) > 0 OR LOCATE(?, s.name) > 0)",
					keyword,
					keyword,
				)
			}
			if err := scope.Order("s.student_no ASC, s.id ASC").
				Limit(config.Config.CPCPerson.MaxOptions + 1).Find(&result).Error; err != nil {
				return err
			}
			if len(result) > config.Config.CPCPerson.MaxOptions {
				return invalid("学生选项过多，请输入学号或姓名搜索")
			}
			for index := range result {
				item := &result[index]
				item.Label = item.Name + "(" + item.StudentNo + ")"
				audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(item.Value, 10))
			}
			audit.ReturnedCount = int64(len(result))
			return nil
		})
	return result, err
}

// Fields 返回党建可写字段及固定学生只读字段目录。
func Fields(ctx context.Context, workspaceID string) ([]cpcpersonModels.Field, error) {
	result := []cpcpersonModels.Field{}
	err := within(ctx, workspaceID, accessConstants.CPCPersonList, event(workspaceID, 0),
		func(_ *gorm.DB, _ personAccess, audit *auditModels.Event) error {
			result = append(result, cpcpersonConstants.Fields...)
			for _, field := range cpcpersonConstants.StudentFields {
				field.Type, field.ReadOnly = "string", true
				field.Options = []cpcpersonModels.Option{}
				result = append(result, field)
			}
			audit.ReturnedCount = int64(len(result))
			return nil
		})
	return result, err
}

// Statistics 与名单复用筛选和读取范围，分页不影响统计结果。
func Statistics(
	ctx context.Context, workspaceID string, query cpcpersonModels.Query,
) (cpcpersonModels.StatisticsData, error) {
	result := cpcpersonModels.StatisticsData{Stages: []cpcpersonModels.StageCount{}}
	err := within(
		ctx,
		workspaceID,
		accessConstants.CPCPersonStatistics,
		queryEvent(workspaceID, query),
		func(tx *gorm.DB, access personAccess, audit *auditModels.Event) error {
			scope, err := filtered(
				tx,
				workspaceID,
				query,
				access,
				accessConstants.CPCPersonStatistics,
			)
			if err != nil {
				return err
			}
			var counts []struct {
				Value string
				Count int64
			}
			if err := scope.Select("p.stage AS value, COUNT(*) AS count").Group("p.stage").
				Scan(&counts).Error; err != nil {
				return err
			}
			values := map[string]int64{}
			for _, item := range counts {
				values[item.Value] = item.Count
			}
			for _, stage := range cpcpersonConstants.Stages {
				count := values[stage.Value]
				result.Stages = append(
					result.Stages,
					cpcpersonModels.StageCount{Option: stage, Count: count},
				)
				result.Total += count
			}
			audit.Conditions["group_by"] = "stage"
			audit.Conditions["matched_people"] = result.Total
			audit.ReturnedCount = int64(len(result.Stages))
			return nil
		},
	)
	return result, err
}

func queryEvent(workspaceID string, query cpcpersonModels.Query) auditModels.Event {
	audit := event(workspaceID, 0)
	audit.Conditions["keyword"] = query.Keyword
	audit.Conditions["filters"] = query.Filters
	audit.Conditions["timeline_filters"] = query.TimelineFilters
	audit.Conditions["timeline_template_id"] = query.TemplateID
	audit.Conditions["timeline_node_id"] = query.NodeID
	audit.Conditions["node_status"] = query.NodeStatuses
	audit.Conditions["page"] = query.Page
	audit.Conditions["page_size"] = query.PageSize
	return audit
}
