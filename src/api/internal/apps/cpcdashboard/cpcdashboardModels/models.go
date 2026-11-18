package cpcdashboardModels

import "github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"

// Counts 保存正式党员、预备党员及两者合计。
type Counts struct {
	// FullMembers 是在编正式党员人数。
	FullMembers int64 `json:"full_members" example:"34"`
	// ProbationaryMembers 是在编预备党员人数。
	ProbationaryMembers int64 `json:"probationary_members" example:"6"`
	// Total 是正式党员与预备党员人数之和，不包含其他发展阶段。
	Total int64 `json:"total" example:"40"`
}

// BranchSummary 是当前空间内一个党支部的固定汇总行。
type BranchSummary struct {
	Counts
	// PartyBranchID 是当前空间党支部编号。
	PartyBranchID uint64 `json:"party_branch_id" example:"1"`
	// PartyBranchName 是党支部简称。
	PartyBranchName string `json:"party_branch_name" example:"示例学生党支部"`
	// Secretaries 是在编人员中担任支部书记的姓名，去重后以英文逗号拼接。
	Secretaries string `json:"secretaries" example:"虚构学生甲"`
	// OrganizationMembers 是在编组织委员姓名，去重后以英文逗号拼接。
	OrganizationMembers string `json:"organization_members" example:"虚构学生乙"`
	// PublicityMembers 是在编宣传委员姓名，去重后以英文逗号拼接。
	PublicityMembers string `json:"publicity_members" example:"虚构学生丙"`
	// DisciplineMembers 是在编纪检或纪律委员姓名，去重后以英文逗号拼接。
	DisciplineMembers string `json:"discipline_members" example:"虚构学生丁"`
	// Advisors 是支部内全部在编人员关联的导师姓名，去重后以英文逗号拼接。
	Advisors string `json:"advisors" example:"虚构导师甲,虚构导师乙"`
}

type Data struct {
	// Items 包含全部党支部，按优先级倒序、同优先级按编号倒序，空支部仍保留。
	Items []BranchSummary `json:"items"`
	// Totals 是全部汇总行的党员人数合计。
	Totals Counts `json:"totals"`
}

type Field struct {
	// Key 是只读的固定字段代码，不接受外部表名或列名。
	Key string `json:"key" example:"party_branch_name"`
	// Label 是字段中文名称。
	Label string `json:"label" example:"支部名称"`
	// Type 是字段类型。
	Type string `json:"type" example:"string"`
	// Description 是字段口径与空值说明。
	Description string `json:"description" example:"党支部简称"`
	// Example 是字段示例。
	Example string `json:"example" example:"示例学生党支部"`
}

type FileData struct {
	// Filename 是 XLSX 下载文件名。
	Filename string `json:"filename" example:"支部党员表.xlsx"`
	// ContentType 是 XLSX 标准媒体类型。
	ContentType string `json:"content_type" example:"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"`
	// Content 是全部单元格按文本存储的 XLSX 二进制 Base64。
	Content string `json:"content" example:"UEsDBAoAAAAAAA=="`
}

type DataResponse struct{ response.Envelope[Data] }
type FieldsResponse struct{ response.Envelope[[]Field] }
type FileResponse struct{ response.Envelope[FileData] }
