package userSvc_test

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/user/userModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/tracing"
)

func checkAuditFilterDefaults(
	t *testing.T,
	server *echo.Echo,
	cookie *http.Cookie,
	admin userModels.User,
) {
	t.Helper()
	// 复用独立账号验收库，仅以虚构对象标识隔离本次筛选记录。
	const marker = "fixture-audit-filter"
	ctx := auditUtils.WithRequest(
		t.Context(),
		auditModels.Request{Method: "test", Path: "audit-filter"},
	)
	ctx, span := tracing.Tracer.Start(ctx, "test.audit.filter")
	defer span.End()
	if err := db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, action := range []string{
			accessConstants.UserUpdate,
			accessConstants.AuditRead,
			accessConstants.AuditStatistics,
			accessConstants.AuditDetail,
			accessConstants.AuditExport,
		} {
			event := auditModels.Event{
				Actor: auditModels.Actor{ID: admin.ID, Username: admin.Username,
					DisplayName: admin.DisplayName, IsAdmin: true},
				Action:     action,
				ObjectType: "fixture",
				ObjectID:   marker,
				Result:     auditConstants.Success,
			}
			if action == accessConstants.AuditExport {
				event.Result, event.ErrorCode, event.ErrorMessage = auditConstants.Failure, "http_403", "虚构拒绝记录"
			}
			if err := auditSvc.Write(tx, event); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, scenario := range []struct {
		name         string
		action       string
		extraActions []string
		extraResults []string
		present      bool
		result       string
		total        int64
		success      int64
		failure      int64
	}{
		{name: "default", total: 2, success: 1, failure: 1},
		{name: "exclude", action: auditConstants.FilterExcludeAudit, present: true, total: 2, success: 1, failure: 1},
		{name: "all", present: true, total: 5, success: 4, failure: 1},
		{name: "list", action: accessConstants.AuditRead, present: true, total: 1, success: 1},
		{name: "statistics", action: accessConstants.AuditStatistics, present: true, total: 1, success: 1},
		{name: "detail", action: accessConstants.AuditDetail, present: true, total: 1, success: 1},
		{name: "success", result: "0", total: 1, success: 1},
		{name: "multiple-actions", action: accessConstants.AuditRead,
			extraActions: []string{accessConstants.AuditDetail}, present: true, total: 2, success: 2},
		{name: "exclude-and-detail", action: auditConstants.FilterExcludeAudit,
			extraActions: []string{accessConstants.AuditDetail}, present: true,
			total: 3, success: 2, failure: 1},
		{name: "multiple-results", result: "0",
			extraResults: []string{"403"}, total: 2, success: 1, failure: 1},
	} {
		t.Run("audit-filter-"+scenario.name, func(t *testing.T) {
			parameters := url.Values{"instance_id": []string{marker}}
			if scenario.present {
				parameters.Set("action_id", scenario.action)
			}
			if scenario.result != "" {
				parameters.Set("result_code", scenario.result)
			}
			for _, action := range scenario.extraActions {
				parameters.Add("action_id", action)
			}
			for _, result := range scenario.extraResults {
				parameters.Add("result_code", result)
			}
			list := data[auditModels.ListData](t, send(t, server, http.MethodGet,
				"/v1/system/audit/logs?"+parameters.Encode(), nil, cookie, http.StatusOK))
			stats := data[auditModels.StatisticsData](t, send(t, server, http.MethodGet,
				"/v1/system/audit/statistics?"+parameters.Encode(), nil, cookie, http.StatusOK))
			if list.Total != scenario.total || stats.Total != scenario.total ||
				stats.Success != scenario.success || stats.Failure != scenario.failure ||
				stats.Denied != scenario.failure {
				t.Fatal("日志、总数与统计未使用相同操作筛选，或排除了额外操作")
			}
			var success, failure int64
			for _, day := range stats.Days {
				success += day.Success
				failure += day.Failure
			}
			if success != stats.Success || failure != stats.Failure {
				t.Fatal("每日趋势未应用同一操作筛选")
			}
			if !scenario.present ||
				(scenario.action == auditConstants.FilterExcludeAudit && len(scenario.extraActions) == 0) {
				for _, item := range list.Items {
					if item.ActionID == accessConstants.AuditRead ||
						item.ActionID == accessConstants.AuditStatistics ||
						item.ActionID == accessConstants.AuditDetail {
						t.Fatal("不含审计仍返回了审计读取记录")
					}
				}
			}
		})
	}
}

func checkAuditSourceFilters(
	t *testing.T,
	server *echo.Echo,
	cookie *http.Cookie,
	admin userModels.User,
) {
	t.Helper()
	// 同一个发现查询分别通过 Web 与 MCP 写入虚构记录，隔离其他验收数据。
	const marker = "fixture-audit-source"
	for _, source := range []string{auditConstants.SourceWeb, auditConstants.SourceMCP} {
		ctx := auditUtils.WithRequest(t.Context(), auditModels.Request{
			Method: http.MethodGet, Path: "/v1/workspaces", Source: source,
		})
		ctx, span := tracing.Tracer.Start(ctx, "test.audit.source")
		err := db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			event := auditModels.Event{
				Actor: auditModels.Actor{ID: admin.ID, Username: admin.Username,
					DisplayName: admin.DisplayName, IsAdmin: true},
				Action:     accessConstants.AccessMyWorkspaces,
				ObjectType: "fixture",
				ObjectID:   marker,
				Result:     auditConstants.Success,
			}
			if source == auditConstants.SourceMCP {
				event.Result = auditConstants.Failure
				event.ErrorCode, event.ErrorMessage = "http_503", "虚构服务异常"
			}
			return auditSvc.Write(tx, event)
		})
		span.End()
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, scenario := range []struct {
		name    string
		sources []string
		total   int64
		success int64
		failure int64
	}{
		{name: "all", total: 2, success: 1, failure: 1},
		{name: "web", sources: []string{"1"}, total: 1, success: 1},
		{name: "mcp", sources: []string{"2"}, total: 1, failure: 1},
		{name: "multiple", sources: []string{"1", "2"},
			total: 2, success: 1, failure: 1},
	} {
		t.Run("audit-source-"+scenario.name, func(t *testing.T) {
			parameters := url.Values{
				"instance_id": []string{marker},
				"access_type": scenario.sources,
			}
			list := data[auditModels.ListData](t, send(t, server, http.MethodGet,
				"/v1/system/audit/logs?"+parameters.Encode(), nil, cookie, http.StatusOK))
			stats := data[auditModels.StatisticsData](t, send(t, server, http.MethodGet,
				"/v1/system/audit/statistics?"+parameters.Encode(), nil, cookie, http.StatusOK))
			if list.Total != scenario.total || stats.Total != scenario.total ||
				stats.Success != scenario.success || stats.Failure != scenario.failure {
				t.Fatal("审计列表与统计未使用同一来源筛选")
			}
			var success, failure int64
			for _, day := range stats.Days {
				success += day.Success
				failure += day.Failure
			}
			if success != scenario.success || failure != scenario.failure {
				t.Fatal("审计每日趋势未使用同一来源筛选")
			}
			for _, item := range list.Items {
				if item.AccessType != auditConstants.AccessWeb &&
					item.AccessType != auditConstants.AccessMCP {
					t.Fatal("审计列表未返回正确的数字来源")
				}
				if len(scenario.sources) == 1 &&
					strconv.Itoa(int(item.AccessType)) != scenario.sources[0] {
					t.Fatal("审计列表未按数字来源精确筛选")
				}
				path := "/v1/system/audit/logs/" + item.EventID + "?event_date=" + item.EventDate
				detail := data[auditModels.Detail](t,
					send(t, server, http.MethodGet, path, nil, cookie, http.StatusOK))
				if detail.AccessType != item.AccessType {
					t.Fatal("审计详情与列表来源不一致")
				}
			}
		})
	}
}
