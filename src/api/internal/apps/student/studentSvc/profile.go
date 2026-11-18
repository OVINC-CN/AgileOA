package studentSvc

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/student/studentModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/db"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/tracing"
)

// Profile 只按可信学生身份读取本人学籍信息，不接受外部学生编号或额外操作授权。
func Profile(ctx context.Context, workspaceID string) (studentModels.ProfileData, error) {
	ctx, span := tracing.Tracer.Start(ctx, "studentSvc.Profile")
	defer span.End()
	var result studentModels.ProfileData
	event := studentEvent(workspaceID, 0)
	identity, err := userUtils.Identity(ctx)
	if err != nil {
		return result, accessUtils.Reject(ctx, accessConstants.StudentServiceRead, event, err)
	}
	if identity.Student == nil || identity.Student.WorkspaceID != workspaceID {
		return result, accessUtils.Reject(ctx, accessConstants.StudentServiceRead, event,
			accessUtils.Denied())
	}
	if db.DB() == nil {
		return result, accessUtils.Reject(ctx, accessConstants.StudentServiceRead, event,
			&response.BusinessError{Status: http.StatusServiceUnavailable, Message: "学生服务暂时不可用"})
	}

	// 与管理修改共用空间锁，仅查询学籍字段及身份复核所需的邮箱。
	student := identity.Student
	event.ObjectID = strconv.FormatUint(student.StudentID, 10)
	err = db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		workspace, err := accessUtils.ReadWorkspace(tx, workspaceID)
		if err != nil {
			return err
		}
		var record studentModels.BasicRecord
		if err := basicScope(tx, workspaceID).Select(
			"s.id",
			"s.student_no",
			"s.name",
			"s.grade",
			"s.status",
			"s.education_level",
			"s.major",
			"s.class_name",
			"s.class_position",
			"s.admission_type",
			"s.arrival_month",
			"s.email",
		).Where("s.id = ?", student.StudentID).Take(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return missing()
			}
			return err
		}
		if !workspace.IsEnabled || record.Status != studentConstants.StatusEnrolled ||
			record.StudentNo != student.StudentNo || record.Email != student.Email {
			return &response.BusinessError{
				Status: http.StatusUnauthorized, Message: "学生登录已失效，请重新登录",
			}
		}

		// 仅转换学籍响应，邮箱和其他档案字段不进入响应模型。
		result = studentModels.ProfileData{
			ID:             record.ID,
			StudentNo:      record.StudentNo,
			Name:           record.Name,
			Grade:          record.Grade,
			Status:         record.Status,
			EducationLevel: record.EducationLevel,
			Major:          record.Major,
			ClassName:      record.ClassName,
			ClassPosition:  record.ClassPosition,
			AdmissionType:  record.AdmissionType,
			ArrivalMonth:   record.ArrivalMonth,
		}

		// 仅记录读取对象和数量，审计提交前不返回学籍信息。
		event.Actor = userUtils.StudentActor(*student)
		event.Action = accessConstants.StudentServiceRead
		event.Result = auditConstants.Success
		event.ReturnedIDs = []string{event.ObjectID}
		event.ReturnedCount = 1
		return auditSvc.Write(tx, event)
	})
	if err != nil {
		return studentModels.ProfileData{}, accessUtils.Reject(
			ctx, accessConstants.StudentServiceRead, event, err,
		)
	}
	return result, nil
}
