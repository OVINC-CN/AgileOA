package studentSvc

import (
	"strings"
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentModels"
)

func inputValues(input studentModels.Input) map[string]string {
	return map[string]string{
		"student_no":                  input.StudentNo,
		"name":                        input.Name,
		"grade":                       input.Grade,
		"status":                      input.Status,
		"education_level":             input.EducationLevel,
		"major":                       input.Major,
		"class_name":                  input.ClassName,
		"class_position":              input.ClassPosition,
		"admission_type":              input.AdmissionType,
		"arrival_month":               input.ArrivalMonth,
		"previous_school_or_employer": input.PreviousSchoolOrEmployer,
		"previous_graduation_month":   input.PreviousGraduationMonth,
		"advisor":                     input.Advisor,
		"advisor_no":                  input.AdvisorNo,
		"advisor_phone":               input.AdvisorPhone,
		"gender":                      input.Gender,
		"ethnicity":                   input.Ethnicity,
		"political_status":            input.PoliticalStatus,
		"phone":                       input.Phone,
		"qq":                          input.QQ,
		"email":                       input.Email,
		"accommodation_type":          input.AccommodationType,
		"campus":                      input.Campus,
		"building":                    input.Building,
		"room":                        input.Room,
		"bed":                         input.Bed,
		"origin":                      input.Origin,
		"native_place":                input.NativePlace,
		"guardian1_name":              input.Guardian1Name,
		"guardian1_relationship":      input.Guardian1Relationship,
		"guardian1_phone":             input.Guardian1Phone,
		"guardian2_name":              input.Guardian2Name,
		"guardian2_relationship":      input.Guardian2Relationship,
		"guardian2_phone":             input.Guardian2Phone,
	}
}

func inputFromValues(values map[string]string) studentModels.Input {
	return studentModels.Input{
		StudentNo:                values["student_no"],
		Name:                     values["name"],
		Grade:                    values["grade"],
		Status:                   values["status"],
		EducationLevel:           values["education_level"],
		Major:                    values["major"],
		ClassName:                values["class_name"],
		ClassPosition:            values["class_position"],
		AdmissionType:            values["admission_type"],
		ArrivalMonth:             values["arrival_month"],
		PreviousSchoolOrEmployer: values["previous_school_or_employer"],
		PreviousGraduationMonth:  values["previous_graduation_month"],
		Advisor:                  values["advisor"],
		AdvisorNo:                values["advisor_no"],
		AdvisorPhone:             values["advisor_phone"],
		Gender:                   values["gender"],
		Ethnicity:                values["ethnicity"],
		PoliticalStatus:          values["political_status"],
		Phone:                    values["phone"],
		QQ:                       values["qq"],
		Email:                    values["email"],
		AccommodationType:        values["accommodation_type"],
		Campus:                   values["campus"],
		Building:                 values["building"],
		Room:                     values["room"],
		Bed:                      values["bed"],
		Origin:                   values["origin"],
		NativePlace:              values["native_place"],
		Guardian1Name:            values["guardian1_name"],
		Guardian1Relationship:    values["guardian1_relationship"],
		Guardian1Phone:           values["guardian1_phone"],
		Guardian2Name:            values["guardian2_name"],
		Guardian2Relationship:    values["guardian2_relationship"],
		Guardian2Phone:           values["guardian2_phone"],
	}
}

func recordFromInput(input studentModels.Input) (studentModels.BasicRecord, error) {
	record := studentModels.BasicRecord{
		StudentNo:                input.StudentNo,
		Name:                     input.Name,
		Grade:                    input.Grade,
		Status:                   input.Status,
		EducationLevel:           input.EducationLevel,
		Major:                    input.Major,
		ClassName:                input.ClassName,
		ClassPosition:            input.ClassPosition,
		AdmissionType:            input.AdmissionType,
		ArrivalMonth:             input.ArrivalMonth,
		PreviousSchoolOrEmployer: input.PreviousSchoolOrEmployer,
		PreviousGraduationMonth:  input.PreviousGraduationMonth,
		Advisor:                  input.Advisor,
		AdvisorNo:                input.AdvisorNo,
		AdvisorPhone:             input.AdvisorPhone,
		Gender:                   input.Gender,
		Ethnicity:                input.Ethnicity,
		PoliticalStatus:          input.PoliticalStatus,
		Phone:                    input.Phone,
		QQ:                       input.QQ,
		Email:                    input.Email,
		Campus:                   input.Campus,
		Building:                 input.Building,
		Room:                     input.Room,
		Bed:                      input.Bed,
		Origin:                   input.Origin,
		NativePlace:              input.NativePlace,
		Guardian1Name:            input.Guardian1Name,
		Guardian1Relationship:    input.Guardian1Relationship,
		Guardian1Phone:           input.Guardian1Phone,
		Guardian2Name:            input.Guardian2Name,
		Guardian2Relationship:    input.Guardian2Relationship,
		Guardian2Phone:           input.Guardian2Phone,
	}
	code, ok := studentConstants.EnumCode("accommodation_type", input.AccommodationType)
	if !ok {
		return record, invalid("住宿类型无效")
	}
	record.AccommodationType = code
	return record, nil
}

