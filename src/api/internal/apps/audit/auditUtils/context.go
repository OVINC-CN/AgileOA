package auditUtils

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/audit/auditModels"
)

type requestKey struct{}

type requestContext struct {
	request    auditModels.Request
	attempted  atomic.Bool
	actorMutex sync.RWMutex
	actor      auditModels.Actor
}

// SetActor 仅由认证成功的服务写入可信身份，不接受客户端提交的身份字段。
func SetActor(ctx context.Context, actor auditModels.Actor) {
	if state, ok := ctx.Value(requestKey{}).(*requestContext); ok {
		state.actorMutex.Lock()
		defer state.actorMutex.Unlock()
		state.actor = actor
	}
}

// Actor 返回当前请求已确认的身份，未认证请求返回空身份。
func Actor(ctx context.Context) auditModels.Actor {
	state, _ := ctx.Value(requestKey{}).(*requestContext)
	if state == nil {
		return auditModels.Actor{}
	}
	state.actorMutex.RLock()
	defer state.actorMutex.RUnlock()
	return state.actor
}

// WithRequest 在 HTTP、任务或批处理入口建立一次上下文，后续逐条审计复用请求 ID。
func WithRequest(ctx context.Context, request auditModels.Request) context.Context {
	// 未指定来源的既有调用按普通接口处理，MCP 入口须显式指定来源。
	if request.Source == "" {
		request.Source = auditConstants.SourceWeb
	}

	// 请求 ID 由服务端生成，不信任外部请求头中的值。
	value := make([]byte, auditConstants.RandomBytes)
	_, _ = rand.Read(value)
	request.ID = hex.EncodeToString(value)
	request.StartedAt = time.Now()
	request.UserAgent = strings.Map(func(value rune) rune {
		if unicode.IsControl(value) {
			return -1
		}
		return value
	}, request.UserAgent)
	characters := []rune(request.UserAgent)
	if len(characters) > auditConstants.UserAgentMaxLength {
		request.UserAgent = string(characters[:auditConstants.UserAgentMaxLength])
	}
	return context.WithValue(ctx, requestKey{}, &requestContext{request: request})
}

// WithOperation 派生同一请求中的接口操作，保留请求编号和身份并隔离审计尝试状态。
func WithOperation(ctx context.Context, method, path string) context.Context {
	request := Request(ctx)
	request.Method = method
	request.Path = path
	return context.WithValue(ctx, requestKey{}, &requestContext{
		request: request,
		actor:   Actor(ctx),
	})
}

// Request 返回共享请求上下文，非 HTTP 入口也必须显式调用 WithRequest。
func Request(ctx context.Context) auditModels.Request {
	// 未建立上下文时返回空值，由审计服务拒绝不完整记录。
	state, _ := ctx.Value(requestKey{}).(*requestContext)
	if state == nil {
		return auditModels.Request{}
	}
	return state.request
}

// Attempt 标记已进入审计服务，防止 HTTP 错误兜底重复记录或递归审计。
func Attempt(ctx context.Context) {
	if state, ok := ctx.Value(requestKey{}).(*requestContext); ok {
		state.attempted.Store(true)
	}
}

// Attempted 返回当前请求是否已由业务审计处理。
func Attempted(ctx context.Context) bool {
	state, _ := ctx.Value(requestKey{}).(*requestContext)
	return state != nil && state.attempted.Load()
}
