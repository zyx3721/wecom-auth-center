package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jerion/wecom-auth-center/server/internal/middleware"
	"github.com/jerion/wecom-auth-center/server/internal/store"
)

// verifyRequest 业务系统后端调用 /api/verify 的请求体。
type verifyRequest struct {
	App    string `json:"app"`
	Ticket string `json:"ticket"`
	TS     int64  `json:"ts"`   // Unix 秒
	Sign   string `json:"sign"` // hex(HMAC-SHA256(key=app_secret, msg=app+"\n"+ticket+"\n"+ts))
}

// verifyResponse /api/verify 成功响应；档案字段在未开启对应开关时为零值。
type verifyResponse struct {
	Userid      string             `json:"userid"`
	Name        string             `json:"name"`
	JobNumber   string             `json:"job_number"`
	Departments []store.Department `json:"departments"`
}

// Verify POST /api/verify
// 签名校验 -> 时间偏差校验 -> ticket 一次性消费（取出即删）-> 返回身份。
// 顺序上先验签名再消费 ticket，无效请求不会冲掉有效凭证。
func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	remote := middleware.RealIP(h.cfg.Server.TrustProxy)(r)
	var req verifyRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeVerifyError(w, http.StatusBadRequest, "invalid_body")
		return
	}

	app, ok := h.appOf(req.App)
	if !ok {
		h.track("verify_app_reject")
		h.audit.Event("verify_app_reject", "app", req.App, "remote", remote)
		writeVerifyError(w, http.StatusUnauthorized, "invalid_app")
		return
	}
	if req.Ticket == "" || req.TS <= 0 || req.Sign == "" {
		writeVerifyError(w, http.StatusBadRequest, "invalid_body")
		return
	}

	if !verifySign(req, app.AppSecret) {
		h.log.Warn("verify 签名校验失败", "app", req.App, "remote", remote)
		h.track("verify_sign_reject")
		h.audit.Event("verify_sign_reject", "app", req.App, "remote", remote)
		writeVerifyError(w, http.StatusUnauthorized, "invalid_sign")
		return
	}
	if skew := h.now().Sub(time.Unix(req.TS, 0)).Abs(); skew > h.cfg.TTL.VerifyTSSkew {
		h.track("verify_ts_reject")
		h.audit.Event("verify_ts_reject", "app", req.App, "remote", remote)
		writeVerifyError(w, http.StatusUnauthorized, "expired_ts")
		return
	}

	rec, ok := h.sso.ConsumeTicket(r.Context(), req.Ticket)
	if !ok {
		h.log.Warn("ticket 校验失败", "app", req.App, "remote", remote)
		h.track("ticket_reject")
		h.audit.Event("ticket_reject", "app", req.App, "remote", remote)
		writeVerifyError(w, http.StatusUnauthorized, "invalid_ticket")
		return
	}
	if rec.App != req.App {
		h.log.Warn("ticket 归属不匹配", "ticket_app", rec.App, "req_app", req.App, "remote", remote)
		h.track("ticket_mismatch")
		h.audit.Event("ticket_mismatch", "app", req.App, "ticket_app", rec.App, "remote", remote)
		writeVerifyError(w, http.StatusUnauthorized, "invalid_ticket")
		return
	}

	h.track("verify_ok")
	h.audit.Event("verify_ok", "app", req.App, "userid", rec.Userid, "remote", remote)
	depts := rec.Departments
	if depts == nil {
		depts = []store.Department{}
	}
	writeJSON(w, http.StatusOK, verifyResponse{
		Userid:      rec.Userid,
		Name:        rec.Name,
		JobNumber:   rec.JobNumber,
		Departments: depts,
	})
}

func verifySign(req verifyRequest, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%s\n%s\n%s", req.App, req.Ticket, strconv.FormatInt(req.TS, 10))
	expected := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(expected), []byte(req.Sign)) == 1
}

// SignTicket 供业务系统接入方参考的签名算法实现（与 handler/verify.go 保持一致）。
func SignTicket(secret, app, ticket string, ts int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%s\n%s\n%s", app, ticket, strconv.FormatInt(ts, 10))
	return hex.EncodeToString(mac.Sum(nil))
}

func writeVerifyError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
