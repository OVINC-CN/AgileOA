package notificationUtils

import (
	"strings"
	"testing"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/notification/notificationModels"
)

func TestNotificationURLBoundaries(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{
		{"", true},
		{"/cpc/roster?workspace=w7m2k9p4s6n8c3x5&person_id=42", true},
		{"/stu/monthly-work?workspace=w7m2k9p4s6n8c3x5&year=2025&month=12&work_id=7", true},
		{"https://example.com/cpc/roster", false},
		{"//example.com/cpc/roster", false},
		{"javascript:alert(1)", false},
		{"/\\example.com", false},
		{"/%2fexample.com", false},
		{"/%5cexample.com", false},
		{"/cpc/%0aroster", false},
		{"/cpc/%ZZ", false},
		{" /cpc/roster", false},
		{"/" + strings.Repeat("a", notificationConstants.URLMaxLength), false},
	} {
		t.Run(test.value, func(t *testing.T) {
			if ValidURL(test.value) != test.valid {
				t.Fatal("通知链接的站内路径校验不符合预期")
			}
			delivery := notificationModels.Delivery{
				UserID: 7, SourceID: "fixture-link", Title: "通知", Content: "正文", URL: test.value,
			}
			if ValidDelivery(delivery) != test.valid {
				t.Fatal("通知投递没有执行链接校验")
			}
		})
	}
}
