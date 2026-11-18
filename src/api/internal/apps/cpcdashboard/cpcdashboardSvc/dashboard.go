package cpcdashboardSvc

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcdashboard/cpcdashboardConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcdashboard/cpcdashboardModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/spreadsheet"
)

// Summary 在空间锁内复核仪表独立查看权限，读取全空间汇总并保存审计。
func Summary(ctx context.Context, workspaceID string) (cpcdashboardModels.Data, error) {
	result := cpcdashboardModels.Data{Items: []cpcdashboardModels.BranchSummary{}}
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.CPCDashboardList,
		event(),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 人员名单权限及管理员关联不参与仪表范围判断。
			var err error
			result, err = loadSummary(tx, principal.Workspace.ID)
			if err != nil {
				return err
			}
			recordSummary(audit, result)
			return nil
		},
	)
	return result, err
}

// Fields 提供只读固定字段目录，复用仪表查看权限。
func Fields(ctx context.Context, workspaceID string) ([]cpcdashboardModels.Field, error) {
	result := []cpcdashboardModels.Field{}
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.CPCDashboardList,
		event(),
		func(_ *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			// 目录与导出表头保持相同顺序。
			result = append(result, cpcdashboardConstants.Fields...)
			audit.ReturnedCount = int64(len(result))
			return nil
		},
	)
	return result, err
}

// Export 使用独立下载权限，在同一事务内生成与仪表口径一致的 XLSX 并保存审计。
func Export(ctx context.Context, workspaceID string) (cpcdashboardModels.FileData, error) {
	var result cpcdashboardModels.FileData
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		accessConstants.CPCDashboardExport,
		event(),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			// 汇总和下载复用相同查询，避免人数、姓名或合计口径不一致。
			data, err := loadSummary(tx, principal.Workspace.ID)
			if err != nil {
				return err
			}
			content, err := spreadsheet.Encode(exportRecords(data))
			if err != nil {
				return err
			}
			result = cpcdashboardModels.FileData{
				Filename:    "支部党员表.xlsx",
				ContentType: spreadsheet.ContentType,
				Content:     content,
			}
			recordSummary(audit, data)
			return nil
		},
	)
	return result, err
}

type branch struct {
	ID        uint64
	ShortName string
}

type stageCount struct {
	PartyBranchID uint64
	Stage         string
	Count         int64
}

type personName struct {
	PartyBranchID uint64
	PartyPosition string
	Name          string
}

type advisorName struct {
	PartyBranchID uint64
	Advisor       string
}

func loadSummary(tx *gorm.DB, workspaceID string) (cpcdashboardModels.Data, error) {
	// 支部独立查询以保留没有在编党员的空支部，排序与数据管理一致。
	var branches []branch
	if err := tx.Table("cpc_party_branches").Select("id, short_name").
		Where("workspace_id = ?", workspaceID).
		Order("priority DESC, id DESC").Scan(&branches).Error; err != nil {
		return cpcdashboardModels.Data{}, err
	}
	var counts []stageCount
	if err := peopleScope(tx, workspaceID).
		Select("p.party_branch_id, p.stage, COUNT(*) AS count").
		Where("p.stage IN ?", []string{
			cpcdashboardConstants.StageFullMember,
			cpcdashboardConstants.StageProbationaryMember,
		}).Group("p.party_branch_id, p.stage").Scan(&counts).Error; err != nil {
		return cpcdashboardModels.Data{}, err
	}

	// 姓名先按支部去重读取，再在应用内拼接，避免数据库字符串聚合被截断。
	var people []personName
	if err := studentScope(tx, workspaceID).
		Select("p.party_branch_id, p.party_position, s.name").
		Where("TRIM(p.party_position) <> ?", "").
		Group("p.party_branch_id, p.party_position, s.name").Scan(&people).Error; err != nil {
		return cpcdashboardModels.Data{}, err
	}
	var advisors []advisorName
	if err := studentScope(tx, workspaceID).
		Select("p.party_branch_id, s.advisor").Where("TRIM(s.advisor) <> ?", "").
		Group("p.party_branch_id, s.advisor").Scan(&advisors).Error; err != nil {
		return cpcdashboardModels.Data{}, err
	}
	return summarize(branches, counts, people, advisors), nil
}

func peopleScope(tx *gorm.DB, workspaceID string) *gorm.DB {
	return tx.Table("cpc_people AS p").
		Where("p.workspace_id = ? AND p.status = ?", workspaceID, cpcpersonConstants.DefaultStatus)
}

func studentScope(tx *gorm.DB, workspaceID string) *gorm.DB {
	return peopleScope(tx, workspaceID).
		Joins("JOIN students AS s ON s.workspace_id = p.workspace_id AND s.id = p.student_id")
}

