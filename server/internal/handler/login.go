package handler

import (
	"net/http"
	"net/url"

	"github.com/jerion/wecom-auth-center/server/internal/config"
	"github.com/jerion/wecom-auth-center/server/internal/metrics"
	"github.com/jerion/wecom-auth-center/server/internal/middleware"
	"github.com/jerion/wecom-auth-center/server/internal/store"
)

// Login GET /login?app=oa&redirect=/path
// 校验白名单 -> 登记随机 state -> 302 到企业微信授权页（mock 模式进入本地模拟扫码页）。
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	appID := r.URL.Query().Get("app")
	if _, ok := h.appOf(appID); !ok {
		http.Error(w, "invalid app", http.StatusBadRequest)
		return
	}
	redirect := sanitizeLocalPath(r.URL.Query().Get("redirect"))
	remote := middleware.RealIP(h.cfg.Server.TrustProxy)(r)
	h.track("login_start")
	h.audit.Event("login_start", "app", appID, "redirect", redirect, "remote", remote)

	state, err := h.sso.NewState(r.Context(), store.StateRecord{App: appID, Redirect: redirect, Remote: remote})
	if err != nil {
		h.log.Error("生成 state 失败", slogErr(err))
		h.renderError(w, http.StatusInternalServerError)
		return
	}

	if h.cfg.Wecom.Mock {
		http.Redirect(w, r,
			h.cfg.Server.ExternalURL+"/mock/scan?code="+mockCode()+"&state="+state,
			http.StatusFound)
		return
	}
	http.Redirect(w, r, h.authorizeURL(state), http.StatusFound)
}

// authorizeURL 按配置的授权模式拼接企业微信入口。appID 目前不参与 URL，保留白名单语义。
func (h *Handler) authorizeURL(state string) string {
	q := url.Values{}
	q.Set("redirect_uri", h.cfg.Server.ExternalURL+"/callback")
	q.Set("state", state)
	switch h.cfg.Wecom.Mode {
	case config.ModeInside:
		// 企业微信内置浏览器网页授权
		q.Set("appid", h.cfg.Wecom.CorpID)
		q.Set("response_type", "code")
		q.Set("scope", "snsapi_base")
		q.Set("agentid", itoa(h.cfg.Wecom.AgentID))
		return "https://open.weixin.qq.com/connect/oauth2/authorize?" + q.Encode() + "#wechat_redirect"
	default:
		// PC 浏览器扫码登录（wwlogin 新入口）
		q.Set("login_type", "CorpApp")
		q.Set("appid", h.cfg.Wecom.CorpID)
		q.Set("agentid", itoa(h.cfg.Wecom.AgentID))
		return "https://login.work.weixin.qq.com/wwlogin/sso/login?" + q.Encode()
	}
}

// Callback GET /callback?code=xxx&state=xxx
// 企业微信唯一配置的回调地址：消费 state -> code 换身份 -> 发 ticket -> 302 回业务系统。
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		h.renderError(w, http.StatusBadRequest)
		return
	}

	// 一次性消费：无论后续成败，state 都不可再次使用
	rec, ok := h.sso.ConsumeState(r.Context(), state)
	if !ok {
		h.log.Warn("state 校验失败", "remote", middleware.RealIP(h.cfg.Server.TrustProxy)(r))
		h.track("state_reject")
		h.audit.Event("state_reject", "remote", middleware.RealIP(h.cfg.Server.TrustProxy)(r))
		h.renderError(w, http.StatusForbidden)
		return
	}

	ui, err := h.wcom.GetUserInfo(r.Context(), code)
	if err != nil {
		h.log.Error("企业微信换取身份失败", slogErr(err))
		h.renderError(w, http.StatusBadGateway)
		return
	}

	ticket, err := h.sso.NewTicket(r.Context(), store.TicketRecord{
		App:         rec.App,
		Redirect:    rec.Redirect,
		Userid:      ui.Userid,
		Name:        ui.Name,
		JobNumber:   ui.JobNumber,
		Departments: ui.Departments,
	})
	if err != nil {
		h.log.Error("生成 ticket 失败", slogErr(err))
		h.renderError(w, http.StatusInternalServerError)
		return
	}

	app, ok := h.appOf(rec.App)
	if !ok { // state 均由白名单生成，理论上不可达；防御性处理
		h.renderError(w, http.StatusInternalServerError)
		return
	}
	h.log.Info("颁发 ticket", "app", rec.App, "userid", ui.Userid)
	h.track("ticket_issue")
	h.audit.Event("ticket_issue", "app", rec.App, "userid", ui.Userid)
	h.metrics.RecordLogin(metrics.LoginRecord{
		Time:        h.now(),
		App:         rec.App,
		Userid:      ui.Userid,
		Name:        ui.Name,
		Remote:      rec.Remote,
		JobNumber:   ui.JobNumber,
		Departments: ui.Departments,
	})
	http.Redirect(w, r, buildRedirectURL(app, ticket, rec.Redirect), http.StatusFound)
}
