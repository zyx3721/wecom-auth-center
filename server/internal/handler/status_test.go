package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jerion/wecom-auth-center/server/internal/metrics"
	"github.com/jerion/wecom-auth-center/server/internal/service"
	"github.com/jerion/wecom-auth-center/server/internal/store"
)

func newStatusHandler(enabled bool) *Handler {
	cfg := newTestConfig()
	cfg.Wecom.Mock = true
	cfg.Status.Enabled = enabled
	cfg.Status.Token = "test-status-token-0123456789abcdef"
	return New(cfg, newSSO(cfg), service.MockClient{}, nil, nil, metrics.New(time.Now), store.NewMemory(time.Now))
}

func getStatusAPI(t *testing.T, h *Handler, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/status?token="+token, nil)
	rec := httptest.NewRecorder()
	h.StatusAPI(rec, req)
	return rec
}

func TestStatusAPITokenGuard(t *testing.T) {
	h := newStatusHandler(true)

	if rec := getStatusAPI(t, h, ""); rec.Code != http.StatusForbidden {
		t.Errorf("缺 token 应 403，实际 %d", rec.Code)
	}
	if rec := getStatusAPI(t, h, "wrong-token"); rec.Code != http.StatusForbidden {
		t.Errorf("错 token 应 403，实际 %d", rec.Code)
	}
	if rec := getStatusAPI(t, h, "test-status-token-0123456789abcdef"); rec.Code != http.StatusOK {
		t.Errorf("正确 token 应 200，实际 %d", rec.Code)
	}
}

func TestStatusAPICounters(t *testing.T) {
	h := newStatusHandler(true)

	loginReq := httptest.NewRequest("GET", "/login?app=oa", nil)
	h.Login(httptest.NewRecorder(), loginReq)

	rec := getStatusAPI(t, h, "test-status-token-0123456789abcdef")
	var payload struct {
		Server map[string]any   `json:"server"`
		Cards  map[string]int64 `json:"cards"`
		Series []struct {
			Date    string `json:"date"`
			Logins  int64  `json:"logins"`
			Tickets int64  `json:"tickets"`
		} `json:"series"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("响应非合法 JSON: %v", err)
	}
	if payload.Cards["loginsToday"] != 1 || payload.Cards["logins7d"] != 1 {
		t.Errorf("登录计数未随埋点累加: %+v", payload.Cards)
	}
	if len(payload.Series) != 7 {
		t.Fatalf("序列应恒为 7 天，实际 %d", len(payload.Series))
	}
	today := payload.Series[len(payload.Series)-1]
	if today.Logins != 1 {
		t.Errorf("今日登录计数不符: %+v", today)
	}
	if payload.Server["version"] == "" || payload.Server["hostname"] == "" {
		t.Errorf("服务信息缺失: %+v", payload.Server)
	}
}

func TestStatusPage(t *testing.T) {
	h := newStatusHandler(true)

	req := httptest.NewRequest("GET", "/status?token=test-status-token-0123456789abcdef", nil)
	rec := httptest.NewRecorder()
	h.Status(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("页面应 200，实际 %d", rec.Code)
	}
	if body := rec.Body.String(); len(body) == 0 || body[:15] != "<!DOCTYPE html>" {
		t.Errorf("页面内容异常")
	}

	badReq := httptest.NewRequest("GET", "/status?token=wrong", nil)
	badRec := httptest.NewRecorder()
	h.Status(badRec, badReq)
	if badRec.Code != http.StatusForbidden {
		t.Errorf("错误 token 访问页面应 403，实际 %d", badRec.Code)
	}
}

func TestStatusDisabled(t *testing.T) {
	h := newStatusHandler(false)

	pageReq := httptest.NewRequest("GET", "/status?token=test-status-token-0123456789abcdef", nil)
	pageRec := httptest.NewRecorder()
	h.Status(pageRec, pageReq)
	if pageRec.Code != http.StatusNotFound {
		t.Errorf("未启用时页面应 404，实际 %d", pageRec.Code)
	}
	if rec := getStatusAPI(t, h, "test-status-token-0123456789abcdef"); rec.Code != http.StatusNotFound {
		t.Errorf("未启用时接口应 404，实际 %d", rec.Code)
	}
}
