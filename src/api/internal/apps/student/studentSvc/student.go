package studentSvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"unicode"
	"unicode/utf8"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectUtils"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/attachment/attachmentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpcperson/cpcpersonUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/notification/notificationUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/student/studentUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
	"github.com/OVINC-CN/AgileOA/src/api/internal/storage"
)

// Create 按独立权限在一个事务中创建基本信息及包含身份证号码的完整敏感信息。
func Create(ctx context.Context, workspaceID string, request studentModels.CreateRequest) (
	studentModels.MutationData, error,
) {
	var result studentModels.MutationData
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.StudentCreate,
		studentEvent(workspaceID, 0),
		func(tx *gorm.DB, principal accessModels.Principal, event *auditModels.Event) error {
			// 校验必填敏感信息并在空间锁内复核处理权限。
			if request.Sensitive == nil {
				return invalid("身份证号码不能为空")
			}
			if err := accessUtils.Require(
				tx,
				principal,
				accessConstants.StudentSensitiveHandle,
			); err != nil {
				return err
			}
			sensitive, err := validateSensitiveReplacement(request.Sensitive)
			if err != nil {
				return err
			}

			// 保存基础、敏感信息及同事务审计快照。
			student, err := create(tx, workspaceID, request.Input, "")
			if err != nil {
				return err
			}
			student.BirthDate, err = saveSensitive(tx, workspaceID, student.ID, sensitive)
			if err != nil {
				return err
			}
			student.BasicRecord, err = basicDetail(tx, workspaceID, student.ID)
			if err != nil {
				return err
			}
			event.SensitiveAfter = sensitiveSnapshot(sensitive)
			event.ObjectID = strconv.FormatUint(student.ID, 10)
			event.After = basicSnapshot(student)
			result = studentModels.MutationData{ID: student.ID, Revision: student.Revision}
			return nil
		})
	return result, err
}

