package studentSvc

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentModels"
)

func exportOffCampusReport(
	scope *gorm.DB,
	event *auditModels.Event,
) (studentModels.FileData, error) {
	// 未填写明细的校外住宿学生也要导出。按复合租户主键关联，确保最多一条明细。
	var students []studentModels.OffCampusReportRow
	err := scope.Joins(
		"LEFT JOIN student_off_campus_details AS d ON d.workspace_id = s.workspace_id AND d.student_id = s.id",
	).Select([]string{
		"s.id", "s.name", "s.gender", "s.ethnicity", "s.major", "s.class_name", "s.student_no",
		"s.phone", "s.email", "s.qq", "s.room", "s.home_address",
		"s.guardian1_name", "s.guardian1_relationship", "s.guardian1_phone",
		"s.guardian2_name", "s.guardian2_relationship", "s.guardian2_phone",
		"s.advisor", "s.advisor_no", "s.advisor_phone",
		"d.reason", "d.start_date", "d.end_date", "d.category", "d.rental_method",
		"d.landlord_name", "d.landlord_phone", "d.roommate_name", "d.roommate_unit",
		"d.roommate_phone", "d.address",
	}).Scan(&students).Error
	if err != nil {
		return studentModels.FileData{}, err
	}
	for _, student := range students {
		event.ReturnedIDs = append(event.ReturnedIDs, strconv.FormatUint(student.ID, 10))
	}
	return xlsxFile("校外住宿明细表.xlsx", offCampusReportRecords(students))
}

func offCampusReportRecords(students []studentModels.OffCampusReportRow) [][]string {
	records := make([][]string, 1, len(students)+1)
	records[0] = []string{
		"序号", "学院", "提交日期", "姓名", "性别", "民族", "专业", "班级", "学号",
		"本人手机号", "电子邮箱/QQ/微信", "现住宿舍号", "校外住宿原因", "校外住宿起止时间",
		"校外住宿类别", "租住方式", "房东姓名", "房东联系电话", "合租人（若有）姓名",
		"合租人（若有）单位", "合租人（若有）联系电话", "校外居住地详细地址",
		"家长称谓及姓名", "家长联系电话", "家庭详细住址", "日常管理措施", "是否已办理退宿手续",
		"未办理退宿原因", "是否有家人或者伴侣同住", "导师姓名", "导师工号", "导师电话",
	}
	for index, student := range students {
		// 优先导出第一位家长，只有第一位全空时才使用第二位家长。
		guardian := joinReportValues(" ", student.Guardian1Relationship, student.Guardian1Name)
		guardianPhone := student.Guardian1Phone
		if guardian == "" && guardianPhone == "" {
			guardian = joinReportValues(" ", student.Guardian2Relationship, student.Guardian2Name)
			guardianPhone = student.Guardian2Phone
		}
		records = append(records, []string{
			strconv.Itoa(index + 1),
			"",
			"", // 序号、学院、提交日期。
			student.Name,
			student.Gender,
			student.Ethnicity,
			student.Major,
			student.ClassName,
			student.StudentNo,
			student.Phone,
			joinReportValues("/", student.Email, student.QQ),
			student.Room,
			student.Reason,
			offCampusReportPeriod(student.StartDate, student.EndDate),
			student.Category,
			student.RentalMethod,
			student.LandlordName,
			student.LandlordPhone,
			student.RoommateName,
			student.RoommateUnit,
			student.RoommatePhone,
			student.Address,
			guardian,
			guardianPhone,
			student.HomeAddress,
			"",
			"",
			"",
			"", // 日常管理措施、退宿手续、未办理原因、同住情况。
			student.Advisor,
			student.AdvisorNo,
			student.AdvisorPhone,
		})
	}
	return records
}

func joinReportValues(separator string, values ...string) string {
	return strings.Join(
		slices.DeleteFunc(values, func(value string) bool { return value == "" }),
		separator,
	)
}

func offCampusReportPeriod(start, end *string) string {
	startDate, startErr := time.Parse(time.DateOnly, offCampusDateText(start))
	endDate, endErr := time.Parse(time.DateOnly, offCampusDateText(end))
	switch {
	case startErr == nil && endErr == nil:
		return startDate.Format(time.DateOnly) + " 至 " + endDate.Format(time.DateOnly)
	case startErr == nil:
		return startDate.Format(time.DateOnly) + "起"
	case endErr == nil:
		return "至" + endDate.Format(time.DateOnly)
	default:
		return ""
	}
}
