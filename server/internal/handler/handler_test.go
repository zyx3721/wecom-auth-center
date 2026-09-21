package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jerion/wecom-auth-center/server/internal/config"
	"github.com/jerion/wecom-auth-center/server/internal/metrics"
	"github.com/jerion/wecom-auth-center/server/internal/service"
	"github.com/jerion/wecom-auth-center/server/internal/store"
)

// newTestHandler 构造 mock 模式的完整 Handler。
func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	cfg := newTestConfig()
	cfg.Wecom.Mock = true
	return New(cfg, newSSO(cfg), service.MockClient{}, nil, nil, metrics.New(time.Now), store.NewMemory(time.Now))
}

func newTestConfig() *config.Config {
	var cfg config.Config
	cfg.Server.ExternalURL = "https://auth.example.com"
	cfg.Server.StaticDir = "web/static"
	cfg.Wecom.Mode = config.ModeQrcode
	cfg.Wecom.CorpID = "wwTestCorpID"
	cfg.Wecom.AgentID = 1000002
	cfg.Wecom.Secret = "test-secret"
	cfg.TTL.State = 5 * time.Minute
	cfg.TTL.Ticket = time.Minute
	cfg.TTL.VerifyTSSkew = time.Minute
	cfg.Apps = map[string]*config.AppConfig{
		"oa": {Domain: "https://oa.example.com", CallbackPath: "/sso/login", AppSecret: strings.Repeat("a", 32)},
		"it": {Domain: "https://it.example.com", CallbackPath: "/sso/login", AppSecret: strings.Repeat("b", 32)},
	}
	return &cfg
}

func newSSO(cfg *config.Config) *service.SSO {
	return service.NewSSO(store.NewMemory(time.Now), cfg.TTL.State, cfg.TTL.Ticket)
}

// loginFor 走一遍 /login，返回 302 Location 的解析结果。
func loginFor(t *testing.T, h *Handler, target string) *url.URL {
	t.Helper()
	req := httptest.NewRequest("GET", target, nil)
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("/login 应 302，实际 %d", rec.Code)
	}
	u, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("解析 Location: %v", err)
	}
	return u
}

// ticketViaCallback 从 login 开始走完 callback，返回最终 302 的地址。
func ticketViaCallback(t *testing.T, h *Handler, loginTarget string) string {
	t.Helper()
	scan := loginFor(t, h, loginTarget) // mock 模式指向 /mock/scan
	code, state := scan.Query().Get("code"), scan.Query().Get("state")

	req := httptest.NewRequest("GET", "/callback?code="+code+"&state="+state, nil)
	rec := httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("/callback 应 302，实际 %d", rec.Code)
	}
	return rec.Header().Get("Location")
}

func postVerify(t *testing.T, h *Handler, secret, app, ticket string, ts int64, sign string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"app": app, "ticket": ticket, "ts": ts, "sign": sign})
	req := httptest.NewRequest("POST", "/api/verify", strings.NewReader(string(body)))
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	h.Verify(rec, req)

	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

// str 取 verify 响应中的字符串字段，缺省为空串。
func str(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func TestLoginRejectsUnknownApp(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest("GET", "/login?app=evil", nil)
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("未知 app 应 400，实际 %d", rec.Code)
	}
}

func TestLoginRedirectsToWeComQRLogin(t *testing.T) {
	cfg := newTestConfig() // 非 mock，验证真实 302 目标
	h := New(cfg, newSSO(cfg), service.MockClient{}, nil, nil, metrics.New(time.Now), store.NewMemory(time.Now))

	loc := loginFor(t, h, "/login?app=oa")
	if loc.Scheme != "https" || loc.Host != "login.work.weixin.qq.com" || loc.Path != "/wwlogin/sso/login" {
		t.Fatalf("302 目标不符: %s", loc)
	}
	q := loc.Query()
	if q.Get("login_type") != "CorpApp" || q.Get("appid") != cfg.Wecom.CorpID || q.Get("state") == "" {
		t.Fatalf("query 参数不符: %v", q)
	}
	if q.Get("agentid") != "1000002" {
		t.Fatalf("agentid 不符: %s", q.Get("agentid"))
	}
	if q.Get("redirect_uri") != "https://auth.example.com/callback" {
		t.Fatalf("redirect_uri 不符: %s", q.Get("redirect_uri"))
	}
}