// Update 使用版本保护在同一事务中更新基本信息、校外住宿明细及已授权的敏感信息。
func Update(ctx context.Context, workspaceID string, id uint64,
	request studentModels.UpdateRequest,
) (studentModels.MutationData, error) {
	var result studentModels.MutationData
	var timelinePersonIDs []uint64
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.StudentUpdate,
		studentEvent(workspaceID, id),
		func(tx *gorm.DB, principal accessModels.Principal, event *auditModels.Event) error {
			before, err := basicDetail(tx, workspaceID, id)
			if err != nil {
				return err
			}
			if before.Revision != request.Revision {
				return stale()
			}
			var sensitive map[string]string
			if request.Sensitive != nil {
				if err := accessUtils.Require(
					tx,
					principal,
					accessConstants.StudentSensitiveHandle,
				); err != nil {
					return err
				}
				sensitive, err = validateSensitiveReplacement(request.Sensitive)
				if err != nil {
					return err
				}
				event.SensitiveBefore, err = loadSensitive(tx, workspaceID, id)
				if err != nil {
					return err
				}
				event.SensitiveAfter = sensitiveSnapshot(sensitive)
			}
			// 兼容省略新增基础字段的客户端，显式空字符串仍可清除原值。
			request.Input.AdvisorNo = before.AdvisorNo
			if request.AdvisorNo != nil {
				request.Input.AdvisorNo = *request.AdvisorNo
			}
			request.Input.AdvisorPhone = before.AdvisorPhone
			if request.AdvisorPhone != nil {
				request.Input.AdvisorPhone = *request.AdvisorPhone
			}
			request.Input.ArrivalMonth = before.ArrivalMonth
			if request.ArrivalMonth != nil {
				request.Input.ArrivalMonth = *request.ArrivalMonth
			}
			request.Input.PreviousSchoolOrEmployer = before.PreviousSchoolOrEmployer
			if request.PreviousSchoolOrEmployer != nil {
				request.Input.PreviousSchoolOrEmployer = *request.PreviousSchoolOrEmployer
			}
			request.Input.PreviousGraduationMonth = before.PreviousGraduationMonth
			if request.PreviousGraduationMonth != nil {
				request.Input.PreviousGraduationMonth = *request.PreviousGraduationMonth
			}
			input, err := cleanInput(request.Input)
			if err != nil {
				return err
			}
			updates, err := basicUpdates(input)
			if err != nil {
				return err
			}
			wasOffCampus := before.AccommodationType == studentConstants.AccommodationOffCampus
			isOffCampus := updates["accommodation_type"] == studentConstants.AccommodationOffCampus
			if request.OffCampusDetails != nil {
				if !wasOffCampus && !isOffCampus {
					return offCampusConflict("仅校外住宿学生可以填写校外住宿明细")
				}
				if err := saveOffCampus(
					tx, workspaceID, id, isOffCampus, *request.OffCampusDetails, event,
				); err != nil {
					return err
				}
			} else if wasOffCampus && !isOffCampus {
				var count int64
				if err := tx.Model(&studentModels.StudentOffCampusDetail{}).
					Where("workspace_id = ? AND student_id = ?", workspaceID, id).
					Count(&count).Error; err != nil {
					return err
				}
				if count != 0 {
					return offCampusConflict("请先清空校外住宿信息，再修改住宿类型")
				}
			}
			updates["revision"] = request.Revision + 1
			changed := tx.Model(&studentModels.Student{}).
				Where("workspace_id = ? AND id = ? AND revision = ?", workspaceID, id, request.Revision).
				Updates(updates)
			if changed.Error != nil {
				return writeError(changed.Error)
			}
			if changed.RowsAffected != 1 {
				return stale()
			}

			if sensitive != nil {
				input.BirthDate, err = saveSensitive(tx, workspaceID, id, sensitive)
				if err != nil {
					return err
				}
			} else {
				input.BirthDate = before.BirthDate
			}
			if input.BirthDate != before.BirthDate {
				timelinePersonIDs, err = cpcpersonUtils.RefreshStudentTimelineStatuses(
					tx, workspaceID, accessConstants.StudentUpdate, id,
				)
				if err != nil {
					return err
				}
			}
			after, err := basicDetail(tx, workspaceID, id)
			if err != nil {
				return err
			}
			event.Before = infoSnapshot(studentInfo(before), before.Avatar)
			event.After = infoSnapshot(studentInfo(after), after.Avatar)
			result = studentModels.MutationData{ID: id, Revision: after.Revision}
			return nil
		})
	if err == nil && len(timelinePersonIDs) > 0 {
		logger.Logger.WithContext(ctx).Infof(
			"CPC timeline statuses refreshed; workspace: %s; student: %d; people: %v",
			workspaceID, id, timelinePersonIDs,
		)
	}
	return result, err
}

// Detail 返回学生基础信息及适用的校外住宿明细，不读取学生敏感字段。
func Detail(
	ctx context.Context,
	workspaceID string,
	id uint64,
) (studentModels.DetailData, error) {
	var result studentModels.DetailData
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.StudentDetail,
		studentEvent(workspaceID, id),
		func(tx *gorm.DB, _ accessModels.Principal, event *auditModels.Event) error {
			record, err := basicDetail(tx, workspaceID, id)
			if err != nil {
				return err
			}
			result, err = detailData(tx, workspaceID, record)
			if err != nil {
				return err
			}
			event.ReturnedIDs = []string{strconv.FormatUint(id, 10)}
			event.ReturnedCount = 1
			return nil
		})
	return result, err
}

