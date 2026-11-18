package notificationSvc

import (
	"net/http"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

func inboxListQuery(query notificationModels.ListQuery) (
	notificationModels.ListQuery, notificationModels.ListData, auditModels.Event,
) {
	// 两类收件箱共用默认筛选、分页及对外字段，不暴露接收人或来源。
	if query.Status == "" {
		query.Status = notificationConstants.StatusUnread
	}
	return query, notificationModels.ListData{
			Items: []notificationModels.Info{}, Page: query.Page, PageSize: query.PageSize,
		}, auditModels.Event{
			ObjectType: auditConstants.ObjectNotification,
			Conditions: auditModels.Snapshot{
				"page": query.Page, "page_size": query.PageSize, "status": query.Status,
			},
		}
}

func listInbox(
	scope *gorm.DB,
	query notificationModels.ListQuery,
	result *notificationModels.ListData,
	event *auditModels.Event,
) error {
	// 调用方必须先固定表及可信接收人范围，每条查询独立克隆以免筛选互相污染。
	if query.Status != notificationConstants.StatusUnread &&
		query.Status != notificationConstants.StatusAll {
		return &response.BusinessError{
			Status: http.StatusBadRequest, Message: "通知筛选条件无效",
		}
	}
	if err := unreadCount(scope, &result.UnreadCount); err != nil {
		return err
	}
	filtered := scope.Session(&gorm.Session{})
	if query.Status == notificationConstants.StatusUnread {
		filtered = filtered.Where("read_at IS NULL")
	}
	if err := filtered.Session(&gorm.Session{}).Count(&result.Total).Error; err != nil {
		return err
	}
	if err := filtered.Session(&gorm.Session{}).
		Select("id", "title", "content", "url", "read_at", "created_at").
		Order("read_at IS NULL DESC").Order("id DESC").
		Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).
		Find(&result.Items).Error; err != nil {
		return err
	}
	for _, row := range result.Items {
		event.ReturnedIDs = append(event.ReturnedIDs, strconv.FormatUint(row.ID, 10))
	}
	event.ReturnedCount = int64(len(result.Items))
	return nil
}

func markInboxRead(
	scope *gorm.DB,
	request notificationModels.ReadRequest,
	result *notificationModels.ReadData,
	event *auditModels.Event,
) error {
	// 整批校验通过后才更新，其他接收人的编号与不存在的编号使用相同提示。
	if len(request.IDs) == 0 || len(request.IDs) > config.Config.HTTP.MaxPageSize {
		return invalidIDs()
	}
	seen := make(map[uint64]bool, len(request.IDs))
	for _, id := range request.IDs {
		if id == 0 || seen[id] {
			return invalidIDs()
		}
		seen[id] = true
	}
	var rows []notificationModels.Info
	if err := scope.Session(&gorm.Session{}).Select("id", "read_at").
		Where("id IN ?", request.IDs).Order("id ASC").Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) != len(request.IDs) {
		return &response.BusinessError{
			Status: http.StatusNotFound, Message: "通知不存在或不可访问",
		}
	}
	unreadIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		if row.ReadAt == nil {
			unreadIDs = append(unreadIDs, row.ID)
		}
	}
	if len(unreadIDs) > 0 {
		changed := scope.Session(&gorm.Session{}).
			Where("id IN ? AND read_at IS NULL", unreadIDs).Update("read_at", time.Now().UTC())
		if changed.Error != nil {
			return changed.Error
		}
		result.Changed = changed.RowsAffected
	}
	event.Before = auditModels.Snapshot{"unread_ids": unreadIDs}
	event.After = auditModels.Snapshot{"read_ids": unreadIDs}
	return unreadCount(scope, &result.UnreadCount)
}

func unreadCount(scope *gorm.DB, count *int64) error {
	return scope.Session(&gorm.Session{}).Where("read_at IS NULL").Count(count).Error
}

func invalidIDs() error {
	return &response.BusinessError{Status: http.StatusBadRequest, Message: "通知编号无效"}
}
