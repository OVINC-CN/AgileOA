package httpd

import (
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/apps/access/accessConstants"
	"github.com/OVINC-CN/AgileWorkspace/src/api/internal/config"
)

type captchaIssueStore struct {
	mu          sync.Mutex
	history     map[string][]time.Time
	minInterval time.Duration
	window      time.Duration
	maxRequests int
	lastCleanup time.Time
}

func captchaIssueLimiter() echo.MiddlewareFunc {
	cfg := config.Config.User.Captcha
	// Echo 按已配置的可信代理规则提取客户端 IP，未启用验证码时直接返回空数据。
	return middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Skipper: func(_ echo.Context) bool { return !cfg.Enabled },
		Store: &captchaIssueStore{
			history:     make(map[string][]time.Time),
			minInterval: cfg.IssueRate.MinInterval,
			window:      cfg.IssueRate.Window,
			maxRequests: cfg.IssueRate.MaxRequests,
		},
		DenyHandler: rejectRateLimit(accessConstants.UserLogin),
	})
}

// Allow 串行检查限额并登记本次获准签发的时间。
func (s *captchaIssueStore) Allow(ip string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.allowAt(ip, time.Now()), nil
}

func (s *captchaIssueStore) allowAt(ip string, now time.Time) bool {
	// 定期移除整个窗口内没有成功签发记录的客户端。
	cutoff := now.Add(-s.window)
	if now.Sub(s.lastCleanup) >= s.window {
		for client, times := range s.history {
			if !times[len(times)-1].After(cutoff) {
				delete(s.history, client)
			}
		}
		s.lastCleanup = now
	}

	// 保留最近的签发时间，两个条件都通过才占用本次额度。
	times := s.history[ip]
	if len(times) > 0 && now.Sub(times[len(times)-1]) < s.minInterval {
		return false
	}
	if len(times) == s.maxRequests {
		if times[0].After(cutoff) {
			return false
		}
		copy(times, times[1:])
		times = times[:len(times)-1]
	}
	s.history[ip] = append(times, now)
	return true
}