func detailData(
	tx *gorm.DB,
	workspaceID string,
	record studentModels.BasicRecord,
) (studentModels.DetailData, error) {
	// 管理详情复用固定字段转换，不读取敏感字段。
	result := studentModels.DetailData{
		Info:          studentInfo(record),
		AvatarUploads: []attachmentModels.Info{},
	}
	var uploads []studentModels.AvatarUpload
	if err := tx.Where("workspace_id = ? AND student_id = ? AND state IN ?", workspaceID, record.ID,
		[]string{"pending", "deleting"}).Order("id DESC").Find(&uploads).Error; err != nil {
		return result, err
	}
	for _, upload := range uploads {
		result.AvatarUploads = append(result.AvatarUploads, attachmentModels.Info{
			ID: upload.ID, Category: "student_avatar", Filename: upload.Filename,
			Size: upload.Size, State: upload.State, Revision: upload.Revision,
			CreatedAt: upload.CreatedAt, UpdatedAt: upload.UpdatedAt,
		})
	}
	if record.AccommodationType == studentConstants.AccommodationOffCampus {
		detail, err := loadOffCampus(tx, workspaceID, record.ID)
		if err != nil {
			return result, err
		}
		values := offCampusValues(detail)
		result.OffCampusDetails = &values
	}
	// 头像路径在导入或上传时已校验，按本空间学生记录里的完整键签名。
	if record.Avatar != "" && storage.Enabled() {
		result.AvatarURL = storage.SignedCDNURL(tx.Statement.Context, record.Avatar)
	}
	return result, nil
}

// Delete 校验业务引用后硬删除学生及依赖记录，同事务保存审计快照。
func Delete(ctx context.Context, workspaceID string, id, revision uint64) error {
	var hasOffCampus bool
	var avatarUploads []studentModels.AvatarUpload
	var notificationIDs []uint64
	err := accessUtils.Workspace(ctx, workspaceID, accessConstants.StudentDelete,
		studentEvent(workspaceID, id),
		func(tx *gorm.DB, _ accessModels.Principal, event *auditModels.Event) error {
			// 校验学生归属及并发版本。
			before, err := basicDetail(tx, workspaceID, id)
			if err != nil {
				return err
			}
			if revision != before.Revision {
				return stale()
			}

			if err := projectUtils.GuardStudentDeletion(tx, workspaceID, id); err != nil {
				return err
			}

			// 删除前读取敏感信息及住宿明细，级联清理仍保留完整的加密快照。
			values, err := loadSensitive(tx, workspaceID, id)
			if err != nil {
				return err
			}
			offCampus, err := loadOffCampus(tx, workspaceID, id)
			if err != nil {
				return err
			}
			hasOffCampus = offCampus.StudentID != 0
			if hasOffCampus {
				values["off_campus_details"] = offCampusValues(offCampus)
			}
			event.Before = infoSnapshot(studentInfo(before), before.Avatar)
			var graduation studentModels.Student
			if err := tx.Select("graduation_year", "graduation_type", "graduation_type_label", "graduation_destination").
				Where("workspace_id = ? AND id = ?", workspaceID, id).
				Take(&graduation).
				Error; err != nil {
				return err
			}
			event.Before["graduation_year"] = graduation.GraduationYear
			event.Before["graduation_type"] = graduation.GraduationType
			event.Before["graduation_type_label"] = graduation.GraduationTypeLabel
			event.Before["graduation_destination"] = graduation.GraduationDestination
			if len(values) > 0 {
				event.SensitiveBefore = values
			}
			notificationIDs, err = notificationUtils.SnapshotStudentDeletion(
				tx,
				workspaceID,
				id,
				event,
			)
			if err != nil {
				return err
			}

			avatarUploads, err = studentUtils.SnapshotAvatarDeletion(tx, workspaceID, id, event)
			if err != nil {
				return err
			}

			// 除已显式保护的申报和审批引用外，其余依赖记录随学生级联删除。
			if err := tx.Where("workspace_id = ? AND id = ?", workspaceID, id).
				Delete(&studentModels.Student{}).Error; err != nil {
				return writeError(err)
			}
			return nil
		})

	// 删除与审计提交后记录操作日志，不记录住宿明细中的敏感内容。
	if err == nil {
		studentUtils.DeleteAvatarObjects(ctx, avatarUploads)
		notificationUtils.LogStudentDeleted(ctx, workspaceID, notificationIDs)
		logger.Logger.WithContext(ctx).Infof(
			"Student deleted; action: %s; workspace: %s; student: %d; off-campus detail: %t",
			accessConstants.StudentDelete, workspaceID, id, hasOffCampus,
		)
	}
	return err
}