func TestLoginSanitizesRedirect(t *testing.T) {
	h := newTestHandler(t)
	loc := loginFor(t, h, "/login?app=oa&redirect=//evil.com")
	if strings.Contains(loc.String(), "evil.com") {
		t.Fatalf("协议相对地址应被清除: %s", loc)
	}
}

func TestSanitizeLocalPath(t *testing.T) {
	cases := map[string]string{
		"/dashboard":  "/dashboard",
		"/a/b?x=1":    "/a/b?x=1",
		"":            "",
		"//evil.com":  "",
		"/\\evil.com": "",
		"http://x":    "",
		"relative":    "",
	}
	for in, want := range cases {
		if got := sanitizeLocalPath(in); got != want {
			t.Errorf("sanitizeLocalPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCallbackIssuesTicketAndRedirectsToApp(t *testing.T) {
	h := newTestHandler(t)
	loc := ticketViaCallback(t, h, "/login?app=oa&redirect=/console")
	if !strings.HasPrefix(loc, "https://oa.example.com/sso/login?ticket=") {
		t.Fatalf("应跳回 oa 的 callback_path，实际 %s", loc)
	}
	if !strings.Contains(loc, "redirect=%2Fconsole") {
		t.Fatalf("应带回相对路径 redirect: %s", loc)
	}
}

func TestCallbackRejectsBadAndReplayedState(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest("GET", "/callback?code=x&state=notexist", nil)
	rec := httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("无效 state 应 403，实际 %d", rec.Code)
	}

	// 正常登录一次，再重放同一 state
	scan := loginFor(t, h, "/login?app=oa")
	first := httptest.NewRequest("GET", "/callback?code="+scan.Query().Get("code")+"&state="+scan.Query().Get("state"), nil)
	rec1 := httptest.NewRecorder()
	h.Callback(rec1, first)
	if rec1.Code != http.StatusFound {
		t.Fatalf("首次 callback 应 302，实际 %d", rec1.Code)
	}
	replay := httptest.NewRequest("GET", "/callback?code="+scan.Query().Get("code")+"&state="+scan.Query().Get("state"), nil)
	rec2 := httptest.NewRecorder()
	h.Callback(rec2, replay)
	if rec2.Code != http.StatusForbidden {
		t.Fatalf("state 重放应 403，实际 %d", rec2.Code)
	}
}

func TestVerifySuccess(t *testing.T) {
	h := newTestHandler(t)
	loc := ticketViaCallback(t, h, "/login?app=oa")
	ticket := extractTicket(t, loc)

	secret := strings.Repeat("a", 32)
	rec, out := postVerify(t, h, secret, "oa", ticket, time.Now().Unix(), SignTicket(secret, "oa", ticket, time.Now().Unix()))
	if rec.Code != http.StatusOK {
		t.Fatalf("verify 应 200，实际 %d body=%s", rec.Code, rec.Body)
	}
	if str(out, "userid") != "mockuser" {
		t.Fatalf("应返回 mockuser，实际 %v", out)
	}
}

func TestVerifyReturnsMockProfile(t *testing.T) {
	h := newTestHandler(t)
	ticket := extractTicket(t, ticketViaCallback(t, h, "/login?app=oa"))
	secret, app, ts := strings.Repeat("a", 32), "oa", time.Now().Unix()

	rec, _ := postVerify(t, h, secret, app, ticket, ts, SignTicket(secret, app, ticket, ts))
	if rec.Code != http.StatusOK {
		t.Fatalf("verify 应 200，实际 %d body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Userid      string             `json:"userid"`
		Name        string             `json:"name"`
		JobNumber   string             `json:"job_number"`
		Departments []store.Department `json:"departments"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("解析 verify 响应: %v", err)
	}
	wantDept := []store.Department{{ID: 2, Name: "研发中心/研发部"}}
	if out.JobNumber != "10001" || len(out.Departments) != 1 || out.Departments[0] != wantDept[0] {
		t.Fatalf("档案字段不符: %+v", out)
	}
}

func TestVerifyTicketIsOneShot(t *testing.T) {
	h := newTestHandler(t)
	ticket := extractTicket(t, ticketViaCallback(t, h, "/login?app=oa"))
	secret, app, ts := strings.Repeat("a", 32), "oa", time.Now().Unix()

	rec, _ := postVerify(t, h, secret, app, ticket, ts, SignTicket(secret, app, ticket, ts))
	if rec.Code != http.StatusOK {
		t.Fatalf("首次 verify 应 200，实际 %d", rec.Code)
	}
	rec, out := postVerify(t, h, secret, app, ticket, ts, SignTicket(secret, app, ticket, ts))
	if rec.Code != http.StatusUnauthorized || out["error"] != "invalid_ticket" {
		t.Fatalf("ticket 重放应 invalid_ticket，实际 %d %v", rec.Code, out)
	}
}

func TestVerifyWrongSign(t *testing.T) {
	h := newTestHandler(t)
	ticket := extractTicket(t, ticketViaCallback(t, h, "/login?app=oa"))
	rec, out := postVerify(t, h, strings.Repeat("a", 32), "oa", ticket,
		time.Now().Unix(), SignTicket("wrong-secret-wrong-secret-wrong-xx", "oa", ticket, time.Now().Unix()))
	if rec.Code != http.StatusUnauthorized || out["error"] != "invalid_sign" {
		t.Fatalf("错误签名应 invalid_sign，实际 %d %v", rec.Code, out)
	}
}

func TestVerifyExpiredTS(t *testing.T) {
	h := newTestHandler(t)
	ticket := extractTicket(t, ticketViaCallback(t, h, "/login?app=oa"))
	secret, app := strings.Repeat("a", 32), "oa"
	oldTS := time.Now().Add(-3 * time.Minute).Unix()
	rec, out := postVerify(t, h, secret, app, ticket, oldTS, SignTicket(secret, app, ticket, oldTS))
	if rec.Code != http.StatusUnauthorized || out["error"] != "expired_ts" {
		t.Fatalf("时间偏差过大应 expired_ts，实际 %d %v", rec.Code, out)
	}
}

func TestVerifyAppMismatch(t *testing.T) {
	h := newTestHandler(t)
	ticket := extractTicket(t, ticketViaCallback(t, h, "/login?app=oa"))
	secret, app, ts := strings.Repeat("b", 32), "it", time.Now().Unix()
	rec, out := postVerify(t, h, secret, app, ticket, ts, SignTicket(secret, app, ticket, ts))
	if rec.Code != http.StatusUnauthorized || out["error"] != "invalid_ticket" {
		t.Fatalf("ticket 归属不匹配应拒绝，实际 %d %v", rec.Code, out)
	}
}

func TestVerifyUnknownApp(t *testing.T) {
	h := newTestHandler(t)
	rec, out := postVerify(t, h, "x", "nope", "t", time.Now().Unix(), "sig")
	if rec.Code != http.StatusUnauthorized || out["error"] != "invalid_app" {
		t.Fatalf("未知 app 应 invalid_app，实际 %d %v", rec.Code, out)
	}
}

func TestHealthz(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	h.Healthz(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("healthz 异常: %d %s", rec.Code, rec.Body)
	}
}

func extractTicket(t *testing.T, redirectURL string) string {
	t.Helper()
	u, err := url.Parse(redirectURL)
	if err != nil {
		t.Fatalf("解析 %s: %v", redirectURL, err)
	}
	return u.Query().Get("ticket")
}
