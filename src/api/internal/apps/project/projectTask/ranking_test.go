package projectTask

import (
	"errors"
	"testing"

	"github.com/hibiken/asynq"
)

func TestRankingRejectsIncompleteTasks(t *testing.T) {
	for _, payload := range []string{
		`{`,
		`{"workspace_id":"w7m2k9p4s6n8c3x5","project_id":3}`,
		`{"workspace_id":"invalid","project_id":3,"source_id":"project-ranking:test"}`,
		`{"workspace_id":"w7m2k9p4s6n8c3x5","project_id":0,"source_id":"project-ranking:test"}`,
		`{"workspace_id":"w7m2k9p4s6n8c3x5","project_id":3,"source_id":"other:test"}`,
	} {
		for _, process := range []asynq.HandlerFunc{ProcessRanking, ProcessRankingFailure} {
			if err := process(
				t.Context(),
				asynq.NewTask("ranking", []byte(payload)),
			); !errors.Is(
				err,
				asynq.SkipRetry,
			) {
				t.Fatalf("不完整任务不得进入排名、通知或重试：%v", err)
			}
		}
	}
}
