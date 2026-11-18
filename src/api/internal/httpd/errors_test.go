package httpd_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/httpd"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/logger"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/response"
)

func TestHTTPFailuresLogDetailsWithResponseTrace(t *testing.T) {
	// 测试不并发修改全局日志钩子，并保留原有的 Trace ID 关联。
	original := make(logrus.LevelHooks, len(logger.Logger.Hooks))
	for level, hooks := range logger.Logger.Hooks {
		original[level] = append([]logrus.Hook(nil), hooks...)
	}
	level := logger.Logger.GetLevel()
	logger.Logger.SetLevel(logrus.ErrorLevel)
	t.Cleanup(func() {
		logger.Logger.ReplaceHooks(original)
		logger.Logger.SetLevel(level)
	})
	hook := test.NewLocal(logger.Logger)

	for _, scenario := range []struct {
		name      string
		failure   error
		detail    string
		hidden    string
		committed bool
	}{
		{
			name:    "internal",
			failure: errors.New("database connection failed"),
			detail:  "database connection failed",
		},
		{
			name: "client",
			failure: echo.NewHTTPError(http.StatusBadRequest).
				SetInternal(errors.New("JSON decoding failed")),
			detail: "JSON decoding failed",
		},
		{
			name: "number",
			failure: echo.NewHTTPError(http.StatusBadRequest).
				SetInternal(&strconv.NumError{
					Func: "ParseInt",
					Num:  "private request value",
					Err:  strconv.ErrSyntax,
				}),
			detail: "ParseInt failed: invalid syntax",
			hidden: "private request value",
		},
		{
			name: "business",
			failure: &response.BusinessError{
				Status:  http.StatusConflict,
				Message: "数据已被修改",
			},
			detail: "business operation rejected with HTTP status 409",
		},
		{
			name:      "committed",
			failure:   errors.New("response stream failed"),
			detail:    "response stream failed",
			committed: true,
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			hook.Reset()
			server := httpd.New()
			server.GET("/test/diagnostics", func(c echo.Context) error {
				if scenario.committed {
					if err := response.Success[*response.ErrorData](c, nil); err != nil {
						return err
					}
				}
				return scenario.failure
			})
			recorder := httptest.NewRecorder()
			server.ServeHTTP(
				recorder,
				httptest.NewRequestWithContext(
					t.Context(),
					http.MethodGet,
					"/test/diagnostics",
					nil,
				),
			)
			result := checkResponse(t, recorder)
			entries := hook.AllEntries()
			if len(entries) != 1 || entries[0].Data["trace"] != result.Trace ||
				!strings.Contains(entries[0].Message, scenario.detail) {
				t.Fatalf("异常日志丢失、重复或未关联响应：%v", entries)
			}
			if scenario.hidden != "" && strings.Contains(entries[0].Message, scenario.hidden) {
				t.Fatal("原始请求值进入了异常日志")
			}
			if !scenario.committed && strings.Contains(result.ErrorMsg, scenario.detail) {
				t.Fatal("内部异常详情进入了公开响应")
			}
		})
	}
}
