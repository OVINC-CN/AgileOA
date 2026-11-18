package studentSvc

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

func saveOffCampus(
	tx *gorm.DB,
	workspaceID string,
	id uint64,
	isOffCampus bool,
	values studentModels.OffCampusValues,
	event *auditModels.Event,
) error {
	record, err := cleanOffCampus(values)
	if err != nil {
		return err
	}
	empty := record == (studentModels.StudentOffCampusDetail{})
	if !isOffCampus && !empty {
		return offCampusConflict("请清空校外住宿信息后再切换住宿类型")
	}
	before, err := loadOffCampus(tx, workspaceID, id)
	if err != nil {
		return err
	}
	if offCampusValues(record) == offCampusValues(before) {
		return nil
	}

	// 明细与学生档案同事务保存，清空时显式硬删除。
	if empty {
		err = tx.Delete(&before).Error
	} else {
		record.WorkspaceID, record.StudentID = workspaceID, id
		err = tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&record).Error
	}
	if err != nil {
		return err
	}

	// 住址及联系方式只写入加密快照，并与本次提交的学生敏感信息合并。
	if before.StudentID != 0 {
		if event.SensitiveBefore == nil {
			event.SensitiveBefore = auditModels.Snapshot{}
		}
		event.SensitiveBefore["off_campus_details"] = offCampusValues(before)
	}
	if !empty {
		if event.SensitiveAfter == nil {
			event.SensitiveAfter = auditModels.Snapshot{}
		}
		event.SensitiveAfter["off_campus_details"] = offCampusValues(record)
	}
	return nil
}

func loadOffCampus(tx *gorm.DB, workspaceID string, id uint64) (
	studentModels.StudentOffCampusDetail, error,
) {
	var detail studentModels.StudentOffCampusDetail
	err := tx.Where("workspace_id = ? AND student_id = ?", workspaceID, id).Take(&detail).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return detail, nil
	}
	return detail, err
}

func cleanOffCampus(details studentModels.OffCampusValues) (
	studentModels.StudentOffCampusDetail, error,
) {
	var record studentModels.StudentOffCampusDetail
	// 只校验固定字段和长度，住宿类别等普通文本不查枚举。
	fields := []struct {
		value     *string
		label     string
		limit     int
		multiline bool
	}{
		{&details.Reason, "校外住宿原因", 1000, true},
		{&details.Category, "校外住宿类别", 64, false},
		{&details.RentalMethod, "租住方式", 64, false},
		{&details.LandlordName, "房东姓名", 50, false},
		{&details.LandlordPhone, "房东联系电话", 32, false},
		{&details.RoommateName, "合租人姓名", 50, false},
		{&details.RoommateUnit, "合租人单位", 100, false},
		{&details.RoommatePhone, "合租人联系电话", 32, false},
		{&details.Address, "校外居住地详细地址", 255, true},
	}
	for _, field := range fields {
		*field.value = strings.TrimSpace(*field.value)
		if !validText(*field.value, field.limit, field.multiline) {
			return record, invalid(field.label + "无效或超出长度限制")
		}
	}

	// 起止日期各自可留空，填写时必须是合法日期。
	start, err := parseOffCampusDate(details.StartDate)
	if err != nil {
		return record, err
	}
	end, err := parseOffCampusDate(details.EndDate)
	if err != nil {
		return record, err
	}
	if start != nil && end != nil && *end < *start {
		return record, invalid("校外住宿结束日期不能早于开始日期")
	}
	record.OffCampusFields = details.OffCampusFields
	record.StartDate, record.EndDate = start, end
	return record, nil
}

func parseOffCampusDate(value string) (*string, error) {
	if value == "" {
		return nil, nil
	}
	date, err := time.Parse(time.DateOnly, value)
	if err != nil || date.Year() < 1000 || date.Format(time.DateOnly) != value {
		return nil, invalid("校外住宿日期须使用有效的 YYYY-MM-DD 格式，年份为 1000 至 9999")
	}
	// 向 DATE 列传入日期文本，避免驱动按连接时区换算后偏移一天。
	return &value, nil
}

func offCampusValues(detail studentModels.StudentOffCampusDetail) studentModels.OffCampusValues {
	return studentModels.OffCampusValues{
		OffCampusFields: detail.OffCampusFields,
		StartDate:       offCampusDateText(detail.StartDate),
		EndDate:         offCampusDateText(detail.EndDate),
	}
}

func offCampusDateText(date *string) string {
	if date == nil {
		return ""
	}
	// MySQL 驱动读取日期时会带时间部分，接口仅保留日期。
	return (*date)[:min(len(*date), len(time.DateOnly))]
}

func offCampusConflict(message string) error {
	return &response.BusinessError{Status: http.StatusConflict, Message: message}
}
