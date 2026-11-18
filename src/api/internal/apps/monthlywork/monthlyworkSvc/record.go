package monthlyworkSvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/monthlywork/monthlyworkConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/monthlywork/monthlyworkModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/notification/notificationModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/notification/notificationUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/logger"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// Save 在工作空间锁内复核权限，业务与审计同事务保存。
func Save(
	ctx context.Context,
	workspaceID, category string,
	id uint64,
	request monthlyworkModels.Request,
) (monthlyworkModels.Info, error) {
	var result monthlyworkModels.Info
	var notifications []notificationModels.Notification
	action := actionFor(category, accessConstants.MonthlyWorkCreate)
	if id != 0 {
		action = actionFor(category, accessConstants.MonthlyWorkUpdate)
	}
	err := accessUtils.Workspace(ctx, workspaceID, action, event(workspaceID, category, id),
		func(tx *gorm.DB, principal accessModels.Principal, audit *auditModels.Event) error {
			input, err := clean(request.Input)
			if err != nil {
				return err
			}

			// 检查并发版本，负责人和参与人关联用户，不参与授权推导。
			var previousOwner uint64
			var previousParticipants []uint64
			record := monthlyworkModels.Record{
				WorkspaceID: workspaceID,
				Category:    category,
				Revision:    1,
			}
			if id != 0 {
				before, err := detail(tx, workspaceID, category, id, false)
				if err != nil {
					return err
				}
				if request.Revision != before.Revision {
					return stale()
				}
				audit.Before = snapshot(before)
				previousOwner = before.OwnerID
				previousParticipants = before.ParticipantIDs
				if request.ParticipantIDs == nil {
					input.ParticipantIDs = previousParticipants
				}
				record.ID, record.Revision = id, before.Revision+1
			}
			if input.OwnerID != previousOwner {
				if err := validateOwner(tx, workspaceID, input.OwnerID); err != nil {
					return err
				}
			}
			links, err := json.Marshal(input.Links)
			if err != nil {
				return err
			}
			record.Date, record.WorkItem, record.OwnerID = input.Date, input.WorkItem, input.OwnerID
			record.Description, record.Links = input.Description, string(links)
			if id == 0 {
				if err := tx.Create(&record).Error; err != nil {
					return writeError(err)
				}
			} else if err := tx.Model(&monthlyworkModels.Record{}).
				Where("workspace_id = ? AND category = ? AND id = ?", workspaceID, category, id).
				Updates(map[string]any{
					"date": record.Date, "work_item": record.WorkItem, "owner_id": record.OwnerID,
					"description": record.Description, "links": record.Links,
					"revision": record.Revision,
				}).Error; err != nil {
				return writeError(err)
			}
			if err := syncParticipants(
				tx,
				workspaceID,
				record.ID,
				input.ParticipantIDs,
				previousParticipants,
			); err != nil {
				return err
			}

			actions, err := accessUtils.Actions(tx, principal.User.ID, workspaceID)
			if err != nil {
				return err
			}
			result, err = detail(
				tx,
				workspaceID,
				category,
				record.ID,
				slices.Contains(
					actions,
					actionFor(category, accessConstants.MonthlyWorkAttachmentRead),
				),
			)
			if err != nil {
				return err
			}
			audit.ObjectID, audit.After = strconv.FormatUint(record.ID, 10), snapshot(result)
			// 分配通知与工作及操作审计同事务，保存失败时不产生通知。
			notifications, err = notifyAssignments(
				tx, principal, record, previousOwner, input.ParticipantIDs, previousParticipants,
			)
			return err
		})
	if err == nil {
		notificationUtils.LogCreated(ctx, notifications)
	}
	return result, err
}

// Delete 硬删除工作及其参与人关联、附件记录。
func Delete(ctx context.Context, workspaceID, category string, id, revision uint64) error {
	err := accessUtils.Workspace(
		ctx,
		workspaceID,
		actionFor(category, accessConstants.MonthlyWorkDelete),
		event(workspaceID, category, id),
		func(tx *gorm.DB, _ accessModels.Principal, audit *auditModels.Event) error {
			before, err := detail(tx, workspaceID, category, id, false)
			if err != nil {
				return err
			}
			if before.Revision != revision {
				return stale()
			}
			audit.Before = snapshot(before)

			// 所属参与人关联及附件记录按外键级联清理。
			return writeError(
				tx.Where("workspace_id = ? AND category = ? AND id = ?", workspaceID, category, id).
					Delete(&monthlyworkModels.Record{}).Error,
			)
		},
	)
	if err == nil {
		logger.Logger.WithContext(ctx).Infof(
			"Monthly work deleted; action: %s; workspace: %s; category: %s; record: %d",
			actionFor(category, accessConstants.MonthlyWorkDelete), workspaceID, category, id,
		)
	}
	return err
}

