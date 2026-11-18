package auditUtils

import (
	"testing"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
)

func TestOperationPreservesRequestIdentityAndIsolatesAttemptState(t *testing.T) {
	// 同一 MCP 请求中的工具共用编号，分别记录原 API 的操作范围。
	parent := WithRequest(t.Context(), auditModels.Request{
		Method: "POST", Path: "/v1/mcp", ClientIP: "192.0.2.1",
		Source: auditConstants.SourceMCP,
	})
	SetActor(parent, auditModels.Actor{ID: 7, Username: "fixture-user"})
	Attempt(parent)
	first := WithOperation(parent, "GET", "/v1/workspaces/:workspace_id/students")
	second := WithOperation(parent, "GET", "/v1/workspaces/:workspace_id/projects")
	if Request(first).ID != Request(parent).ID || Request(second).ID != Request(parent).ID ||
		Request(first).Source != auditConstants.SourceMCP ||
		Request(second).Source != auditConstants.SourceMCP ||
		Actor(first).ID != 7 || Request(first).Method != "GET" ||
		Request(first).Path == Request(second).Path || Request(parent).Path != "/v1/mcp" {
		t.Fatal("派生操作丢失请求身份或修改了其他操作范围")
	}
	if Attempted(first) || Attempted(second) {
		t.Fatal("派生操作继承了其他操作的审计尝试状态")
	}
	Attempt(first)
	if !Attempted(first) || Attempted(second) || !Attempted(parent) {
		t.Fatal("工具之间的审计尝试状态未隔离")
	}
}
