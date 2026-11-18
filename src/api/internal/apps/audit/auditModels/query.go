package auditModels

import (
	"encoding/json"
	"time"

	"github.com/OVINC-CN/AgileOA/src/api/internal/pagination"
	"github.com/OVINC-CN/AgileOA/src/api/internal/response"
)

// Query 只接受十项标量条件，两个时间边界均筛选开始时刻。
type Query struct {
	pagination.Query
	UserTenants  []string
	UserID       *uint64
	WorkspaceIDs []string
	Actions      []string
	ResultCodes  []int
	AccessTypes  []uint8
	InstanceID   *string
	RequestID    string
	From         time.Time
	Until        time.Time
}

// Option 提供筛选代码及中文显示名称。
type Option struct {
	// Value 是协议字段值，数字代码以十进制文本提供给选项控件。
	Value string `json:"value" example:"0"`
	// Label 是中文显示名称。
	Label string `json:"label" example:"成功"`
}

// Summary 是审计列表的安全事件字段，不读取实例 JSON 或密文。
type Summary struct {
	// ID 是数据库分配的排序编号，按字符串传递以保留精度。
	ID uint64 `json:"id,string" swaggertype:"string" example:"123"`
	// EventID 是保留的独立随机事件编号。
	EventID string `json:"event_id" example:"12f8f7ba823d4a8eac12b5db99db71b4"`
	// EventDate 是开始时刻所属的 UTC 分区日期。
	EventDate string `json:"event_date" example:"2026-11-16"`
	// WorkspaceID 是业务空间编号，系统操作为 system。
	WorkspaceID string `json:"workspace_id" example:"system"`
	// EventContent 是事件中文说明，历史记录可以为空。
	EventContent *string `json:"event_content" example:"编辑学生信息" extensions:"x-nullable"`
	// ActionID 是已登记的英文操作标识。
	ActionID string `json:"action_id" example:"student.update"`
	// RequestID 保存事件的 OpenTelemetry Trace ID。
	RequestID string `json:"request_id" example:"4bf92f3577b34da6a3ce929d0e0e4736"`
	// UserID 是组织账号或学生编号，按字符串保留完整精度。
	UserID uint64 `json:"user_id,string" swaggertype:"string" example:"1"`
	// UserIdentifyType 为组织 1、学生 2。
	UserIdentifyType uint8 `json:"user_identify_type" example:"1"`
	// UserIdentifyTenantID 为组织 system 或学生所属空间。
	UserIdentifyTenantID string `json:"user_identify_tenant_id" example:"system"`
	// StartTime 是请求开始时刻的 UTC Unix 毫秒值。
	StartTime int64 `json:"start_time" example:"1794787200000"`
	// EndTime 是审计生成时刻的 UTC Unix 毫秒值。
	EndTime int64 `json:"end_time" example:"1794787200012"`
	// AccessType 为 web 1、mcp 2。
	AccessType uint8 `json:"access_type" example:"1"`
	// AccessSourceIP 是按可信代理规则确定的客户端 IP。
	AccessSourceIP string `json:"access_source_ip" example:"192.0.2.1"`
	// AccessUserAgent 是已清理控制字符的客户端信息。
	AccessUserAgent string `json:"access_user_agent" example:"Mozilla/5.0"`
	// ResourceTypeID 是固定业务资源类型。
	ResourceTypeID string `json:"resource_type_id" example:"student"`
	// InstanceID 是原资源编号，保留字符串和前导零。
	InstanceID string `json:"instance_id" example:"7"`
	// InstanceName 是可选的安全名称。
	InstanceName *string `json:"instance_name" example:"示例对象" extensions:"x-nullable"`
	// ResultCode 为成功 0 或非零数字状态、业务错误码。
	ResultCode int `json:"result_code" example:"0"`
	// ResultContent 是可公开的错误说明，成功时为空。
	ResultContent string `json:"result_content" example:""`
	// SnapshotUserInfo 完整保留操作者快照，不关联当前账号。
	SnapshotUserInfo json.RawMessage `json:"snapshot_user_info" swaggertype:"object"`
}

