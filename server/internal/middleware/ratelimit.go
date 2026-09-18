package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimiter 按 IP 的固定窗口计数限流。单实例够用；多实例部署时限流应前移到网关。
type RateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	now    func() time.Time
	counts map[string]*windowCount
}

type windowCount struct {
	start int64 // 当前窗口起点（Unix 纳秒）
	n     int
}

func NewRateLimiter(limit int, window time.Duration, now func() time.Time) *RateLimiter {
	if now == nil {
		now = time.Now
	}
	return &RateLimiter{limit: limit, window: window, now: now, counts: map[string]*windowCount{}}
}

// Allow 报告该 IP 当前窗口内是否仍可访问。
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	nowNs := rl.now().UnixNano()
	start := nowNs - nowNs%int64(rl.window)

	c, ok := rl.counts[ip]
	if !ok || c.start != start {
		// 惰性清理：窗口推进时顺带丢弃其他过期条目，防止 map 无界增长
		if len(rl.counts) > 4*rl.limit {
			for k, v := range rl.counts {
				if v.start != start {
					delete(rl.counts, k)
				}
			}
		}
		rl.counts[ip] = &windowCount{start: start, n: 1}
		return true
	}
	if c.n >= rl.limit {
		return false
	}
	c.n++
	return true
}

// Middleware 按客户端 IP 限流；超限返回 429。
func (rl *RateLimiter) Middleware(extractIP func(*http.Request) string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.Allow(extractIP(r)) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RealIP 从请求中提取客户端 IP，trustProxy 时优先取反代注入的 X-Real-IP。
func RealIP(trustProxy bool) func(*http.Request) string {
	return func(r *http.Request) string {
		if trustProxy {
			if v := r.Header.Get("X-Real-IP"); v != "" {
				return v
			}
		}
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return r.RemoteAddr
		}
		return host
	}
}
