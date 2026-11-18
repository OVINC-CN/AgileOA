package cpctimelineSvc

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"
)

// List 只返回当前空间的模板摘要，完整定义使用独立详情权限。
func List(
	ctx context.Context,
	workspaceID string,
	query cpctimelineModels.Query,
) (cpctimelineModels.ListData, error) {
	result := cpctimelineModels.ListData{
		Items: []cpctimelineModels.Summary{}, Page: query.Page, PageSize: query.PageSize,
	}
	query.Keyword = strings.TrimSpace(query.Keyword)
	auditEvent := event(workspaceID, 0)
	auditEvent.Conditions["keyword"] = query.Keyword
	auditEvent.Conditions["page"] = query.Page
	auditEvent.Conditions["page_size"] = query.PageSize
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.CPCTimelineList,
		auditEvent,
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 固定名称、说明搜索，不接受外部列名。
			if !validText(query.Keyword, cpctimelineConstants.NameLength, false, false) {
				return invalid("搜索关键词最多 100 字符，且不能包含控制字符")
			}
			scope := tx.Model(&cpctimelineModels.Template{}).
				Where("workspace_id = ?", principal.Workspace.ID)
			if query.Keyword != "" {
				scope = scope.Where("(LOCATE(?, name) > 0 OR LOCATE(?, description) > 0)",
					query.Keyword, query.Keyword)
			}
			if err := scope.Count(&result.Total).Error; err != nil {
				return err
			}
			nodeCount := tx.Model(&cpctimelineModels.TemplateNode{}).Select("COUNT(*)").
				Where("workspace_id = ? AND timeline_template_id = cpc_timeline_templates.id",
					principal.Workspace.ID)
			if err := scope.Select("id, name, description, revision, created_at, updated_at, "+
				"(?) AS node_count", nodeCount).
				Order("id DESC").Limit(query.PageSize).Offset((query.Page - 1) * query.PageSize).
				Find(&result.Items).Error; err != nil {
				return err
			}

			// 查询审计提交后再返回摘要。
			for _, item := range result.Items {
				audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(item.ID, 10))
			}
			audit.ReturnedCount = int64(len(result.Items))
			return nil
		},
	)
	return result, err
}

// Detail 按独立详情权限读取完整定义并先保存读取审计。
func Detail(
	ctx context.Context,
	workspaceID string,
	id uint64,
) (cpctimelineModels.Info, error) {
	var result cpctimelineModels.Info
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.CPCTimelineDetail,
		event(workspaceID, id),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			var err error
			result, err = load(tx, principal.Workspace.ID, id)
			if err != nil {
				return err
			}
			audit.ReturnedIDs = []string{strconv.FormatUint(id, 10)}
			audit.ReturnedCount = 1
			return nil
		})
	return result, err
}

// Fields 返回固定协议目录，不查询人员或自定义数据库字段。
func Fields(
	ctx context.Context,
	workspaceID string,
) ([]cpctimelineModels.CatalogField, error) {
	var result []cpctimelineModels.CatalogField
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.CPCTimelineList,
		event(workspaceID, 0),
		func(_ *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			result = append([]cpctimelineModels.CatalogField{}, cpctimelineConstants.Fields...)
			audit.ReturnedCount = int64(len(result))
			return nil
		})
	return result, err
}

// ImportTemplate 提供通用 JSON 格式，不生成任何预置业务记录。
func ImportTemplate(
	ctx context.Context,
	workspaceID string,
) (cpctimelineModels.FileData, error) {
	var result cpctimelineModels.FileData
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.CPCTimelineImport,
		event(workspaceID, 0),
		func(_ *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			input := cpctimelineModels.Input{
				Name:      "新时间线模板",
				Transfers: []cpctimelineModels.Transfer{},
				Definition: cpctimelineModels.Definition{Nodes: []cpctimelineModels.Node{{
					ID: "node_start", Name: "起始节点", TimeFieldID: "occurred_at",
					GroupName: "起始分组",
					Fields: []cpctimelineModels.Field{{
						ID: "occurred_at", Name: "发生时间", Type: "date", Required: true,
					}, {
						ID: "birth_date", Name: "出生日期", Type: "date",
						StudentField: cpctimelineConstants.StudentBirthDate,
					}, {
						ID: "adulthood_date", Name: "成年日期", Type: "date",
						StudentField: cpctimelineConstants.StudentAdulthoodDate,
					}},
					Rules: []cpctimelineModels.Rule{},
				}}},
			}
			content, err := json.MarshalIndent(input, "", "  ")
			if err != nil {
				return err
			}
			result = cpctimelineModels.FileData{
				Filename: "timeline-template.json", ContentType: "application/json;charset=utf-8",
				Content: string(content) + "\n",
			}
			audit.ReturnedCount = 1
			return nil
		})
	return result, err
}
