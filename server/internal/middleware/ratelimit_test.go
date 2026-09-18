package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimitBlocksOverLimit(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rl := NewRateLimiter(3, time.Minute, func() time.Time { return base })

	for i := 0; i < 3; i++ {
		if !rl.Allow("1.2.3.4") {
			t.Fatalf("第 %d 次请求不应被限流", i+1)
		}
	}
	if rl.Allow("1.2.3.4") {
		t.Fatal("超过限制后应拒绝")
	}
	if !rl.Allow("5.6.7.8") {
		t.Fatal("不同 IP 不应被连带限流")
	}
}

func TestRateLimitResetsOnWindow(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := base
	rl := NewRateLimiter(2, time.Minute, func() time.Time { return now })

	rl.Allow("1.2.3.4")
	rl.Allow("1.2.3.4")
	if rl.Allow("1.2.3.4") {
		t.Fatal("窗口内第 3 次应拒绝")
	}
	now = base.Add(time.Minute)
	if !rl.Allow("1.2.3.4") {
		t.Fatal("新窗口应重新放行")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute, time.Now)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	h := rl.Middleware(RealIP(false), next)

	req := httptest.NewRequest("POST", "/api/verify", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("首次请求应通过，实际 %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("第二次请求应 429，实际 %d", rec.Code)
	}
}