func clean(input monthlyworkModels.Input) (monthlyworkModels.Input, error) {
	// 日期保持自然日格式，说明允许换行，名称与负责人不接受控制字符。
	input.Date, input.WorkItem = strings.TrimSpace(input.Date), strings.TrimSpace(input.WorkItem)
	input.Description = strings.TrimSpace(input.Description)
	date, err := time.Parse(time.DateOnly, input.Date)
	if err != nil || date.Format(time.DateOnly) != input.Date ||
		input.Date < monthlyworkConstants.MinDate || input.Date > monthlyworkConstants.MaxDate {
		return input, invalid("请填写有效日期，格式为 YYYY-MM-DD，年份为 1900 至 9999")
	}
	if input.WorkItem == "" || input.OwnerID == 0 ||
		!validText(input.WorkItem, monthlyworkConstants.NameLength, false) ||
		!validText(input.Description, monthlyworkConstants.DescriptionLength, true) {
		return input, invalid("工作项目、负责人或详细说明无效或超过长度限制")
	}
	if len(input.Links) > config.Config.MonthlyWork.MaxLinks {
		return input, invalid("外部链接数量超过限制")
	}
	if len(input.ParticipantIDs) > config.Config.MonthlyWork.MaxOptions {
		return input, invalid("参与人数量超过限制")
	}
	input.ParticipantIDs = append([]uint64{}, input.ParticipantIDs...)
	slices.Sort(input.ParticipantIDs)
	input.ParticipantIDs = slices.Compact(input.ParticipantIDs)
	if slices.Contains(input.ParticipantIDs, 0) {
		return input, invalid("参与人用户编号无效")
	}
	for index, link := range input.Links {
		link.Name, link.URL = strings.TrimSpace(link.Name), strings.TrimSpace(link.URL)
		parsed, err := url.Parse(link.URL)
		if !validText(link.Name, monthlyworkConstants.NameLength, false) ||
			!validText(link.URL, monthlyworkConstants.URLLength, false) || err != nil ||
			(parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" ||
			parsed.User != nil || strings.ContainsRune(link.URL, '\\') ||
			strings.IndexFunc(link.URL, unicode.IsSpace) >= 0 {
			return input, invalid("外部链接须为有效的 HTTP 或 HTTPS 地址，名称最多 100 字符，地址最多 2048 字符")
		}
		input.Links[index] = link
	}
	if input.Links == nil {
		input.Links = []monthlyworkModels.Link{}
	}
	return input, nil
}

func detail(
	tx *gorm.DB,
	workspaceID, category string,
	id uint64,
	includeFiles bool,
) (monthlyworkModels.Info, error) {
	var record monthlyworkModels.Record
	if err := tx.Where("workspace_id = ? AND category = ? AND id = ?", workspaceID, category, id).
		Take(&record).
		Error; err != nil {
		return monthlyworkModels.Info{}, lookupError(err)
	}
	items, err := recordInfos(tx, workspaceID, []monthlyworkModels.Record{record}, includeFiles)
	if err != nil {
		return monthlyworkModels.Info{}, err
	}
	return items[0], nil
}

func snapshot(info monthlyworkModels.Info) auditModels.Snapshot {
	// 外部地址可能带签名参数，审计仅保留名称及地址摘要，不复制地址或令牌。
	links := make([]auditModels.Snapshot, 0, len(info.Links))
	for _, link := range info.Links {
		digest := sha256.Sum256([]byte(link.URL))
		links = append(
			links,
			auditModels.Snapshot{"name": link.Name, "url_hash": hex.EncodeToString(digest[:])},
		)
	}
	participants := make([]auditModels.Snapshot, 0, len(info.Participants))
	for _, participant := range info.Participants {
		participants = append(participants, auditModels.Snapshot{
			"id": participant.ID, "name": participant.Name, "username": participant.Username,
		})
	}
	return auditModels.Snapshot{
		"id":              info.ID,
		"date":            info.Date,
		"work_item":       info.WorkItem,
		"owner_id":        info.OwnerID,
		"owner_name":      info.OwnerName,
		"owner_username":  info.OwnerUsername,
		"participant_ids": info.ParticipantIDs,
		"participants":    participants,
		"description":     info.Description,
		"attachment_ids":  info.AttachmentIDs,
		"links":           links,
		"link_count":      len(info.Links),
		"revision":        info.Revision,
	}
}

func event(workspaceID, category string, id uint64) auditModels.Event {
	result := auditModels.Event{TenantID: &workspaceID, ObjectType: "monthly_work",
		Conditions: auditModels.Snapshot{"workspace_id": workspaceID, "category": category}}
	if id != 0 {
		result.ObjectID = strconv.FormatUint(id, 10)
	}
	return result
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

func invalid(message string) error {
	return &response.BusinessError{Status: http.StatusBadRequest, Message: message}
}

func stale() error {
	return &response.BusinessError{Status: http.StatusConflict, Message: "记录已被修改，请刷新后重试"}
}

func lookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &response.BusinessError{Status: http.StatusNotFound, Message: "工作记录不存在或不可访问"}
	}
	return err
}

func writeError(err error) error {
	var database *mysql.MySQLError
	if errors.As(err, &database) {
		switch database.Number {
		case 1451:
			return &response.BusinessError{
				Status:  http.StatusConflict,
				Message: "删除工作失败，请检查关联业务后重试",
			}
		case 1452:
			return invalid("关联记录不存在或不可访问")
		}
	}
	return err
}