// Extend 只保留请求方法和安全路由模板。
type Extend struct {
	// Method 是 HTTP 方法或内部操作方法。
	Method string `json:"method" example:"PUT"`
	// Path 是路由模板，不包含原始查询参数。
	Path string `json:"path" example:"/v1/workspaces/:workspace_id/students/:id"`
}

// Detail 提供前后完整普通 JSON，用于详情即时生成差异。
type Detail struct {
	Summary
	// InstanceData 是变更后内容，未涉及为 JSON null。
	InstanceData json.RawMessage `json:"instance_data" swaggertype:"object" extensions:"x-nullable"`
	// InstanceOriginData 是变更前内容，未涉及为 JSON null。
	InstanceOriginData json.RawMessage `json:"instance_origin_data" swaggertype:"object" extensions:"x-nullable"`
	// ExtendData 只保留方法与路由模板。
	ExtendData Extend `json:"extend_data"`
	// HasSensitiveSnapshot 表示另有受保护快照，不返回密文、密钥或关联上下文。
	HasSensitiveSnapshot bool `json:"has_sensitive_snapshot" example:"false"`
}

// ListData 是标准审计分页与同一时间范围的筛选目录。
type ListData struct {
	// Items 是当前页安全事件。
	Items []Summary `json:"items"`
	// Total 是全部匹配记录数。
	Total int64 `json:"total" example:"120"`
	// Page 是当前页码。
	Page int `json:"page" example:"1"`
	// PageSize 是每页条数。
	PageSize int `json:"page_size" example:"20"`
	// Actions 是已登记的操作目录。
	Actions []Option `json:"actions"`
	// Workspaces 包含系统、停用与已删除的业务空间。
	Workspaces []Option `json:"workspaces"`
	// UserTenants 是用户归属目录。
	UserTenants []Option `json:"user_tenants"`
	// ResultCodes 是时间窗口内的数字操作结果。
	ResultCodes []Option `json:"result_codes"`
	// StartTime 是实际筛选下界，UTC Unix 毫秒，包含。
	StartTime int64 `json:"start_time" example:"1794787200000"`
	// EndTime 是 start_time 的实际筛选上界，不包含。
	EndTime int64 `json:"end_time" example:"1794873600000"`
}

// Day 是按 UTC 日期聚合的成功与失败次数。
type Day struct {
	// Date 是 UTC 日期。
	Date string `json:"date" example:"2026-11-16"`
	// Success 是成功次数。
	Success int64 `json:"success" example:"30"`
	// Failure 是失败次数。
	Failure int64 `json:"failure" example:"2"`
}

// StatisticsData 由同一次按日聚合生成总计和趋势。
type StatisticsData struct {
	// Total 是匹配记录总数。
	Total int64 `json:"total" example:"120"`
	// Success 是成功次数。
	Success int64 `json:"success" example:"100"`
	// Failure 是失败次数。
	Failure int64 `json:"failure" example:"20"`
	// Denied 是结果码为 403 的次数。
	Denied int64 `json:"denied" example:"3"`
	// Days 是 UTC 日期趋势，缺少记录的日期补零。
	Days []Day `json:"days" gorm:"-"`
}

// LatestLogin 保存审计内部汇总的最近登录。
type LatestLogin struct {
	UserID uint64
	Latest time.Time
}

// LoginDay 保存用户管理原展示时区的登录趋势。
type LoginDay struct {
	Date  string
	Count int64
}

// ListResponse 使用通用响应信封。
type ListResponse struct{ response.Envelope[ListData] }

// DetailResponse 使用通用响应信封。
type DetailResponse struct{ response.Envelope[Detail] }

// StatisticsResponse 使用通用响应信封。
type StatisticsResponse struct {
	response.Envelope[StatisticsData]
}
