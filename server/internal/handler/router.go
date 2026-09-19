package handler

import (
	"net/http"
	"time"

	"github.com/jerion/wecom-auth-center/server/internal/middleware"
)

// Router 组装全部路由与中间件（日志、恢复、按配置对 /login 与 /api/verify 限流）。
func Router(h *Handler) http.Handler {
	mux := http.NewServeMux()

	loginLimit := middleware.NewRateLimiter(h.cfg.RateLimit.LoginPerMinute, time.Minute, h.now)
	verifyLimit := middleware.NewRateLimiter(h.cfg.RateLimit.VerifyPerMinute, time.Minute, h.now)
	extractIP := middleware.RealIP(h.cfg.Server.TrustProxy)

	mux.Handle("GET /login", loginLimit.Middleware(extractIP, http.HandlerFunc(h.Login)))
	mux.HandleFunc("GET /callback", h.Callback)
	mux.Handle("POST /api/verify", verifyLimit.Middleware(extractIP, http.HandlerFunc(h.Verify)))
	mux.HandleFunc("GET /healthz", h.Healthz)
	mux.HandleFunc("GET /status", h.Status)
	mux.HandleFunc("GET /api/status", h.StatusAPI)

	if h.cfg.Wecom.Mock {
		mux.HandleFunc("GET /mock/scan", h.MockScan)
	}

	// 企业微信域名校验文件等静态资源（web/static/WW_verify_xxxxxxxx.txt）
	fs := http.FileServer(http.Dir(h.cfg.Server.StaticDir))
	mux.Handle("GET /", fs)

	return middleware.RequestLog(middleware.Recover(mux), h.log)
}
