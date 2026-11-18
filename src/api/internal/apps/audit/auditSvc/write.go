package auditSvc

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditConstants"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditModels"
	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/audit/auditUtils"
	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/db"
	"github.com/OVINC-CN/AgileOA/src/api/internal/tracing"
)

// ErrUnavailable 阻止业务数据返回或业务事务提交，不携带内部数据库参数。
var ErrUnavailable = errors.New("audit persistence unavailable")

// Write 保留业务调用入口，在原事务中生成并持久化标准审计事件。
func Write(tx *gorm.DB, event auditModels.Event) error {
	// 仅使用可信身份，匿名请求和当前用户信息仍不写审计。
	if event.Action == accessConstants.UserInfo {
		return nil
	}
	if tx != nil && tx.Statement != nil && event.Actor.ID == 0 && event.Actor.StudentID == 0 {
		event.Actor = auditUtils.Actor(tx.Statement.Context)
	}
	if event.Actor.ID == 0 && event.Actor.StudentID == 0 {
		return nil
	}
	if tx == nil || tx.Statement == nil {
		return ErrUnavailable
	}
	if _, transaction := tx.Statement.ConnPool.(gorm.TxCommitter); !transaction {
		return ErrUnavailable
	}
	auditUtils.Attempt(tx.Statement.Context)
	request := auditUtils.Request(tx.Statement.Context)
	traceID := tracing.TraceID(tx.Statement.Context)
	if request.ID == "" || request.StartedAt.IsZero() ||
		!accessConstants.Registered(event.Action) || event.ObjectType == "" ||
		!trace.SpanContextFromContext(tx.Statement.Context).TraceID().IsValid() {
		return ErrUnavailable
	}

	// 来源、身份和数字结果只在审计模块转换，不要求业务改写 Event。
	accessType := uint8(auditConstants.AccessWeb)
	switch request.Source {
	case auditConstants.SourceWeb:
	case auditConstants.SourceMCP:
		accessType = auditConstants.AccessMCP
	default:
		return ErrUnavailable
	}
	code, err := resultCode(event)
	if err != nil {
		return err
	}
	workspaceID := auditConstants.SystemWorkspace
	if event.TenantID != nil {
		workspaceID = *event.TenantID
		if workspaceID == "" {
			return ErrUnavailable
		}
	}
	userID := event.Actor.ID
	userType := uint8(auditConstants.IdentityUser)
	userTenant := auditConstants.SystemWorkspace
	if event.Actor.StudentID != 0 {
		if event.Actor.ID != 0 || event.Actor.WorkspaceID == "" {
			return ErrUnavailable
		}
		userID = event.Actor.StudentID
		userType = auditConstants.IdentityStudent
		userTenant = event.Actor.WorkspaceID
	}

	// 只编码实际存储的前后内容和完整身份，不再保存旧条件、返回编号或普通差异。
	before, err := encode(event.Before)
	if err != nil {
		return err
	}
	after, err := encode(event.After)
	if err != nil {
		return err
	}
	actor, err := encode(event.Actor)
	if err != nil {
		return err
	}
	extend, err := json.Marshal(auditModels.Extend{Method: request.Method, Path: request.Path})
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	start := request.StartedAt.UTC()
	if start.After(now) {
		return ErrUnavailable
	}
	eventDate := start.Format(time.DateOnly)

	// 密文保存加密时的原六元组，历史 SQL 回填也按相同编码恢复 AAD。
	sensitive, keyID, err := encryptSensitive(event, traceID, eventDate)
	if err != nil {
		return err
	}
	var sensitiveContext []byte
	if len(sensitive) > 0 {
		sensitiveContext, err = snapshotContext(event, traceID, eventDate)
		if err != nil {
			return err
		}
	}
	if len(before)+len(after)+len(actor)+len(extend)+len(sensitive)+len(sensitiveContext) >
		config.Config.Audit.MaxPayloadBytes {
		return ErrUnavailable
	}

	// 时间与编号由审计模块生成；插入和原业务共享事务。
	value := make([]byte, auditConstants.RandomBytes)
	if _, err := rand.Read(value); err != nil {
		return fmt.Errorf("%w: event identifier generation failed: %w", ErrUnavailable, err)
	}
	content := accessConstants.Label(event.Action)
	message := ""
	if code != 0 {
		message = event.ErrorMessage
	}
	record := auditModels.Record{
		EventID: hex.EncodeToString(value), EventDate: eventDate,
		WorkspaceID: workspaceID, EventContent: &content, ActionID: event.Action,
		RequestID: traceID, UserID: userID, UserIdentifyType: userType,
		UserIdentifyTenantID: userTenant,
		StartTime:            start.UnixMilli(), EndTime: now.UnixMilli(),
		AccessType: accessType, AccessSourceIP: request.ClientIP,
		AccessUserAgent: request.UserAgent,
		ResourceTypeID:  event.ObjectType, InstanceID: event.ObjectID,
		InstanceData: after, InstanceOriginData: before,
		ResultCode: code, ResultContent: message, SnapshotUserInfo: actor, ExtendData: extend,
		SensitiveSnapshot: sensitive, SensitiveKeyID: keyID, SensitiveContext: sensitiveContext,
	}
	if err := tx.Create(&record).Error; err != nil {
		return fmt.Errorf("%w: record persistence failed: %w", ErrUnavailable, err)
	}
	return nil
}

