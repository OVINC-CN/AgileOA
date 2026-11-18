package db

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/OVINC-CN/AgileOA/src/api/internal/tracing"
)

func mockMySQL(t *testing.T) sqlmock.Sqlmock {
	t.Helper()

	// 使用真实 GORM 连接池接口，避免测试仍依赖旧 ORM 的关闭和检查方法。
	sqlDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	original := db
	t.Cleanup(func() {
		db = original
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		_ = sqlDB.Close()
	})
	db, err = gorm.Open(
		gormmysql.New(gormmysql.Config{
			Conn:                      sqlDB,
			SkipInitializeWithVersion: true,
		}),
		&gorm.Config{
			DisableAutomaticPing: true,
			Logger:               gormlogger.Discard,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return mock
}

func TestMySQLPingPreservesContextAndTrace(t *testing.T) {
	for _, test := range []struct {
		name     string
		pingErr  error
		canceled bool
	}{
		{name: "success"},
		{name: "failure", pingErr: errors.New("database connection unavailable")},
		{name: "canceled", pingErr: context.Canceled, canceled: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			// 关闭上报时也必须记录真实 Ping，并保留失败和取消状态。
			mock := mockMySQL(t)
			if !test.canceled {
				mock.ExpectPing().WillReturnError(test.pingErr)
			}
			recorder := tracetest.NewSpanRecorder()
			tracing.Provider.RegisterSpanProcessor(recorder)
			t.Cleanup(func() { tracing.Provider.UnregisterSpanProcessor(recorder) })
			ctx, parent := tracing.Tracer.Start(t.Context(), "test.database.ping")
			defer parent.End()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			if test.canceled {
				cancel()
			}
			if err := pingMySQL(ctx); !errors.Is(err, test.pingErr) {
				t.Fatalf("数据库检查错误不正确：%v", err)
			}

			// Ping 必须作为当前请求的客户端子跨度，而非独立追踪。
			spans := recorder.Ended()
			if len(spans) != 1 {
				t.Fatalf("Ping 跨度数量不正确：%d", len(spans))
			}
			span := spans[0]
			if span.Name() != "mysql.ping" || span.SpanKind() != trace.SpanKindClient ||
				span.Parent().SpanID() != parent.SpanContext().SpanID() ||
				span.SpanContext().TraceID() != parent.SpanContext().TraceID() {
				t.Fatal("Ping 未正确关联当前请求追踪")
			}
			if test.pingErr != nil &&
				(span.Status().Code != codes.Error || len(span.Events()) == 0) {
				t.Fatal("Ping 失败未记录错误状态和异常")
			}
		})
	}
}

func TestReadinessChecksRedisAfterMySQLFailure(t *testing.T) {
	// MySQL 失败不能阻止 Redis 检查，两个错误都应保留供内部日志使用。
	mysqlErr := errors.New("database connection unavailable")
	redisErr := errors.New("cache connection unavailable")
	mock := mockMySQL(t)
	mock.ExpectPing().WillReturnError(mysqlErr)
	original := rdb
	rdb = redis.NewClient(&redis.Options{})
	t.Cleanup(func() {
		_ = rdb.Close()
		rdb = original
	})
	checked := false
	rdb.AddHook(pingHook{process: func(ctx context.Context, cmd redis.Cmder) error {
		checked = true
		if cmd.Name() != "ping" || ctx != t.Context() {
			t.Error("Redis 检查未沿用原始请求上下文")
		}
		return redisErr
	}})
	err := Ping(t.Context())
	if !checked || !errors.Is(err, mysqlErr) || !errors.Is(err, redisErr) {
		t.Fatalf("就绪检查跳过了资源或丢失错误：%v", err)
	}
}

func TestCloseReleasesBothPools(t *testing.T) {
	// GORM 不提供 Close 方法，必须释放其底层连接池，并继续关闭 Redis。
	mock := mockMySQL(t)
	closeErr := errors.New("database shutdown failed")
	mock.ExpectClose().WillReturnError(closeErr)
	original := rdb
	rdb = redis.NewClient(&redis.Options{})
	t.Cleanup(func() {
		_ = rdb.Close()
		rdb = original
	})
	if err := Close(); !errors.Is(err, closeErr) {
		t.Fatalf("连接池关闭错误丢失：%v", err)
	}
	if err := rdb.Ping(t.Context()).Err(); !errors.Is(err, redis.ErrClosed) {
		t.Fatalf("Redis 连接池未关闭：%v", err)
	}
}

type pingHook struct {
	process redis.ProcessHook
}

func (hook pingHook) DialHook(next redis.DialHook) redis.DialHook {
	// 检查命令由测试钩子处理，不修改连接行为。
	return next
}

func (hook pingHook) ProcessHook(_ redis.ProcessHook) redis.ProcessHook {
	// 拦截检查命令，避免单元测试访问外部 Redis。
	return hook.process
}

func (hook pingHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	// 当前测试不修改流水线行为。
	return next
}
