package auditSvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
	"github.com/OVINC-CN/AgileOA/src/api/internal/tracing"
)

var summaryColumns = []string{
	"id", "event_date", "event_id", "workspace_id", "event_content", "action_id",
	"request_id", "user_id", "user_identify_type", "user_identify_tenant_id",
	"start_time", "end_time", "access_type", "access_source_ip", "access_user_agent",
	"resource_type_id", "instance_id", "instance_name", "result_code", "result_content",
	"snapshot_user_info",
}

// List 先在标量索引上计数、选页，再按复合主键读取当前页。
func List(ctx context.Context, query auditModels.Query) (auditModels.ListData, error) {
	result := auditModels.ListData{
		Items:     []auditModels.Summary{},
		Page:      query.Page,
		PageSize:  query.PageSize,
		Actions:   actionOptions(),
		StartTime: query.From.UnixMilli(),
		EndTime:   query.Until.UnixMilli(),
	}
	event := queryEvent()
	err := read(
		ctx,
		accessConstants.AuditRead,
		event,
		func(tx *gorm.DB) (auditModels.Event, error) {
			// 目录只读取同一时间窗口的标量，空间名称在去重后一次关联。
			if err := filterOptions(tx, query, &result); err != nil {
				return event, err
			}
			return event, readPage(tx, query, &result)
		},
	)
	return result, err
}

func readPage(tx *gorm.DB, query auditModels.Query, result *auditModels.ListData) error {
	// 总数与选页共享标量条件，先在数据库内截取当前页复合键。
	if err := queryScope(tx, query).Count(&result.Total).Error; err != nil {
		return err
	}
	keys := queryScope(tx, query).Select("id,event_date").
		Order("id DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize)
	columns := make([]string, 0, len(summaryColumns))
	for _, column := range summaryColumns {
		columns = append(columns, "entries."+column)
	}
	first, last := dateBounds(query)
	var records []auditModels.Record
	if err := tx.Table("(?) AS audit_page", keys).Select(strings.Join(columns, ",")).
		Joins("JOIN audit_events AS entries ON entries.id = audit_page.id AND entries.event_date = audit_page.event_date").
		Where("entries.event_date >= ? AND entries.event_date <= ?", first, last.Format(time.DateOnly)).
		Order("entries.id DESC").Scan(&records).Error; err != nil {
		return err
	}

	// 日期和大整数键不经过 Go 重编码；存在匹配记录的页不能静默返回空结果。
	if len(records) == 0 && int64(query.Page-1)*int64(query.PageSize) < result.Total {
		return fmt.Errorf("%w: nonempty audit page returned no records", ErrUnavailable)
	}
	for _, record := range records {
		item, err := summary(record)
		if err != nil {
			return err
		}
		result.Items = append(result.Items, item)
	}
	return nil
}

// Detail 仅读取普通字段和前后 JSON，密文与关联上下文不离开数据库。
func Detail(ctx context.Context, id, date string) (auditModels.Detail, error) {
	var result auditModels.Detail
	event := queryEvent()
	event.ObjectID = id
	err := read(
		ctx,
		accessConstants.AuditDetail,
		event,
		func(tx *gorm.DB) (auditModels.Event, error) {
			var record auditModels.Record
			columns := append(slices.Clone(summaryColumns),
				"instance_data", "instance_origin_data", "extend_data",
				"COALESCE(OCTET_LENGTH(sensitive_snapshot),0) > 0 AS has_sensitive_snapshot",
			)
			if err := tx.Model(&auditModels.Record{}).Select(columns).
				Where("event_id = ? AND event_date = ?", id, date).Take(&record).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return event, &response.BusinessError{
						Status: http.StatusNotFound, Message: "审计记录不存在",
					}
				}
				return event, err
			}
			item, err := summary(record)
			if err != nil {
				return event, err
			}
			result = auditModels.Detail{
				Summary: item, InstanceData: record.InstanceData,
				InstanceOriginData:   record.InstanceOriginData,
				HasSensitiveSnapshot: record.HasSensitiveSnapshot,
			}
			if err := json.Unmarshal(record.ExtendData, &result.ExtendData); err != nil {
				return event, err
			}
			return event, nil
		},
	)
	return result, err
}

func dateBounds(query auditModels.Query) (string, time.Time) {
	last := query.Until.Add(-time.Millisecond).UTC()
	day := time.Date(last.Year(), last.Month(), last.Day(), 0, 0, 0, 0, time.UTC)
	return query.From.UTC().Format(time.DateOnly), day
}