func resultCode(event auditModels.Event) (int, error) {
	if event.Result == auditConstants.Success {
		return 0, nil
	}
	if event.Result != auditConstants.Failure {
		return 0, ErrUnavailable
	}
	value := strings.TrimPrefix(event.ErrorCode, "http_")
	code, err := strconv.ParseInt(value, 10, 32)
	if err != nil || code == 0 {
		return 0, fmt.Errorf("%w: numeric result code required", ErrUnavailable)
	}
	return int(code), nil
}

// Failure 在失败业务事务回滚后独立落库，请求取消也必须使用有界超时尝试审计。
func Failure(ctx context.Context, event auditModels.Event) error {
	// 统一排除鉴权拒绝与 HTTP 兜底失败，避免当前用户信息依赖审计存储。
	if event.Action == accessConstants.UserInfo {
		return nil
	}
	if event.Actor.ID == 0 && event.Actor.StudentID == 0 {
		event.Actor = auditUtils.Actor(ctx)
	}
	if event.Actor.ID == 0 && event.Actor.StudentID == 0 {
		return nil
	}
	// 失败审计不携带业务前后响应，也不重新执行失败业务。
	if db.DB() == nil {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), config.Config.Audit.WriteTimeout)
	defer cancel()
	event.Result = auditConstants.Failure
	return db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return Write(tx, event)
	})
}

func encode(value any) ([]byte, error) {
	// 序列化后递归检查字段名，不允许在普通或敏感快照中保存凭据。
	content, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%w: snapshot encoding failed: %w", ErrUnavailable, err)
	}
	var decoded any
	if err := json.Unmarshal(content, &decoded); err != nil {
		return nil, fmt.Errorf("%w: snapshot decoding failed: %w", ErrUnavailable, err)
	}
	if containsCredential(decoded) {
		return nil, fmt.Errorf("%w: snapshot contains a credential field", ErrUnavailable)
	}
	return content, nil
}

func containsCredential(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if credentialField(key, item) || containsCredential(item) {
				return true
			}
		}
	case []any:
		for _, item := range typed {
			if containsCredential(item) {
				return true
			}
		}
	case string:
		// CDN URL 可以记录对象标识，包含签名或认证参数的完整 URL 禁止进入审计。
		if parsed, err := url.Parse(typed); err == nil {
			for key := range parsed.Query() {
				if credentialName(key) {
					return true
				}
			}
		}
	}
	return false
}

func credentialField(name string, value any) bool {
	// 账号密码策略仅保存布尔状态，不能因字段名包含 password 而当作凭据。
	if name != "password_login_enabled" && name != "password_change_required" {
		return credentialName(name)
	}
	if _, ok := value.(bool); ok {
		return false
	}

	// 差异快照只允许完整的前后值，创建和删除时缺失的一侧使用 null。
	change, ok := value.(map[string]any)
	if !ok || len(change) != 2 {
		return true
	}
	for _, field := range []string{"before", "after"} {
		item, exists := change[field]
		if !exists {
			return true
		}
		if item != nil {
			if _, ok := item.(bool); !ok {
				return true
			}
		}
	}
	return false
}