func create(tx *gorm.DB, workspaceID string, request studentModels.Input, avatar string) (
	studentModels.Student, error,
) {
	input, err := cleanInput(request)
	if err != nil {
		return studentModels.Student{}, err
	}
	record, err := recordFromInput(input)
	if err != nil {
		return studentModels.Student{}, err
	}
	if avatar != "" && !validAvatarKey(workspaceID, avatar) {
		return studentModels.Student{}, invalid("头像路径无效，请检查导入内容")
	}
	record.Avatar = avatar
	record.Revision = 1
	student := studentModels.Student{WorkspaceID: workspaceID, BasicRecord: record}

	if err := tx.Create(&student).Error; err != nil {
		return student, writeError(err)
	}
	return student, nil
}

func basicScope(tx *gorm.DB, workspaceID string) *gorm.DB {
	return tx.Table("students AS s").Where("s.workspace_id = ?", workspaceID)
}

func basicDetail(tx *gorm.DB, workspaceID string, id uint64) (studentModels.BasicRecord, error) {
	var record studentModels.BasicRecord
	err := basicScope(
		tx,
		workspaceID,
	).Select(publicColumns()).
		Where("s.id = ?", id).
		Take(&record).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return studentModels.BasicRecord{}, missing()
	}
	return record, err
}

func validText(value string, limit int, multiline bool) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > limit {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) &&
			(!multiline || (char != '\n' && char != '\r' && char != '\t')) {
			return false
		}
	}
	return true
}

func studentEvent(workspaceID string, id uint64) auditModels.Event {
	event := auditModels.Event{TenantID: &workspaceID, ObjectType: studentConstants.ObjectStudent,
		Conditions: auditModels.Snapshot{"workspace_id": workspaceID}}
	if id > 0 {
		event.ObjectID = strconv.FormatUint(id, 10)
	}
	return event
}

func basicSnapshot(student studentModels.Student) auditModels.Snapshot {
	return infoSnapshot(studentInfo(student.BasicRecord), student.Avatar)
}

func infoSnapshot(info studentModels.Info, avatar string) auditModels.Snapshot {
	result := auditModels.Snapshot{
		"id":         info.ID,
		"revision":   info.Revision,
		"birth_date": info.BirthDate,
		"created_at": info.CreatedAt,
		"updated_at": info.UpdatedAt,
	}
	for key, value := range inputValues(info.Input) {
		result[key] = value
	}
	if avatar != "" {
		digest := sha256.Sum256([]byte(avatar))
		result["avatar_hash"] = hex.EncodeToString(digest[:])
	}
	return result
}

func invalid(message string) error {
	return &response.BusinessError{Status: http.StatusBadRequest, Message: message}
}
func missing() error {
	return &response.BusinessError{Status: http.StatusNotFound, Message: "学生不存在或不可访问"}
}
func stale() error {
	return &response.BusinessError{Status: http.StatusConflict, Message: "学生信息已被修改，请刷新后重试"}
}
func writeError(err error) error {
	var databaseError *mysql.MySQLError
	if errors.As(err, &databaseError) {
		switch databaseError.Number {
		case 1062:
			return &response.BusinessError{Status: http.StatusConflict, Message: "当前工作空间已存在该学号"}
		case 1451:
			return &response.BusinessError{
				Status:  http.StatusConflict,
				Message: "删除学生失败，请检查关联业务后重试",
			}
		}
	}
	return err
}
