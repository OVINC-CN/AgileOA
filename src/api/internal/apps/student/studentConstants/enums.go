package studentConstants

// 固定枚举使用 VARCHAR(4) 文本存储，API 保留稳定英文代码。
const (
	EnumUnknown              = ""
	FlagYes                  = "yes"
	FlagNo                   = "no"
	AccommodationOnCampus    = "in"
	AccommodationOffCampus   = "out"
	AccommodationJointAbroad = "abrd"
)

// StatusEnrolled 是学生档案中手动填写的在籍状态。
const StatusEnrolled = "在籍"

// EnumCode 将固定目录中的英文枚举转换为数据库文本，不支持任意枚举字段。
func EnumCode(field, value string) (string, bool) {
	definition, ok := Lookup(field)
	if !ok || definition.Type != "enum" {
		return EnumUnknown, false
	}
	if value == "" || value == "unknown" {
		return EnumUnknown, true
	}
	switch field {
	case "only_child",
		"single_parent",
		"orphan",
		"student_loan",
		"registered_poverty",
		"has_religion":
		switch value {
		case "yes":
			return FlagYes, true
		case "no":
			return FlagNo, true
		}
	case "accommodation_type":
		switch value {
		case "on_campus":
			return AccommodationOnCampus, true
		case "off_campus":
			return AccommodationOffCampus, true
		case "joint_abroad":
			return AccommodationJointAbroad, true
		}
	}
	return EnumUnknown, false
}

// EnumValue 将数据库文本枚举转换为英文协议代码，空字符串对应未填写或未知。
func EnumValue(field string, code string) string {
	switch field {
	case "only_child",
		"single_parent",
		"orphan",
		"student_loan",
		"registered_poverty",
		"has_religion":
		switch code {
		case FlagYes:
			return "yes"
		case FlagNo:
			return "no"
		}
	case "accommodation_type":
		switch code {
		case AccommodationOnCampus:
			return "on_campus"
		case AccommodationOffCampus:
			return "off_campus"
		case AccommodationJointAbroad:
			return "joint_abroad"
		}
	}
	return "unknown"
}
