// Package handler 实现认证中心的 HTTP 接口。
package handler

import (
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/jerion/wecom-auth-center/server/internal/audit"
	"github.com/jerion/wecom-auth-center/server/internal/config"
	"github.com/jerion/wecom-auth-center/server/internal/metrics"
	"github.com/jerion/wecom-auth-center/server/internal/service"
	"github.com/jerion/wecom-auth-center/server/internal/store"
)

// Handler 聚合路由处理所需的依赖。
type Handler struct {
	cfg     *config.Config
	sso     *service.SSO
	wcom    service.WeCom
	now     func() time.Time
	log     *slog.Logger
	audit   *audit.Logger
	metrics *metrics.Metrics
	st      store.Store
}

func New(cfg *config.Config, sso *service.SSO, wcom service.WeCom, log *slog.Logger, auditLog *audit.Logger, met *metrics.Metrics, st store.Store) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{cfg: cfg, sso: sso, wcom: wcom, now: time.Now, log: log, audit: auditLog, metrics: met, st: st}
}

// track 监控事件计数，未接入计数器时为空操作
func (h *Handler) track(event string) {
	h.metrics.Inc(event)
}

// appOf 查白名单，返回业务系统配置。
func (h *Handler) appOf(app string) (*config.AppConfig, bool) {
	a, ok := h.cfg.Apps[app]
	return a, ok
}

// sanitizeLocalPath 只接受以 / 开头、且不是协议相对地址（//、/\）的站内路径。
func sanitizeLocalPath(p string) string {
	if p == "" || !strings.HasPrefix(p, "/") ||
		strings.HasPrefix(p, "//") || strings.HasPrefix(p, "/\\") {
		return ""
	}
	return p
}

// buildRedirectURL 拼接业务系统回调地址：domain + callback_path + ticket（及可选 redirect）。
func buildRedirectURL(app *config.AppConfig, ticket, redirect string) string {
	u := app.Domain + app.CallbackPath + "?ticket=" + url.QueryEscape(ticket)
	if redirect != "" {
		u += "&redirect=" + url.QueryEscape(redirect)
	}
	return u
}
