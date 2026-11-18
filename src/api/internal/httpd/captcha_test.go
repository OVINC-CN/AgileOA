package httpd

import (
	"testing"
	"time"
)

func TestCaptchaIssueStoreSlidingWindow(t *testing.T) {
	t.Parallel()

	store := captchaIssueStore{
		history:     make(map[string][]time.Time),
		minInterval: time.Second,
		window:      time.Minute,
		maxRequests: 5,
	}
	const firstIP, secondIP = "192.0.2.1", "192.0.2.2"
	start := time.Unix(0, 0)
	for _, test := range []struct {
		ip      string
		elapsed time.Duration
		want    bool
	}{
		{firstIP, 0, true},
		{firstIP, 999 * time.Millisecond, false},
		{firstIP, time.Second, true},
		{firstIP, 2 * time.Second, true},
		{firstIP, 3 * time.Second, true},
		{firstIP, 4 * time.Second, true},
		{firstIP, 5 * time.Second, false},
		{secondIP, 5 * time.Second, true},
		{firstIP, time.Minute - time.Nanosecond, false},
		{firstIP, time.Minute, true},
		{firstIP, time.Minute + time.Second, true},
		{firstIP, 2 * time.Minute, true},
	} {
		if got := store.allowAt(test.ip, start.Add(test.elapsed)); got != test.want {
			t.Errorf("客户端 %s 在 %s 的签发结果为 %t，期望 %t", test.ip, test.elapsed, got, test.want)
		}
	}
	if _, ok := store.history[secondIP]; ok {
		t.Error("未清理过期的签发记录")
	}
}
