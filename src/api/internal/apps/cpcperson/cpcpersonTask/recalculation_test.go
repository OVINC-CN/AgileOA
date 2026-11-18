package cpcpersonTask

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/cpcperson/cpcpersonSvc"
)

func TestReorderNotifiesOnlyForTerminalFailures(t *testing.T) {
	for _, test := range []struct {
		name             string
		cause            error
		retried, maximum int
		want             bool
	}{
		{"临时失败仍可重试", errors.New("temporary failure"), 0, 3, false},
		{"最后一次重试前", errors.New("temporary failure"), 2, 3, false},
		{"重试耗尽", errors.New("temporary failure"), 3, 3, true},
		{"不重试的任务", errors.New("temporary failure"), 0, 0, true},
		{"未匹配支部立即通知", fmt.Errorf("%w: person_ids: [1]", cpcpersonSvc.ErrUnmatched), 0, 3, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if finalFailure(test.cause, test.retried, test.maximum) != test.want {
				t.Fatal("失败通知的时机不符合任务重试规则")
			}
		})
	}
}

func TestReorderResultNotificationsArePrivateAndIdempotent(t *testing.T) {
	value := payload{
		WorkspaceID: "w7m2k9p4s6n8c3x5", WorkspaceName: "虚构工作空间",
		RecipientID: 7, NotificationSource: "fixture-reorder",
	}
	for _, cause := range []error{
		nil,
		fmt.Errorf("%w: person_ids: [42]", cpcpersonSvc.ErrUnmatched),
		errors.New("fixture-internal-error"),
	} {
		notice := resultNotification(value, cause)
		if notice.UserID != value.RecipientID || notice.SourceID != value.NotificationSource ||
			notice.URL != "/cpc/roster?workspace="+value.WorkspaceID ||
			!strings.Contains(notice.Content, value.WorkspaceName) ||
			strings.Contains(notice.Content, "person_ids") ||
			strings.Contains(notice.Content, "fixture-internal-error") {
			t.Fatal("通知丢失接收人或幂等标识，或泄露原始错误")
		}
		if cause == nil && notice.Title != "重排人员成功" {
			t.Fatal("成功重排未生成完成通知")
		}
		if cause != nil && notice.Title != "重排人员失败" {
			t.Fatal("失败重排未生成失败通知")
		}
	}
}
