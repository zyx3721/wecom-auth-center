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
)

// verifyRequest 业务系统后端调用 /api/verify 的请求体。
type verifyRequest struct {
	App    string `json:"app"`
	Ticket string `json:"ticket"`
	TS     int64  `json:"ts"`   // Unix 秒
	Sign   string `json:"sign"` // hex(HMAC-SHA256(key=app_secret, msg=app+"\n"+ticket+"\n"+ts))
}

// Verify POST /api/verify
// 签名校验 -> 时间偏差校验 -> ticket 一次性消费（取出即删）-> 返回身份。
// 顺序上先验签名再消费 ticket，无效请求不会冲掉有效凭证。
func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeVerifyError(w, http.StatusBadRequest, "invalid_body")
		return
	}

	app, ok := h.appOf(req.App)
	if !ok {
		writeVerifyError(w, http.StatusUnauthorized, "invalid_app")
		return
	}
	if req.Ticket == "" || req.TS <= 0 || req.Sign == "" {
		writeVerifyError(w, http.StatusBadRequest, "invalid_body")
		return
	}

	if !verifySign(req, app.AppSecret) {
		h.log.Warn("verify 签名校验失败", "app", req.App)
		writeVerifyError(w, http.StatusUnauthorized, "invalid_sign")
		return
	}
	if skew := h.now().Sub(time.Unix(req.TS, 0)).Abs(); skew > h.cfg.TTL.VerifyTSSkew {
		writeVerifyError(w, http.StatusUnauthorized, "expired_ts")
		return
	}

	rec, ok := h.sso.ConsumeTicket(r.Context(), req.Ticket)
	if !ok {
		h.log.Warn("ticket 校验失败", "app", req.App)
		writeVerifyError(w, http.StatusUnauthorized, "invalid_ticket")
		return
	}
	if rec.App != req.App {
		h.log.Warn("ticket 归属不匹配", "ticket_app", rec.App, "req_app", req.App)
		writeVerifyError(w, http.StatusUnauthorized, "invalid_ticket")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"userid": rec.Userid,
		"name":   rec.Name,
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