func windowScope(tx *gorm.DB, query auditModels.Query) *gorm.DB {
	first, last := dateBounds(query)
	return tx.Model(&auditModels.Record{}).
		Where("event_date >= ? AND event_date <= ?", first, last.Format(time.DateOnly)).
		Where("start_time >= ? AND start_time < ?", query.From.UnixMilli(), query.Until.UnixMilli())
}

func queryScope(tx *gorm.DB, query auditModels.Query) *gorm.DB {
	scope := windowScope(tx, query)
	if len(query.UserTenants) > 0 {
		scope = scope.Where("user_identify_tenant_id IN ?", query.UserTenants)
	}
	if query.UserID != nil {
		scope = scope.Where("user_id = ?", *query.UserID)
	}
	if len(query.WorkspaceIDs) > 0 {
		scope = scope.Where("workspace_id IN ?", query.WorkspaceIDs)
	}
	actions := make([]string, 0, len(query.Actions))
	exclude := false
	for _, action := range query.Actions {
		if action == auditConstants.FilterExcludeAudit {
			exclude = true
		} else {
			actions = append(actions, action)
		}
	}
	if exclude {
		matches := tx.Where("action_id NOT IN ?", []string{
			accessConstants.AuditRead, accessConstants.AuditStatistics, accessConstants.AuditDetail,
		})
		if len(actions) > 0 {
			matches = matches.Or("action_id IN ?", actions)
		}
		scope = scope.Where(matches)
	} else if len(actions) > 0 {
		scope = scope.Where("action_id IN ?", actions)
	}
	if len(query.ResultCodes) > 0 {
		scope = scope.Where("result_code IN ?", query.ResultCodes)
	}
	if len(query.AccessTypes) > 0 {
		scope = scope.Where("access_type IN ?", query.AccessTypes)
	}
	if query.AccessSourceIP != "" {
		scope = scope.Where("access_source_ip = ?", query.AccessSourceIP)
	}
	if query.InstanceID != nil {
		scope = scope.Where("instance_id = ?", *query.InstanceID)
	}
	if query.RequestID != "" {
		scope = scope.Where("request_id = ?", query.RequestID)
	}
	return scope
}

func filterOptions(tx *gorm.DB, query auditModels.Query, result *auditModels.ListData) error {
	var groups []struct {
		WorkspaceID          string
		UserIdentifyTenantID string
		ResultCode           int
	}
	if err := windowScope(tx, query).
		Select("workspace_id,user_identify_tenant_id,result_code").
		Group("workspace_id,user_identify_tenant_id,result_code").Scan(&groups).Error; err != nil {
		return err
	}
	spaces := map[string]bool{}
	tenants := map[string]bool{}
	codes := map[int]bool{0: true}
	for _, row := range groups {
		spaces[row.WorkspaceID] = true
		tenants[row.UserIdentifyTenantID] = true
		codes[row.ResultCode] = true
	}
	ids := make([]string, 0, len(spaces)+len(tenants))
	for _, group := range []map[string]bool{spaces, tenants} {
		for id := range group {
			if id != auditConstants.SystemWorkspace && !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
	}
	slices.Sort(ids)
	labels, err := workspaceNames(tx, ids)
	if err != nil {
		return err
	}
	options := func(values map[string]bool) []auditModels.Option {
		result := []auditModels.Option{{Value: auditConstants.SystemWorkspace, Label: "系统"}}
		for _, id := range ids {
			if !values[id] {
				continue
			}
			result = append(result, auditModels.Option{
				Value: id,
				Label: workspaceLabel(id, labels),
			})
		}
		return result
	}
	result.Workspaces = options(spaces)
	result.UserTenants = options(tenants)
	numbers := make([]int, 0, len(codes))
	for code := range codes {
		numbers = append(numbers, code)
	}
	slices.Sort(numbers)
	result.ResultCodes = make([]auditModels.Option, 0, len(numbers))
	for _, code := range numbers {
		result.ResultCodes = append(result.ResultCodes, auditModels.Option{
			Value: strconv.Itoa(code), Label: resultLabel(code),
		})
	}
	return nil
}

func workspaceNames(tx *gorm.DB, ids []string) (map[string]string, error) {
	var names []struct {
		ID   string
		Name string
	}
	if len(ids) > 0 {
		if err := tx.Table("workspaces").Select("id,name").Where("id IN ?", ids).
			Scan(&names).Error; err != nil {
			return nil, err
		}
	}
	labels := make(map[string]string, len(names))
	for _, item := range names {
		labels[item.ID] = item.Name
	}
	return labels, nil
}

func workspaceLabel(id string, labels map[string]string) string {
	if id == auditConstants.SystemWorkspace {
		return "系统"
	}
	name, found := labels[id]
	if !found {
		name = "已删除工作空间"
	}
	return name + "（" + id + "）"
}

func resultLabel(code int) string {
	switch code {
	case 0:
		return "成功"
	case http.StatusBadRequest:
		return "400 请求无效"
	case http.StatusUnauthorized:
		return "401 未登录"
	case http.StatusForbidden:
		return "403 无权限"
	case http.StatusNotFound:
		return "404 不存在"
	case http.StatusConflict:
		return "409 冲突"
	case http.StatusRequestEntityTooLarge:
		return "413 内容超限"
	case http.StatusTooManyRequests:
		return "429 请求过多"
	case http.StatusInternalServerError:
		return "500 处理失败"
	case http.StatusServiceUnavailable:
		return "503 暂不可用"
	default:
		return "错误码 " + strconv.Itoa(code)
	}
}

func queryEvent() auditModels.Event {
	return auditModels.Event{ObjectType: auditConstants.ObjectAudit}
}

func read(
	ctx context.Context,
	action string,
	event auditModels.Event,
	operation func(*gorm.DB) (auditModels.Event, error),
) error {
	ctx, span := tracing.Tracer.Start(ctx, action)
	defer span.End()
	if db.DB() == nil {
		return &response.BusinessError{
			Status:  http.StatusServiceUnavailable,
			Message: "暂时无法完成操作，请稍后重试",
		}
	}
	err := db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, err := userUtils.Require(tx, action)
		event.Actor = auditModels.Actor{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.DisplayName,
			IsAdmin:     user.IsAdmin,
		}
		if err != nil {
			return err
		}
		details, err := operation(tx)
		if err != nil {
			return err
		}
		details.Actor = event.Actor
		details.Action = action
		details.Result = auditConstants.Success
		return Write(tx, details)
	})
	if err != nil {
		return queryFailure(ctx, action, event, err)
	}
	return nil
}

