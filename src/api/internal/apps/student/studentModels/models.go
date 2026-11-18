package studentModels

import "time"

// BasicRecord 是只包含基础字段的数据库投影，列表和详情不加载敏感字段。
type BasicRecord struct {
	ID uint64 `gorm:"primaryKey"`
	// StudentNo 是学号。
	StudentNo string
	// Name 是姓名。
	Name string
	// Avatar 是头像的私有 COS 对象键，空字符串表示未设置。
	Avatar string
	// Grade 是年级。
	Grade string
	// Status 是用户手动维护的学生状态。
	Status string
	// EducationLevel 是学历。
	EducationLevel string
	// Major 是专业。
	Major string
	// ClassName 是班级。
	ClassName string
	// ClassPosition 是班内任职。
	ClassPosition string
	// AdmissionType 是录取类型。
	AdmissionType string
	// ArrivalMonth 是来校年月，格式为 YYYY-MM，未填写时为空。
	ArrivalMonth string
	// PreviousSchoolOrEmployer 是毕（肄）业学校及单位。
	PreviousSchoolOrEmployer string
	// PreviousGraduationMonth 是既往毕（肄）业年月，格式为 YYYY-MM，未填写时为空。
	PreviousGraduationMonth string
	// Advisor 是导师姓名。
	Advisor string
	// AdvisorNo 是导师工号。
	AdvisorNo string
	// AdvisorPhone 是导师电话。
	AdvisorPhone string
	// BirthDate 是数据库根据身份证号码生成的出生日期。
	BirthDate string `gorm:"->"`
	// Gender 是性别。
	Gender string
	// Ethnicity 是民族。
	Ethnicity string
	// PoliticalStatus 是政治面貌。
	PoliticalStatus string
	// Phone 是电话号码。
	Phone string
	// QQ 是QQ号码。
	QQ string
	// Email 是邮箱。
	Email string
	// AccommodationType 是住宿类型。
	AccommodationType string
	// Campus 是住宿园区。
	Campus string
	// Building 是宿舍楼。
	Building string
	// Room 是宿舍号。
	Room string
	// Bed 是床位号。
	Bed string
	// Origin 是户籍所在地（省市）。
	Origin string
	// NativePlace 是独立填写的籍贯。
	NativePlace string
	// Guardian1Name 是家长1姓名。
	Guardian1Name string
	// Guardian1Relationship 是家长1关系。
	Guardian1Relationship string
	// Guardian1Phone 是家长1联系方式。
	Guardian1Phone string
	// Guardian2Name 是家长2姓名。
	Guardian2Name string
	// Guardian2Relationship 是家长2关系。
	Guardian2Relationship string
	// Guardian2Phone 是家长2联系方式。
	Guardian2Phone string
	Revision       uint64
	CreatedAt      time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt      time.Time `gorm:"->;autoUpdateTime:false"`
}

// Student 保存学生基础、敏感和毕业去向字段，敏感信息不嵌入基础响应。
type Student struct {
	BasicRecord
	WorkspaceID string
	// GraduationYear 是独立填写的毕业年份，不根据年级推算。
	GraduationYear string
	// GraduationType 是毕业去向类型的英文代码，空字符串表示尚未绑定。
	GraduationType string
	// GraduationTypeLabel 是用户填写的类型显示名称。
	GraduationTypeLabel string
	// GraduationDestination 是毕业去向，单独通过毕业去向权限维护。
	GraduationDestination string
	// IdentityNumber 是身份证号码，统一受敏感信息权限控制。
	IdentityNumber string
	// HomeAddress 是家庭详细住址，统一受敏感信息权限控制。
	HomeAddress string
	// OnlyChild 是是否独生子女，统一受敏感信息权限控制。
	OnlyChild string
	// SingleParent 是是否单亲家庭，统一受敏感信息权限控制。
	SingleParent string
	// Orphan 是是否孤儿，统一受敏感信息权限控制。
	Orphan string
	// StudentLoan 是是否助学贷款，统一受敏感信息权限控制。
	StudentLoan string
	// RegisteredPoverty 是是否建档立卡生，统一受敏感信息权限控制。
	RegisteredPoverty string
	// HasReligion 是是否有宗教信仰，统一受敏感信息权限控制。
	HasReligion string
	// ReligionCategory 是宗教信仰类别，统一受敏感信息权限控制。
	ReligionCategory string
	// ReligionIdentification 是宗教信仰标识，统一受敏感信息权限控制。
	ReligionIdentification string
	// ReligionStartYear 是宗教信仰起始年份，统一受敏感信息权限控制。
	ReligionStartYear string
	// FamilyReligion 是家庭成员宗教信仰情况，统一受敏感信息权限控制。
	FamilyReligion string
	// MedicalHistory 是治疗中的心理疾病或既往病史，统一受敏感信息权限控制。
	MedicalHistory string
	// SpecialCircumstances 是其他本人或家庭特殊情况，统一受敏感信息权限控制。
	SpecialCircumstances string
}