func credentialName(name string) bool {
	name = strings.ToLower(name)
	for _, forbidden := range []string{"password", "token", "credential", "secret", "cookie", "authorization", "signature"} {
		// secretary 表示书记，不能将其中的 secret 子串误判为密钥字段。
		candidate := name
		if forbidden == "secret" {
			candidate = strings.ReplaceAll(candidate, "secretary", "")
		}
		if strings.Contains(candidate, forbidden) {
			return true
		}
	}
	return name == "key" || name == "sig" || name == "session" || name == "apikey" ||
		strings.HasSuffix(name, "_key")
}

func diff(before, after auditModels.Snapshot) map[string]auditModels.Change {
	// 仅比较固定字段快照，读取操作没有前后快照时差异为空。
	changes := map[string]auditModels.Change{}
	for key, value := range before {
		if !reflect.DeepEqual(value, after[key]) {
			changes[key] = auditModels.Change{Before: value, After: after[key]}
		}
	}
	for key, value := range after {
		if _, exists := before[key]; !exists {
			changes[key] = auditModels.Change{Before: nil, After: value}
		}
	}
	return changes
}

func encryptSensitive(
	event auditModels.Event,
	requestID, eventDate string,
) ([]byte, string, error) {
	// 只在存在敏感快照时要求独立密钥，没有密钥必须阻断业务。
	if event.SensitiveBefore == nil && event.SensitiveAfter == nil {
		return nil, "", nil
	}
	keyID := config.Config.Audit.SnapshotKeyID
	key, err := base64.StdEncoding.DecodeString(config.Config.Audit.SnapshotKeys[keyID])
	if err != nil {
		return nil, "", fmt.Errorf("%w: snapshot key decoding failed: %w", ErrUnavailable, err)
	}
	if len(key) != 32 {
		return nil, "", fmt.Errorf("%w: snapshot key must contain 32 bytes", ErrUnavailable)
	}
	content, err := encode(map[string]any{
		"before": event.SensitiveBefore,
		"after":  event.SensitiveAfter,
		"diff":   diff(event.SensitiveBefore, event.SensitiveAfter),
	})
	if err != nil {
		return nil, "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, "", fmt.Errorf(
			"%w: snapshot cipher initialization failed: %w",
			ErrUnavailable,
			err,
		)
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, "", fmt.Errorf(
			"%w: snapshot AEAD initialization failed: %w",
			ErrUnavailable,
			err,
		)
	}
	// 租户、请求、对象和分区日期作为关联数据，防止密文被替换到其他租户或记录。
	context, err := snapshotContext(event, requestID, eventDate)
	if err != nil {
		return nil, "", fmt.Errorf(
			"%w: snapshot associated data encoding failed: %w",
			ErrUnavailable,
			err,
		)
	}
	aad, err := SensitiveAssociatedData(context)
	if err != nil {
		return nil, "", err
	}
	// #nosec G407 -- NewGCMWithRandomNonce 内部生成随机 nonce 并前置到密文，API 要求传入 nil。
	return aead.Seal(nil, nil, content, aad), keyID, nil
}

func snapshotContext(event auditModels.Event, requestID, eventDate string) ([]byte, error) {
	return json.Marshal(
		[]any{event.TenantID, requestID, event.Action, event.ObjectType, event.ObjectID, eventDate},
	)
}

// SensitiveAssociatedData 按原 Go 编码恢复新旧密文的关联数据，不使用数据库 JSON 的原始文本。
func SensitiveAssociatedData(context []byte) ([]byte, error) {
	var values []*string
	if err := json.Unmarshal(context, &values); err != nil || len(values) != 6 {
		return nil, fmt.Errorf("%w: invalid sensitive context", ErrUnavailable)
	}
	for _, value := range values[1:] {
		if value == nil {
			return nil, fmt.Errorf("%w: incomplete sensitive context", ErrUnavailable)
		}
	}
	return json.Marshal(values)
}
