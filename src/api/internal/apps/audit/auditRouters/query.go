package auditRouters

import (
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditSvc"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/workspace/workspaceUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/displaytime"
	"github.com/OVINC-CN/AgileOA/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileOA/src/api/internal/queryfilter"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

var errInvalidFilter = errors.New("invalid audit filter")

// List 仅提取固定标量条件，不接受 JSON 路径或模糊匹配。
// @Summary 审计日志列表
// @Description 系统管理员查询标准事件，按数据库 ID 倒序。默认查询展示时区的最近自然日，所有筛选均为标量精确条件。
// @Tags 审计日志
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量"
// @Param user_identify_tenant_id query []string false "用户租户，组织为 system，学生为所属空间" collectionFormat(multi)
// @Param user_id query string false "用户编号，十进制无符号整数，精确匹配"
// @Param workspace_id query []string false "业务空间编号，系统为 system" collectionFormat(multi)
// @Param action_id query []string false "操作；省略默认 exclude_audit，显式空值为全部" collectionFormat(multi)
// @Param result_code query []int false "数字结果码，成功为 0" collectionFormat(multi)
// @Param access_type query []int false "来源，web 为 1、mcp 为 2" collectionFormat(multi)
// @Param instance_id query string false "对象编号精确匹配；显式空值匹配空编号"
// @Param request_id query string false "32 位 Trace ID 精确匹配"
// @Param start_time query string false "开始时刻下界，UTC Unix 毫秒，包含"
// @Param end_time query string false "开始时刻上界，UTC Unix 毫秒，不包含"
// @Success 200 {object} auditModels.ListResponse "标准审计分页"
// @Router /v1/system/audit/logs [get]
func List(c echo.Context) error {
	query, err := parse(c)
	if err != nil {
		return auditSvc.Reject(c.Request().Context(), accessConstants.AuditRead)
	}
	data, err := auditSvc.List(c.Request().Context(), query)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Statistics 复用列表的全部已提交筛选条件。
// @Summary 审计日志统计
// @Description 两个时间参数均限制事件的 start_time，按配置的展示时区聚合自然日趋势。
// @Tags 审计日志
// @Produce json
// @Param user_identify_tenant_id query []string false "用户租户，组织为 system，学生为所属空间" collectionFormat(multi)
// @Param user_id query string false "用户编号，十进制无符号整数，精确匹配"
// @Param workspace_id query []string false "业务空间编号，系统为 system" collectionFormat(multi)
// @Param action_id query []string false "操作；省略默认 exclude_audit，显式空值为全部" collectionFormat(multi)
// @Param result_code query []int false "数字结果码，成功为 0" collectionFormat(multi)
// @Param access_type query []int false "来源，web 为 1、mcp 为 2" collectionFormat(multi)
// @Param instance_id query string false "对象编号精确匹配；显式空值匹配空编号"
// @Param request_id query string false "32 位 Trace ID 精确匹配"
// @Param start_time query string false "开始时刻下界，UTC Unix 毫秒，包含"
// @Param end_time query string false "开始时刻上界，UTC Unix 毫秒，不包含"
// @Success 200 {object} auditModels.StatisticsResponse "标准审计统计"
// @Router /v1/system/audit/statistics [get]
func Statistics(c echo.Context) error {
	query, err := parse(c)
	if err != nil {
		return auditSvc.Reject(c.Request().Context(), accessConstants.AuditStatistics)
	}
	data, err := auditSvc.Statistics(c.Request().Context(), query)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

// Detail 返回前后完整普通 JSON，不预存差异。
// @Summary 审计日志详情
// @Description 系统管理员按事件编号和 UTC 日期读取普通快照，私有密文与关联上下文不返回。
// @Tags 审计日志
// @Produce json
// @Param id path string true "32 位十六进制 event_id"
// @Param event_date query string true "开始时刻所属 UTC 日期，YYYY-MM-DD"
// @Success 200 {object} auditModels.DetailResponse "标准审计详情"
// @Router /v1/system/audit/logs/{id} [get]
func Detail(c echo.Context) error {
	id, date := c.Param("id"), c.QueryParam("event_date")
	_, idErr := hex.DecodeString(id)
	day, dateErr := time.Parse(time.DateOnly, date)
	if len(id) != 32 || idErr != nil || dateErr != nil || day.Year() < 1000 {
		return auditSvc.Reject(c.Request().Context(), accessConstants.AuditDetail)
	}
	data, err := auditSvc.Detail(c.Request().Context(), strings.ToLower(id), date)
	if err != nil {
		return err
	}
	return response.Success(c, data)
}

func parse(c echo.Context) (auditModels.Query, error) {
	// 拒绝旧关键词和其他未登记参数，避免旧条件被静默忽略。
	allowed := map[string]bool{
		"page": true, "page_size": true,
		"user_identify_tenant_id": true, "user_id": true, "workspace_id": true,
		"action_id": true, "result_code": true, "access_type": true,
		"instance_id": true, "request_id": true, "start_time": true, "end_time": true,
	}
	parameters := c.QueryParams()
	for name := range parameters {
		if !allowed[name] {
			return auditModels.Query{}, errInvalidFilter
		}
	}
	for _, name := range []string{
		"user_id", "instance_id", "request_id", "start_time", "end_time",
	} {
		if len(parameters[name]) > 1 {
			return auditModels.Query{}, errInvalidFilter
		}
	}
	page, err := pagination.Parse(c)
	if err != nil {
		return auditModels.Query{}, err
	}

	// 默认范围按展示时区取自然日，查询边界统一为 UTC 时刻。
	start, end := displaytime.Window(time.Now(), config.Config.Audit.QueryDefaultDays)
	startValue, endValue := c.QueryParam("start_time"), c.QueryParam("end_time")
	if startValue != "" {
		start, err = parseTime(startValue)
		if err != nil {
			return auditModels.Query{}, err
		}
	}
	if endValue != "" {
		end, err = parseTime(endValue)
		if err != nil {
			return auditModels.Query{}, err
		}
	}
	query := auditModels.Query{
		Query:        page,
		UserTenants:  queryfilter.Values(parameters["user_identify_tenant_id"], false),
		WorkspaceIDs: queryfilter.Values(parameters["workspace_id"], false),
		Actions:      queryfilter.Values(parameters["action_id"], false),
		RequestID:    c.QueryParam("request_id"),
		From:         start, Until: end,
	}
	if !parameters.Has("action_id") {
		query.Actions = []string{auditConstants.FilterExcludeAudit}
	}
	if !start.Before(end) || ((startValue != "" || endValue != "") &&
		!displaytime.WithinDays(start, end, config.Config.Audit.QueryMaxDays)) {
		return query, errInvalidFilter
	}

	// 用户编号和数字结果不经过浮点数，文本编号保留原始内容。
	if value := c.QueryParam("user_id"); value != "" {
		id, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return query, errInvalidFilter
		}
		query.UserID = &id
	}
	if parameters.Has("instance_id") {
		value := c.QueryParam("instance_id")
		if len([]rune(value)) > 128 {
			return query, errInvalidFilter
		}
		query.InstanceID = &value
	}
	if query.RequestID != "" {
		_, err := hex.DecodeString(query.RequestID)
		if len(query.RequestID) != 32 || err != nil ||
			query.RequestID == strings.Repeat("0", 32) {
			return query, errInvalidFilter
		}
		query.RequestID = strings.ToLower(query.RequestID)
	}
	for _, value := range queryfilter.Values(parameters["result_code"], false) {
		code, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return query, errInvalidFilter
		}
		query.ResultCodes = append(query.ResultCodes, int(code))
	}
	for _, value := range queryfilter.Values(parameters["access_type"], false) {
		code, err := strconv.ParseUint(value, 10, 8)
		if err != nil || (code != auditConstants.AccessWeb && code != auditConstants.AccessMCP) {
			return query, errInvalidFilter
		}
		query.AccessTypes = append(query.AccessTypes, uint8(code))
	}
	for _, action := range query.Actions {
		if action != auditConstants.FilterExcludeAudit && !accessConstants.Registered(action) {
			return query, errInvalidFilter
		}
	}
	for _, ids := range [][]string{query.UserTenants, query.WorkspaceIDs} {
		for _, id := range ids {
			if id != auditConstants.SystemWorkspace && !workspaceUtils.ValidID(id) {
				return query, errInvalidFilter
			}
		}
	}
	return query, nil
}

func parseTime(value string) (time.Time, error) {
	milliseconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}, errInvalidFilter
	}
	result := time.UnixMilli(milliseconds).UTC()
	if result.Year() < 1000 || result.Year() > 9999 {
		return time.Time{}, errInvalidFilter
	}
	return result, nil
}
