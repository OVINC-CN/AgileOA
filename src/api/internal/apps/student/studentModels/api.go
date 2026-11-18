package studentModels

import (
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

// Input 是学生基本信息固定字段目录，不包含敏感字段。
type Input struct {
	// StudentNo 是学号，最多 32 个字符。
	StudentNo string `json:"student_no" validate:"required,max=32" example:"S20260001"`
	// Name 是姓名，最多 50 个字符。
	Name string `json:"name" validate:"required,max=50" example:"虚构学生甲"`
	// Grade 是年级，最多 16 个字符。
	Grade string `json:"grade" validate:"required,max=16" example:"2021级"`
	// Status 是用户手动维护的学生状态，最多 8 个字符。
	Status string `json:"status" validate:"required,max=8" example:"在读"`
	// EducationLevel 是学历，最多 32 个字符。
	EducationLevel string `json:"education_level" validate:"required,max=32" example:"博士"`
	// Major 是专业，最多 100 个字符。
	Major string `json:"major" validate:"required,max=100" example:"物流工程与管理"`
	// ClassName 是班级，最多 64 个字符。
	ClassName string `json:"class_name" validate:"required,max=64" example:"2026级一班"`
	// ClassPosition 是班内任职，最多 64 个字符。
	ClassPosition string `json:"class_position" validate:"max=64" example:"班长"`
	// AdmissionType 是录取类型，最多 32 个字符。
	AdmissionType string `json:"admission_type" validate:"required,max=32" example:"非定向"`
	// ArrivalMonth 是来校年月，格式为 YYYY-MM，年份为 1900 至 9999，未填写时为空。
	ArrivalMonth string `json:"arrival_month" validate:"max=7" example:"2026-09"`
	// PreviousSchoolOrEmployer 是毕（肄）业学校及单位，最多 255 个字符。
	PreviousSchoolOrEmployer string `json:"previous_school_or_employer" validate:"max=255" example:"虚构大学"`
	// PreviousGraduationMonth 是既往毕（肄）业年月，格式为 YYYY-MM，年份为 1900 至 9999。
	PreviousGraduationMonth string `json:"previous_graduation_month" validate:"max=7" example:"2026-06"`
	// Advisor 是导师姓名，最多 50 个字符。
	Advisor string `json:"advisor" validate:"max=50" example:"虚构导师甲"`
	// AdvisorNo 是导师工号，最多 32 个字符。
	AdvisorNo string `json:"advisor_no" validate:"max=32" example:"T000001"`
	// AdvisorPhone 是导师电话，最多 32 个字符。
	AdvisorPhone string `json:"advisor_phone" validate:"max=32" example:"虚构电话0004"`
	// BirthDate 是根据身份证号码生成的只读出生日期，无法解析时为空。
	BirthDate string `json:"birth_date" readonly:"true" maxLength:"10" example:"2000-01-01"`
	// Gender 是性别，最多 16 个字符。
	Gender string `json:"gender" validate:"required,max=16" example:"女"`
	// Ethnicity 是民族，最多 32 个字符。
	Ethnicity string `json:"ethnicity" validate:"required,max=32" example:"汉族"`
	// PoliticalStatus 是政治面貌，最多 32 个字符。
	PoliticalStatus string `json:"political_status" validate:"max=32" example:"群众"`
	// Phone 是电话号码，最多 32 个字符。
	Phone string `json:"phone" validate:"max=32" example:"虚构电话0001"`
	// QQ 是QQ号码，最多 20 个字符。
	QQ string `json:"qq" validate:"max=20" example:"10000001"`
	// Email 是邮箱，最多 254 个字符。
	Email string `json:"email" validate:"max=254" example:"student@example.com"`
	// AccommodationType 是住宿类型：unknown 未知、on_campus 校内、off_campus 校外、
	// joint_abroad 出国联培。必填，不接受空值或 unknown。
	AccommodationType string `json:"accommodation_type" validate:"required,max=16" example:"joint_abroad"`
	// Campus 是住宿园区，最多 64 个字符。
	Campus string `json:"campus" validate:"max=64" example:"虚构园区"`
	// Building 是宿舍楼，最多 32 个字符。
	Building string `json:"building" validate:"max=32" example:"1号楼"`
	// Room 是宿舍号，最多 32 个字符。
	Room string `json:"room" validate:"max=32" example:"101"`
	// Bed 是床位号，最多 16 个字符。
	Bed string `json:"bed" validate:"max=16" example:"1"`
	// Origin 是户籍所在地（省市），最多 128 个字符。
	Origin string `json:"origin" validate:"max=128" example:"虚构省虚构市"`
	// NativePlace 是独立填写的籍贯，最多 128 个字符。
	NativePlace string `json:"native_place" validate:"max=128" example:"虚构省虚构县"`
	// Guardian1Name 是家长1姓名，最多 50 个字符。
	Guardian1Name string `json:"guardian1_name" validate:"max=50" example:"虚构家长甲"`
	// Guardian1Relationship 是家长1关系，最多 32 个字符。
	Guardian1Relationship string `json:"guardian1_relationship" validate:"max=32" example:"父亲"`
	// Guardian1Phone 是家长1联系方式，最多 32 个字符。
	Guardian1Phone string `json:"guardian1_phone" validate:"max=32" example:"虚构电话0002"`
	// Guardian2Name 是家长2姓名，最多 50 个字符。
	Guardian2Name string `json:"guardian2_name" validate:"max=50" example:"虚构家长乙"`
	// Guardian2Relationship 是家长2关系，最多 32 个字符。
	Guardian2Relationship string `json:"guardian2_relationship" validate:"max=32" example:"母亲"`
	// Guardian2Phone 是家长2联系方式，最多 32 个字符。
	Guardian2Phone string `json:"guardian2_phone" validate:"max=32" example:"虚构电话0003"`
}

// CreateRequest 按权限创建基本信息及包含必填身份证号码的完整敏感信息。
type CreateRequest struct {
	Input
	// Sensitive 必须包含全部十四个敏感字段及非空身份证号码，另需处理敏感信息权限。
	Sensitive []FieldValue `json:"sensitive" validate:"required,len=14,dive"`
}

// UpdateRequest 通过版本避免覆盖其他人的修改。
type UpdateRequest struct {
	Input
	// AdvisorNo 是导师工号，最多 32 个字符，省略时保留原值。
	AdvisorNo *string `json:"advisor_no,omitempty" validate:"omitempty,max=32" example:"T000001"`
	// AdvisorPhone 是导师电话，最多 32 个字符，省略时保留原值。
	AdvisorPhone *string `json:"advisor_phone,omitempty" validate:"omitempty,max=32" example:"虚构电话0004"`
	// ArrivalMonth 是来校年月，省略时保留原值，空字符串清空。
	ArrivalMonth *string `json:"arrival_month,omitempty" validate:"omitempty,max=7" example:"2026-09"`
	// PreviousSchoolOrEmployer 是毕（肄）业学校及单位，省略时保留原值，空字符串清空。
	PreviousSchoolOrEmployer *string `json:"previous_school_or_employer,omitempty" validate:"omitempty,max=255" example:"虚构大学"`
	// PreviousGraduationMonth 是既往毕（肄）业年月，省略时保留原值，空字符串清空。
	PreviousGraduationMonth *string `json:"previous_graduation_month,omitempty" validate:"omitempty,max=7" example:"2026-06"`
	// Revision 是最近读取的学生版本，所有基础与隐私修改都会递增。
	Revision uint64 `json:"revision" validate:"required" example:"1"`
	// Sensitive 是可选的全部十四个敏感字段，省略时保持原值，提交时另需处理敏感信息权限。
	Sensitive []FieldValue `json:"sensitive,omitempty" validate:"omitempty,len=14,dive"`
	// OffCampusDetails 是可选的校外住宿明细，省略时保持原值，提交空明细时清除记录。
	OffCampusDetails *OffCampusValues `json:"off_campus_details,omitempty"`
}

// Info 是列表与基础详情共用的公开字段目录，不包含隐私字段。
type Info struct {
	// ID 是学生记录编号，仅在指定工作空间中使用。
	ID uint64 `json:"id" example:"1"`
	Input
	// Revision 是当前并发校验版本。
	Revision uint64 `json:"revision" example:"1"`
	// CreatedAt 是 UTC 创建时间。
	CreatedAt time.Time `json:"created_at" example:"2026-11-03T00:00:00Z"`
	// UpdatedAt 是 UTC 最近修改时间。
	UpdatedAt time.Time `json:"updated_at" example:"2026-11-03T00:00:00Z"`
}

// DetailData 在基本详情中提供按需签发的头像链接。
type DetailData struct {
	Info
	// AvatarUploads 是需要显式清理的待完成或待删除头像，不包含凭据和对象键。
	AvatarUploads []attachmentModels.Info `json:"avatar_uploads"`
	// AvatarURL 是短时 CDN 头像链接，无可用头像时为空。
	AvatarURL string `json:"avatar_url" example:"https://cdn.example.com/avatar.png?eo-sign=..."`
	// OffCampusDetails 是校外住宿明细，仅校外住宿学生返回，未填写的字段为空字符串。
	OffCampusDetails *OffCampusValues `json:"off_campus_details,omitempty"`
}

// ProfileData 仅提供学生本人的编号和固定学籍字段。
type ProfileData struct {
	// ID 是学生记录编号，仅在绑定的工作空间中使用。
	ID uint64 `json:"id" example:"1"`
	// StudentNo 是学号，最多 32 个字符。
	StudentNo string `json:"student_no" example:"S20260001"`
	// Name 是姓名，最多 50 个字符。
	Name string `json:"name" example:"虚构学生甲"`
	// Grade 是年级，最多 16 个字符。
	Grade string `json:"grade" example:"2026级"`
	// Status 是用户手动维护的学生状态，最多 8 个字符。
	Status string `json:"status" example:"在籍"`
	// EducationLevel 是学历，最多 32 个字符。
	EducationLevel string `json:"education_level" example:"博士"`
	// Major 是专业，最多 100 个字符。
	Major string `json:"major" example:"物流工程与管理"`
	// ClassName 是班级，最多 64 个字符。
	ClassName string `json:"class_name" example:"2026级一班"`
	// ClassPosition 是班内任职，最多 64 个字符。
	ClassPosition string `json:"class_position" example:"班长"`
	// AdmissionType 是录取类型，最多 32 个字符。
	AdmissionType string `json:"admission_type" example:"非定向"`
	// ArrivalMonth 是来校年月，格式为 YYYY-MM，年份为 1900 至 9999，未填写时为空。
	ArrivalMonth string `json:"arrival_month" example:"2026-09"`
}

// ProfileResponse 仅返回学生本人的学籍信息。
type ProfileResponse struct{ response.Envelope[ProfileData] }

// MutationData 不隐含列表、详情或隐私读取权限。
type MutationData struct {
	// ID 是本次操作的学生编号。
	ID uint64 `json:"id" example:"1"`
	// Revision 是操作后的并发校验版本。
	Revision uint64 `json:"revision" example:"2"`
}

// ListQuery 仅允许固定目录中的筛选维度。
type ListQuery struct {
	pagination.Query
	Keyword string
	Filters map[string][]string
}

// ListData 是不含隐私字段的分页明细。
type ListData struct {
	// Items 是本页学生基础信息，空结果为数组。
	Items []Info `json:"items"`
	// Total 是筛选后的学生总数。
	Total int64 `json:"total" example:"20"`
	// Page 是当前页码。
	Page int `json:"page" example:"1"`
	// PageSize 是当前每页条数。
	PageSize int `json:"page_size" example:"10"`
}

// FieldValue 的字段代码必须来自固定目录，不能选择任意数据库列。
type FieldValue struct {
	// Field 是固定敏感字段代码，具体名称及示例由字段目录接口提供。
	Field string `json:"field" validate:"required" example:"identity_number"`
	// Value 是字段值，身份证号码必填，其他字段可用空字符串清除；内置选项使用英文代码。
	Value string `json:"value" example:"FICTIONAL000000001"`
}

// SensitiveRequest 完整替换十四个敏感字段，身份证号码必填，其他字段可留空。
type SensitiveRequest struct {
	// Revision 是最近读取的学生版本。
	Revision uint64 `json:"revision" validate:"required" example:"1"`
	// Values 必须包含全部十四个敏感字段，不允许缺失、重复或提交基础字段。
	Values []FieldValue `json:"values" validate:"required,len=14,dive"`
}

// SensitiveData 只返回统一授权的敏感信息。
type SensitiveData struct {
	// StudentID 是当前空间中的学生记录编号。
	StudentID uint64 `json:"student_id" example:"1"`
	// Revision 是当前学生版本。
	Revision uint64 `json:"revision" example:"1"`
	// Values 只包含统一敏感信息目录中的十四个字段。
	Values []FieldValue `json:"values"`
}

// Option 是从当前空间明细去重得到的值和显示名称。
type Option struct {
	// Value 是实际字段值或内置英文代码。
	Value string `json:"value" example:"班长"`
	// Label 是简体中文显示名称或用户填写的名称。
	Label string `json:"label" example:"班长"`
}

// Field 描述导入导出和填写使用的固定字段目录。
type Field struct {
	// Key 是稳定的英文字段代码。
	Key string `json:"key" example:"class_position"`
	// Label 是模板列名和表单显示名称。
	Label string `json:"label" example:"班内任职"`
	// Group 是信息类别：basic 为基本信息，sensitive 为敏感信息。
	Group string `json:"group" example:"basic"`
	// Type 是表单类型：text、enum、date 或 month；业务字段统一使用 VARCHAR 存储。
	Type string `json:"type" example:"text"`
	// Required 表示写入时必须提供非空值，未知枚举不视为已填写。
	Required bool `json:"required" example:"true"`
	// MaxLength 是请求字段值最多允许的字符数，是否类英文代码最长为 unknown 的 7 个字符。
	MaxLength int `json:"max_length" example:"100"`
	// StorageLength 是数据库 VARCHAR 列允许的字符数，固定枚举使用四字符以内的文本代码。
	StorageLength int `json:"storage_length" example:"4"`
	// Enumerable 表示支持从明细查询筛选选项。
	Enumerable bool `json:"enumerable" example:"true"`
	// Example 是虚构示例，不作为默认业务数据。
	Example string `json:"example" example:"班长"`
	// Options 是固定英文代码的中文名称，自由文本字段为空数组。
	Options []Option `json:"options"`
}

// StatisticsData 保存当前筛选结果的四项统计，筛选权限由业务服务校验。
type StatisticsData struct {
	// Total 是符合条件的学生数量。
	Total int64 `json:"total" example:"120"`
	// ClassCount 是不含空班级的班级名称去重数量。
	ClassCount int64 `json:"class_count" example:"6"`
	// ReligiousCount 是宗教信仰为是的学生数量。
	ReligiousCount int64 `json:"religious_count" example:"3"`
	// OffCampusCount 是校外住宿的学生数量。
	OffCampusCount int64 `json:"off_campus_count" example:"12"`
}

// FileData 在统一协议内返回 Base64 编码的 XLSX，由浏览器解码并保存。
type FileData struct {
	// Filename 是 XLSX 下载文件名。
	Filename string `json:"filename" example:"students.xlsx"`
	// ContentType 是 XLSX 标准媒体类型。
	ContentType string `json:"content_type" example:"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"`
	// Content 是 Base64 编码的 XLSX，全部单元格按文本存储，日期使用 yyyy-mm 或 yyyy-mm-dd。
	Content string `json:"content" example:"UEsDBAoAAAAAAA=="`
}

// ImportRequest 仅接受固定 XLSX 模板，新增记录，不覆盖已有学生。
type ImportRequest struct {
	// Content 是 Base64 编码的 XLSX，字段及校验沿用固定模板。
	Content string `json:"content" validate:"required" example:"UEsDBAoAAAAAAA=="`
}

// ImportData 是全部成功提交后的导入结果，任一错误整批回滚。
type ImportData struct {
	// Created 是新增学生数量。
	Created int `json:"created" example:"1"`
}

// 各接口通过统一响应复用公共字段。
type DetailResponse struct{ response.Envelope[DetailData] }
type MutationResponse struct {
	response.Envelope[MutationData]
}
type ListResponse struct{ response.Envelope[ListData] }
type SensitiveResponse struct {
	response.Envelope[SensitiveData]
}
type OptionsResponse struct{ response.Envelope[[]Option] }
type FieldsResponse struct{ response.Envelope[[]Field] }
type StatisticsResponse struct {
	response.Envelope[StatisticsData]
}
type FileResponse struct{ response.Envelope[FileData] }
type ImportResponse struct{ response.Envelope[ImportData] }
