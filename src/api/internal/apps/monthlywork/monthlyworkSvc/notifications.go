package monthlyworkSvc

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/monthlywork/monthlyworkConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/monthlywork/monthlyworkModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/notification/notificationUtils"
)

func notifyAssignments(
	tx *gorm.DB,
	principal accessModels.Principal,
	record monthlyworkModels.Record,
	previousOwner uint64,
	current, previous []uint64,
) ([]notificationModels.Notification, error) {
	// 仅通知新负责人和新增参与人，同一个人在同次分配中只接收一条。
	recipients := assignmentRecipients(
		principal.User.ID, record.OwnerID, previousOwner, current, previous,
	)
	notifications := make([]notificationModels.Notification, 0, len(recipients))
	category := "学生月度工作"
	path := "/stu/monthly-work"
	if record.Category == monthlyworkConstants.CategoryCPC {
		category = "党建月度工作"
		path = "/cpc/monthly-work"
	}
	year, date, _ := strings.Cut(record.Date, "-")
	month, _, _ := strings.Cut(date, "-")
	link := notificationUtils.WorkspaceURL(path, record.WorkspaceID, map[string]string{
		"work_id": strconv.FormatUint(record.ID, 10),
		"year":    year,
		"month":   month,
	})
	for _, userID := range recipients {
		assignment := "参与人"
		if userID == record.OwnerID {
			assignment = "负责人"
		}
		value := notificationModels.Delivery{
			UserID: userID,
			SourceID: fmt.Sprintf(
				"monthly-work:%s:%d:%d", record.WorkspaceID, record.ID, record.Revision,
			),
			Title: "工作分配通知",
			URL:   link,
			Content: fmt.Sprintf(
				"%s将你分配为工作空间“%s”中%s“%s”的%s。\n工作日期：%s",
				principal.User.DisplayName, principal.Workspace.Name, category,
				record.WorkItem, assignment, record.Date,
			),
		}
		notification, created, err := notificationUtils.Create(tx, value)
		if err != nil {
			return nil, err
		}
		if created {
			notifications = append(notifications, notification)
		}
	}
	return notifications, nil
}

func assignmentRecipients(
	operatorID, owner, previousOwner uint64,
	current, previous []uint64,
) []uint64 {
	retained := make(map[uint64]bool, len(previous))
	for _, id := range previous {
		retained[id] = true
	}
	ids := make([]uint64, 0, len(current)+1)
	if owner != previousOwner && owner != operatorID {
		ids = append(ids, owner)
	}
	for _, id := range current {
		if !retained[id] && id != operatorID {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}