func studentInfo(record studentModels.BasicRecord) studentModels.Info {
	return studentModels.Info{
		ID:        record.ID,
		Revision:  record.Revision,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
		Input: studentModels.Input{
			StudentNo:                record.StudentNo,
			Name:                     record.Name,
			Grade:                    record.Grade,
			Status:                   record.Status,
			EducationLevel:           record.EducationLevel,
			Major:                    record.Major,
			ClassName:                record.ClassName,
			ClassPosition:            record.ClassPosition,
			AdmissionType:            record.AdmissionType,
			ArrivalMonth:             record.ArrivalMonth,
			PreviousSchoolOrEmployer: record.PreviousSchoolOrEmployer,
			PreviousGraduationMonth:  record.PreviousGraduationMonth,
			Advisor:                  record.Advisor,
			AdvisorNo:                record.AdvisorNo,
			AdvisorPhone:             record.AdvisorPhone,
			BirthDate:                record.BirthDate,
			Gender:                   record.Gender,
			Ethnicity:                record.Ethnicity,
			PoliticalStatus:          record.PoliticalStatus,
			Phone:                    record.Phone,
			QQ:                       record.QQ,
			Email:                    record.Email,
			AccommodationType: studentConstants.EnumValue(
				"accommodation_type",
				record.AccommodationType,
			),
			Campus:                record.Campus,
			Building:              record.Building,
			Room:                  record.Room,
			Bed:                   record.Bed,
			Origin:                record.Origin,
			NativePlace:           record.NativePlace,
			Guardian1Name:         record.Guardian1Name,
			Guardian1Relationship: record.Guardian1Relationship,
			Guardian1Phone:        record.Guardian1Phone,
			Guardian2Name:         record.Guardian2Name,
			Guardian2Relationship: record.Guardian2Relationship,
			Guardian2Phone:        record.Guardian2Phone,
		},
	}
}

func cleanInput(input studentModels.Input) (studentModels.Input, error) {
	values := inputValues(input)
	for key, value := range values {
		value = strings.TrimSpace(value)
		field, _ := studentConstants.Lookup(key)
		if err := validateValue(field, value); err != nil {
			return input, err
		}
		if field.Type == "enum" && value == "" {
			value = "unknown"
		}
		values[key] = value
	}
	return inputFromValues(values), nil
}

func basicUpdates(input studentModels.Input) (map[string]any, error) {
	updates := map[string]any{}
	for key, value := range inputValues(input) {
		field, _ := studentConstants.Lookup(key)
		stored, err := storageValue(field, value)
		if err != nil {
			return nil, err
		}
		updates[key] = stored
	}
	return updates, nil
}

func validateValue(field studentConstants.Definition, value string) error {
	// 必填字段不接受空白或表示未填写的枚举。
	if field.Required &&
		(strings.TrimSpace(value) == "" || field.Type == "enum" && value == "unknown") {
		return invalid(field.Label + "不能为空")
	}
	if !validText(value, field.MaxLength, field.MaxLength > 500) {
		return invalid(field.Label + "无效或超出长度限制")
	}
	if field.Type == "month" && value != "" {
		month, err := time.Parse("2006-01", value)
		if err != nil || month.Year() < 1900 {
			return invalid(field.Label + "必须为 YYYY-MM 格式的有效年月（1900 至 9999 年）")
		}
	}
	if field.Type == "enum" {
		if _, ok := studentConstants.EnumCode(field.Key, value); !ok {
			return invalid(field.Label + "必须使用目录中的英文枚举代码")
		}
	}
	return nil
}

func storageValue(field studentConstants.Definition, value string) (string, error) {
	if field.Type == "enum" {
		code, ok := studentConstants.EnumCode(field.Key, value)
		if !ok {
			return "", invalid(field.Label + "枚举值无效")
		}
		return code, nil
	}
	return value, nil
}

func publicColumns() string {
	columns := []string{"s.id", "s.revision", "s.created_at", "s.updated_at"}
	for _, field := range studentConstants.Fields {
		if field.Group == studentConstants.Basic {
			columns = append(columns, field.Column)
		}
	}
	return strings.Join(columns, ",")
}
