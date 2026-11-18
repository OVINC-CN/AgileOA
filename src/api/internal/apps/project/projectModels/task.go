package projectModels

import (
	"time"

	"go.opentelemetry.io/otel/propagation"
)

// AutoSubmitPayload 保存调度时的截止时间，不携带用户身份或凭据。
type AutoSubmitPayload struct {
	WorkspaceID  string                 `json:"workspace_id"`
	ProjectID    uint64                 `json:"project_id"`
	Deadline     time.Time              `json:"deadline"`
	TraceContext propagation.MapCarrier `json:"trace_context"`
}

// RankingPayload 仅携带重排对象、通知幂等来源和追踪，不携带用户审计身份。
type RankingPayload struct {
	WorkspaceID  string                 `json:"workspace_id"`
	ProjectID    uint64                 `json:"project_id"`
	SourceID     string                 `json:"source_id"`
	TraceContext propagation.MapCarrier `json:"trace_context"`
}
