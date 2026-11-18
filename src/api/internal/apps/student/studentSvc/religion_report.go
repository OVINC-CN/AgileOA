package studentSvc

import (
	"strconv"
	"strings"
	"time"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentModels"
)

// 宗教信仰情况明细只读取报表使用的业务字段。
var religionReportColumns = []string{
	"s.name", "s.gender", "s.birth_date", "s.ethnicity", "s.native_place",
	"s.identity_number", "s.political_status", "s.grade", "s.major", "s.home_address",
	"s.phone", "s.religion_category", "s.religion_identification", "s.religion_start_year",
	"s.family_religion",
}

func religionReportRecords(students []studentModels.Student) [][]string {
	records := make([][]string, 1, len(students)+1)
	records[0] = []string{
		"序号", "姓名", "性别", "出生年月", "民族", "籍贯", "身份证号码", "政治面貌",
		"入学年份", "专业", "家庭详细住址", "联系电话", "XJ类别", "XJ标识",
		"XJ起始年份", "家庭成员XJ情况",
	}
	for index, student := range students {
		// 出生日期按月输出。能识别为年份的年级去除后缀，自定义年级沿用原值。
		var birthMonth string
		if date, err := time.Parse(time.DateOnly, student.BirthDate); err == nil {
			birthMonth = date.Format("2006-01")
		}
		enrollmentYear := student.Grade
		year := strings.TrimSuffix(strings.TrimSuffix(student.Grade, "级"), "年")
		if _, err := time.Parse("2006", year); err == nil {
			enrollmentYear = year
		}
		records = append(records, []string{
			strconv.Itoa(index + 1),
			student.Name,
			student.Gender,
			birthMonth,
			student.Ethnicity,
			student.NativePlace,
			student.IdentityNumber,
			student.PoliticalStatus,
			enrollmentYear,
			student.Major,
			student.HomeAddress,
			student.Phone,
			student.ReligionCategory,
			student.ReligionIdentification,
			student.ReligionStartYear,
			student.FamilyReligion,
		})
	}
	return records
}
