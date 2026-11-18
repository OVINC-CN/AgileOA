package tracing_test

import (
	"context"
	"encoding/hex"
	"flag"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"go.opentelemetry.io/otel/trace"
	collector "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
	"github.com/OVINC-CN/AgileOA/src/api/internal/tracing"
)

type recordedExport struct {
	request *collector.ExportTraceServiceRequest
	headers metadata.MD
}

type traceReceiver struct {
	collector.UnimplementedTraceServiceServer
	exports chan recordedExport
}

func init() {
	// 子进程在包初始化前读取配置，测试入口也需要识别同一参数。
	flag.String("config", "", "测试进程使用的 YAML 配置文件")
}

func (r *traceReceiver) Export(
	ctx context.Context,
	request *collector.ExportTraceServiceRequest,
) (*collector.ExportTraceServiceResponse, error) {
	// 保存真正通过 gRPC 收到的请求及认证信息，不使用模拟 exporter。
	headers, _ := metadata.FromIncomingContext(ctx)
	select {
	case r.exports <- recordedExport{request: request, headers: headers}:
		return &collector.ExportTraceServiceResponse{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestGRPCExportUsesYAML(t *testing.T) {
	for _, scenario := range []struct {
		name, serviceName, wantServiceName string
	}{
		{"yaml-fallback", "", "trace-test-service"},
		{"environment-service-name", "AsyncWorker", "AsyncWorker"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			testGRPCExport(t, scenario.serviceName, scenario.wantServiceName)
		})
	}
}

func testGRPCExport(t *testing.T, serviceName, wantServiceName string) {
	t.Helper()

	// 使用独立接收端和进程，验证生产 init 的完整配置及上报路径。
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	receiver := &traceReceiver{exports: make(chan recordedExport, 1)}
	server := grpc.NewServer()
	collector.RegisterTraceServiceServer(server, receiver)
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		if err := <-serveDone; err != nil {
			t.Error(err)
		}
	})

	// 上报参数通过临时 YAML 提供，服务名允许由环境变量单独覆盖。
	cfg := *config.Config
	cfg.App.Name = "trace-test-service"
	cfg.App.Environment = "integration-test"
	cfg.Trace.ReportEnabled = true
	cfg.Trace.Endpoint = listener.Addr().String()
	cfg.Trace.Insecure = true
	cfg.Trace.Headers = map[string]string{"authorization": "yaml-test-token"}
	content, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(filename, content, 0o600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext( // #nosec G204 -- 仅重启当前测试二进制。
		ctx,
		executable,
		"-test.run=^TestGRPCExportProcess$",
		"--config",
		filename,
	)
	command.Env = append(
		os.Environ(),
		"OTEL_SERVICE_NAME="+serviceName,
		"OTEL_TRACES_SAMPLER=always_off",
		"OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://127.0.0.1:1/v1/traces",
		"OTEL_EXPORTER_OTLP_TRACES_INSECURE=false",
		"OTEL_EXPORTER_OTLP_TRACES_HEADERS=authorization=environment,x-from-env=ignored",
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("gRPC 上报子进程失败：%v\n%s", err, output)
	}

	// 接收端必须拿到真实 Span 和 YAML 认证信息，环境变量不能覆盖配置。
	select {
	case exported := <-receiver.exports:
		authorization := exported.headers.Get("authorization")
		if len(authorization) != 1 || authorization[0] != cfg.Trace.Headers["authorization"] {
			t.Fatalf("上报认证信息未读取 YAML：%v", authorization)
		}
		if len(exported.headers.Get("x-from-env")) != 0 {
			t.Fatal("环境变量中的上报头覆盖了 YAML")
		}
		for _, resource := range exported.request.GetResourceSpans() {
			// 仅服务名优先使用环境变量，部署环境及上报参数仍来自 YAML。
			attributes := make(map[string]string)
			for _, attribute := range resource.GetResource().GetAttributes() {
				attributes[attribute.GetKey()] = attribute.GetValue().GetStringValue()
			}
			if attributes["service.name"] != wantServiceName ||
				attributes["deployment.environment.name"] != cfg.App.Environment {
				t.Fatalf("追踪服务名优先级或部署环境不正确：%v", attributes)
			}
			for _, scope := range resource.GetScopeSpans() {
				for _, span := range scope.GetSpans() {
					if span.GetName() == "test.grpc.export" {
						traceID, err := trace.TraceIDFromHex(hex.EncodeToString(span.GetTraceId()))
						if err != nil || !traceID.IsValid() {
							t.Fatal("gRPC 上报包含无效的 Trace ID")
						}
						return
					}
				}
			}
		}
		t.Fatal("gRPC 接收端未收到预期追踪")
	case <-ctx.Done():
		t.Fatal("等待 gRPC 上报超时")
	}
}

func TestGRPCExportProcess(t *testing.T) {
	// 仅由父测试携带独立配置启动，普通检查不启用外部上报。
	if !config.Config.Trace.ReportEnabled {
		t.Skip("当前进程未启用追踪上报")
	}

	// 通过生产全局入口创建 Span，再等待批量上报完成。
	ctx, span := tracing.Tracer.Start(t.Context(), "test.grpc.export")
	if !span.IsRecording() || !span.SpanContext().IsValid() {
		t.Fatal("环境变量关闭了追踪")
	}
	span.End()
	if err := tracing.Provider.ForceFlush(ctx); err != nil {
		t.Fatal(err)
	}
}
