package studentModels

import "time"

// OffCampusFields 是独立维护的校外住宿明细文本字段。
type OffCampusFields struct {
	// Reason 是校外住宿原因，最多 1000 个字符。
	Reason string `json:"reason" maxLength:"1000" example:"便于在校外开展实习"`
	// Category 是校外住宿类别，最多 64 个字符。
	Category string `json:"category" maxLength:"64" example:"租房"`
	// RentalMethod 是租住方式，最多 64 个字符。
	RentalMethod string `json:"rental_method" maxLength:"64" example:"整租"`
	// LandlordName 是房东姓名，最多 50 个字符。
	LandlordName string `json:"landlord_name" maxLength:"50" example:"虚构房东甲"`
	// LandlordPhone 是房东联系电话，最多 32 个字符。
	LandlordPhone string `json:"landlord_phone" maxLength:"32" example:"虚构电话0005"`
	// RoommateName 是合租人姓名，最多 50 个字符。
	RoommateName string `json:"roommate_name" maxLength:"50" example:"虚构合租人甲"`
	// RoommateUnit 是合租人单位，最多 100 个字符。
	RoommateUnit string `json:"roommate_unit" maxLength:"100" example:"虚构单位"`
	// RoommatePhone 是合租人联系电话，最多 32 个字符。
	RoommatePhone string `json:"roommate_phone" maxLength:"32" example:"虚构电话0006"`
	// Address 是校外居住地详细地址，最多 255 个字符。
	Address string `json:"address" maxLength:"255" example:"虚构路1号101室"`
}

// OffCampusValues 是校外住宿接口使用的日期和文本字段。
type OffCampusValues struct {
	OffCampusFields
	// StartDate 是校外住宿开始日期，使用 YYYY-MM-DD，未填写时为空。
	StartDate string `json:"start_date" maxLength:"10" example:"2026-09-01"`
	// EndDate 是校外住宿结束日期，使用 YYYY-MM-DD，未填写时为空。
	EndDate string `json:"end_date" maxLength:"10" example:"2027-06-30"`
}

// StudentOffCampusDetail 以工作空间和学生编号唯一标识一条校外住宿明细。
type StudentOffCampusDetail struct {
	// WorkspaceID 是所属工作空间。
	WorkspaceID string `gorm:"primaryKey"`
	// StudentID 是当前空间中的学生记录编号。
	StudentID uint64 `gorm:"primaryKey;autoIncrement:false"`
	OffCampusFields
	// StartDate 是可空的开始日期。
	StartDate *string `gorm:"type:date"`
	// EndDate 是可空的结束日期。
	EndDate   *string   `gorm:"type:date"`
	CreatedAt time.Time `gorm:"->;autoCreateTime:false"`
	UpdatedAt time.Time `gorm:"->;autoUpdateTime:false"`
}

// OffCampusReportRow 只投影校外住宿报表所需的学生档案与住宿明细列。
type OffCampusReportRow struct {
	BasicRecord
	StudentOffCampusDetail
	// HomeAddress 是家庭详细住址，仅在处理敏感信息权限通过后读取。
	HomeAddress string
}
