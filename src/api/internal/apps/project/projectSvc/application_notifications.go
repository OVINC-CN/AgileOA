package projectSvc

import (
	"context"
	"fmt"
	"strconv"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationUtils"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/project/projectModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/user/userConstants"
)

type applicationNotifications struct {
	users    []notificationModels.Notification
	students []notificationModels.StudentNotification
}

func notifyApplication(
	tx *gorm.DB,
	workspaceID string,
	before, after projectModels.ApplicationInfo,
) (applicationNotifications, error) {
	var result applicationNotifications
	recipients := applicationNotificationRecipients(before, after)
	if len(recipients) == 0 {
		return result, nil
	}

	// 消息只包含定位审批所需的信息，不复制申报说明、审批意见或附件。
	title := "申报待审批通知"
	content := fmt.Sprintf(
		"%s(%s)的 %d 年“%s”申报待你审批，请查看申报详情。",
		after.StudentName, after.StudentNo, after.Year, after.ProjectName,
	)
	switch after.Status {
	case projectConstants.ApplicationRejected:
		title = "申报驳回通知"
		content = fmt.Sprintf("你的 %d 年“%s”申报已被驳回，请查看审批意见并修改后重新提交。",
			after.Year, after.ProjectName)
	case projectConstants.ApplicationApproved:
		title = "申报通过通知"
		content = fmt.Sprintf("你的 %d 年“%s”申报已全部审批通过。", after.Year, after.ProjectName)
	}
	sourceID := fmt.Sprintf("project-application:%s:%d:%d", workspaceID, after.ID, after.Revision)
	for _, recipient := range recipients {
		link := applicationNotificationURL(workspaceID, after, recipient.IdentityType)
		if recipient.IdentityType == userConstants.IdentityStudent {
			notice, created, err := notificationUtils.CreateStudent(tx,
				notificationModels.StudentDelivery{
					WorkspaceID: workspaceID, StudentID: recipient.ID, SourceID: sourceID,
					Title: title, Content: content, URL: link,
				})
			if err != nil {
				return result, err
			}
			if created {
				result.students = append(result.students, notice)
			}
		} else {
			notice, created, err := notificationUtils.Create(tx, notificationModels.Delivery{
				UserID: recipient.ID, SourceID: sourceID, Title: title, Content: content, URL: link,
			})
			if err != nil {
				return result, err
			}
			if created {
				result.users = append(result.users, notice)
			}
		}
	}
	return result, nil
}

func applicationNotificationRecipients(
	before, after projectModels.ApplicationInfo,
) []projectModels.ApplicationReviewer {
	// 只有新的终态通知申报人，保存草稿、驳回后的普通修改不产生通知。
	if after.Status == projectConstants.ApplicationRejected ||
		after.Status == projectConstants.ApplicationApproved {
		if after.Status == before.Status {
			return nil
		}
		return []projectModels.ApplicationReviewer{{
			IdentityType: userConstants.IdentityStudent, ID: after.StudentID,
		}}
	}
	if after.Status != projectConstants.ApplicationPending {
		return nil
	}

	// 或签、并签均通知当前节点的全部未签署处理人，保留原待办时不重复提醒。
	previous := map[string]bool{}
	if before.Status == projectConstants.ApplicationPending &&
		before.CurrentStep == after.CurrentStep {
		for _, step := range before.Steps {
			if step.Position != before.CurrentStep {
				continue
			}
			for _, reviewer := range step.Reviewers {
				if !reviewer.Approved {
					previous[identityKey(reviewer.IdentityType, reviewer.ID)] = true
				}
			}
		}
	}
	result := []projectModels.ApplicationReviewer{}
	for _, step := range after.Steps {
		if step.Position != after.CurrentStep {
			continue
		}
		for _, reviewer := range step.Reviewers {
			key := identityKey(reviewer.IdentityType, reviewer.ID)
			if reviewer.Approved || previous[key] {
				continue
			}
			previous[key] = true
			result = append(result, reviewer)
		}
	}
	return result
}

func applicationNotificationURL(
	workspaceID string,
	info projectModels.ApplicationInfo,
	identityType string,
) string {
	// 学生与组织账号进入各自入口，链接不改变目标接口的实时权限检查。
	path, tab := "/stu/projects", "applications"
	if identityType == userConstants.IdentityStudent {
		path, tab = "/student-service/projects", "reviews"
		if info.Status != projectConstants.ApplicationPending {
			tab = "mine"
		}
	}
	return notificationUtils.WorkspaceURL(path, workspaceID, map[string]string{
		"tab": tab, "application_id": strconv.FormatUint(info.ID, 10),
	})
}

func (notifications applicationNotifications) logCreated(ctx context.Context) {
	// 由调用方确认业务和审计已提交后统一记录两类通知。
	notificationUtils.LogCreated(ctx, notifications.users)
	notificationUtils.LogStudentCreated(ctx, notifications.students)
}