func summarize(
	branches []branch,
	counts []stageCount,
	people []personName,
	advisors []advisorName,
) cpcdashboardModels.Data {
	// 索引仅包含当前空间支部，其他支部的聚合值不会进入结果。
	result := cpcdashboardModels.Data{
		Items: make([]cpcdashboardModels.BranchSummary, len(branches)),
	}
	indices := make(map[uint64]int, len(branches))
	names := make(map[uint64]map[string][]string, len(branches))
	for index, item := range branches {
		indices[item.ID] = index
		names[item.ID] = map[string][]string{}
		result.Items[index] = cpcdashboardModels.BranchSummary{
			PartyBranchID: item.ID, PartyBranchName: item.ShortName,
		}
	}
	for _, item := range counts {
		index, ok := indices[item.PartyBranchID]
		if !ok {
			continue
		}
		switch item.Stage {
		case cpcdashboardConstants.StageFullMember:
			result.Items[index].FullMembers += item.Count
		case cpcdashboardConstants.StageProbationaryMember:
			result.Items[index].ProbationaryMembers += item.Count
		}
	}
	for _, item := range people {
		if _, ok := indices[item.PartyBranchID]; !ok {
			continue
		}
		for _, position := range strings.FieldsFunc(item.PartyPosition, positionSeparator) {
			key := positionKey(position)
			if key != "" {
				names[item.PartyBranchID][key] = append(names[item.PartyBranchID][key], item.Name)
			}
		}
	}
	for _, item := range advisors {
		if _, ok := indices[item.PartyBranchID]; ok {
			names[item.PartyBranchID]["advisors"] = append(
				names[item.PartyBranchID]["advisors"], item.Advisor,
			)
		}
	}

	// 汇总姓名与人数，零值支部仍保留；合计只计算正式与预备党员。
	for index := range result.Items {
		item := &result.Items[index]
		values := names[item.PartyBranchID]
		item.Secretaries = joinNames(values["secretaries"])
		item.OrganizationMembers = joinNames(values["organization_members"])
		item.PublicityMembers = joinNames(values["publicity_members"])
		item.DisciplineMembers = joinNames(values["discipline_members"])
		item.Advisors = joinNames(values["advisors"])
		item.Total = item.FullMembers + item.ProbationaryMembers
		result.Totals.FullMembers += item.FullMembers
		result.Totals.ProbationaryMembers += item.ProbationaryMembers
		result.Totals.Total += item.Total
	}
	return result
}

func positionSeparator(value rune) bool {
	return unicode.IsSpace(value) || strings.ContainsRune(",，、;；/兼", value)
}

func positionKey(position string) string {
	switch position {
	case "支部书记", "党支部书记", "书记":
		return "secretaries"
	case "组织委员":
		return "organization_members"
	case "宣传委员":
		return "publicity_members"
	case "纪检委员", "纪律委员":
		return "discipline_members"
	default:
		return ""
	}
}

func joinNames(values []string) string {
	// 空姓名不占位，去重后按稳定顺序使用英文逗号拼接。
	clean := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			clean = append(clean, value)
		}
	}
	slices.Sort(clean)
	return strings.Join(slices.Compact(clean), ",")
}

func event() auditModels.Event {
	return auditModels.Event{
		ObjectType: "cpc_dashboard",
		Conditions: auditModels.Snapshot{
			"status": cpcpersonConstants.DefaultStatus, "group_by": "party_branch_id",
		},
	}
}

func recordSummary(audit *auditModels.Event, data cpcdashboardModels.Data) {
	for _, item := range data.Items {
		audit.ReturnedIDs = append(audit.ReturnedIDs, strconv.FormatUint(item.PartyBranchID, 10))
	}
	audit.ReturnedCount = int64(len(data.Items))
	audit.Conditions["matched_members"] = data.Totals.Total
}

func exportRecords(data cpcdashboardModels.Data) [][]string {
	// 固定表头顺序与页面一致，最后一列为导师姓名；下载包含表尾合计。
	header := make([]string, len(cpcdashboardConstants.Fields))
	for index, field := range cpcdashboardConstants.Fields {
		header[index] = field.Label
	}
	records := make([][]string, 1, len(data.Items)+2)
	records[0] = header
	for _, item := range data.Items {
		records = append(records, []string{
			item.PartyBranchName,
			strconv.FormatInt(item.FullMembers, 10),
			strconv.FormatInt(item.ProbationaryMembers, 10),
			strconv.FormatInt(item.Total, 10),
			item.Secretaries,
			item.OrganizationMembers,
			item.PublicityMembers,
			item.DisciplineMembers,
			item.Advisors,
		})
	}
	return append(records, []string{
		"合计",
		strconv.FormatInt(data.Totals.FullMembers, 10),
		strconv.FormatInt(data.Totals.ProbationaryMembers, 10),
		strconv.FormatInt(data.Totals.Total, 10),
		"", "", "", "", "",
	})
}