func queryFailure(ctx context.Context, action string, event auditModels.Event, cause error) error {
	// 查询或事务异常在转换公开提示前保留，失败审计异常单独记录。
	response.RecordError(ctx, cause)
	failure := &response.BusinessError{
		Status:  http.StatusServiceUnavailable,
		Message: "暂时无法完成操作，请稍后重试",
	}
	var provided *response.BusinessError
	if errors.As(cause, &provided) {
		failure = provided
	}
	event.Action = action
	event.ErrorCode = "http_" + strconv.Itoa(failure.Status)
	event.ErrorMessage = failure.Message
	if err := Failure(ctx, event); err != nil {
		response.RecordError(ctx, err)
		return &response.BusinessError{
			Status:  http.StatusServiceUnavailable,
			Message: "暂时无法完成操作，请稍后重试",
		}
	}
	return failure
}

// Reject 对审计查询参数错误独立记录，不递归审计内部落库。
func Reject(ctx context.Context, action string) error {
	return queryFailure(
		ctx,
		action,
		auditModels.Event{ObjectType: auditConstants.ObjectAudit},
		&response.BusinessError{Status: http.StatusBadRequest, Message: "审计查询参数无效"},
	)
}

func summary(record auditModels.Record) (auditModels.Summary, error) {
	if !json.Valid(record.SnapshotUserInfo) {
		return auditModels.Summary{}, ErrUnavailable
	}
	return auditModels.Summary{
		ID: record.ID, EventID: record.EventID, EventDate: dateOnly(record.EventDate),
		WorkspaceID: record.WorkspaceID, EventContent: record.EventContent,
		ActionID: record.ActionID, RequestID: record.RequestID,
		UserID: record.UserID, UserIdentifyType: record.UserIdentifyType,
		UserIdentifyTenantID: record.UserIdentifyTenantID,
		StartTime:            record.StartTime, EndTime: record.EndTime,
		AccessType: record.AccessType, AccessSourceIP: record.AccessSourceIP,
		AccessUserAgent: record.AccessUserAgent,
		ResourceTypeID:  record.ResourceTypeID, InstanceID: record.InstanceID,
		InstanceName: record.InstanceName,
		ResultCode:   record.ResultCode, ResultContent: record.ResultContent,
		SnapshotUserInfo: record.SnapshotUserInfo,
	}, nil
}

func dateOnly(value string) string {
	if len(value) > len(time.DateOnly) {
		return value[:len(time.DateOnly)]
	}
	return value
}

func actionOptions() []auditModels.Option {
	catalog := accessConstants.Catalog()
	result := make([]auditModels.Option, 0, len(catalog)+1)
	result = append(
		result,
		auditModels.Option{Value: auditConstants.FilterExcludeAudit, Label: "不含审计"},
	)
	for _, action := range catalog {
		result = append(result, auditModels.Option{Value: action.Value, Label: action.Label})
	}
	return result
}
