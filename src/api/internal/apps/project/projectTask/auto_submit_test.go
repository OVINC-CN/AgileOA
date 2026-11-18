package projectTask

import (
	"errors"
	"testing"

	"github.com/hibiken/asynq"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/project/projectConstants"
)

func TestAutoSubmitRejectsIncompleteTasks(t *testing.T) {
	for _, payload := range []string{
		`{`,
		`{"workspace_id":"w7m2k9p4s6n8c3x5","project_id":3}`,
		`{"workspace_id":"invalid","project_id":3,"deadline":"2026-12-31T15:59:00Z"}`,
		`{"workspace_id":"w7m2k9p4s6n8c3x5","project_id":0,"deadline":"2026-12-31T15:59:00Z"}`,
	} {
		err := ProcessAutoSubmit(
			t.Context(),
			asynq.NewTask(projectConstants.AutoSubmitDrafts, []byte(payload)),
		)
		if !errors.Is(err, asynq.SkipRetry) {
			t.Fatalf("不完整任务不得进入业务查询或重试：%v", err)
		}
	}
}
