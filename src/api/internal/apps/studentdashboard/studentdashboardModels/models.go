package studentdashboardModels

import "github.com/OVINC-CN/AgileOA/src/api/internal/response"

// Counts 保存各专业人数及全部专业的总数。
type Counts struct {
	// MajorCounts 以专业名称为键，空字符串表示历史记录未填写专业；缺少的键按零计数。
	MajorCounts map[string]int64 `json:"major_counts"`
	// Total 是全部专业的在籍学生人数之和。
	Total int64 `json:"total" example:"40"`
}

// ClassSummary 是当前空间一个班级的学生专业汇总行。
type ClassSummary struct {
	Counts
	// ClassName 是班级名称，历史空值保留为空字符串。
	ClassName string `json:"class_name" example:"2026级一班"`
	// Monitors 是在籍学生中班内任职包含独立班长职务的姓名，去重排序后以英文逗号拼接。
	Monitors string `json:"monitors" example:"虚构学生甲,虚构学生乙"`
}

type Data struct {
	// Majors 是按名称升序排列的专业列，历史空专业仍作为独立列保留。
	Majors []string `json:"majors" example:"会计学,审计学"`
	// Items 是在籍学生按班级名称升序排列的汇总行，无在籍学生的班级不展示。
	Items []ClassSummary `json:"items"`
	// Totals 是各专业及全部班级的在籍学生人数合计。
	Totals Counts `json:"totals"`
}

type Field struct {
	// Key 是固定字段代码，不接受外部表名或列名。
	Key string `json:"key" example:"class_name"`
	// Label 是字段中文名称。
	Label string `json:"label" example:"班级"`
	// Type 是字段类型。
	Type string `json:"type" example:"string"`
	// Description 是字段口径与空值说明。
	Description string `json:"description" example:"班级名称"`
	// Example 是字段示例。
	Example string `json:"example" example:"2026级一班"`
}

type FileData struct {
	// Filename 是 XLSX 下载文件名。
	Filename string `json:"filename" example:"学生专业表.xlsx"`
	// ContentType 是 XLSX 标准媒体类型。
	ContentType string `json:"content_type" example:"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"`
	// Content 是全部单元格按文本存储的 XLSX 二进制 Base64。
	Content string `json:"content" example:"UEsDBAoAAAAAAA=="`
}

type DataResponse struct{ response.Envelope[Data] }
type FieldsResponse struct{ response.Envelope[[]Field] }
type FileResponse struct{ response.Envelope[FileData] }
