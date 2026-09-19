package handler

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/jerion/wecom-auth-center/server/internal/buildinfo"
	"github.com/jerion/wecom-auth-center/server/internal/metrics"
	"github.com/jerion/wecom-auth-center/server/internal/store"
)

// rejectEvents 监控页展示的拒绝类安全事件
var rejectEvents = []string{"state_reject", "verify_app_reject", "verify_sign_reject", "verify_ts_reject", "ticket_reject", "ticket_mismatch"}

// Status GET /status?token=xxx — KMS 风格监控页。
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	if !h.statusEnabled(w, r) {
		return
	}
	if !h.checkStatusToken(w, r, true) {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, statusHTML)
}

// StatusAPI GET /api/status?token=xxx — 监控页数据接口。
func (h *Handler) StatusAPI(w http.ResponseWriter, r *http.Request) {
	if !h.statusEnabled(w, r) {
		return
	}
	if !h.checkStatusToken(w, r, false) {
		return
	}
	writeJSON(w, http.StatusOK, h.statusPayload(r))
}

// statusEnabled 监控未启用时按 404 处理
func (h *Handler) statusEnabled(w http.ResponseWriter, r *http.Request) bool {
	if h.cfg.Status.Enabled {
		return true
	}
	http.NotFound(w, r)
	return false
}

// checkStatusToken 恒定时间比较访问令牌，失败时按页面/接口分别返回 403
func (h *Handler) checkStatusToken(w http.ResponseWriter, r *http.Request, page bool) bool {
	token := r.URL.Query().Get("token")
	if subtle.ConstantTimeCompare([]byte(token), []byte(h.cfg.Status.Token)) == 1 {
		return true
	}
	if page {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "<!DOCTYPE html><html lang=\"zh-CN\"><meta charset=\"utf-8\"><title>访问被拒绝</title><body style=\"font-family:system-ui;display:flex;align-items:center;justify-content:center;height:100vh;color:#6b7280\"><p>令牌无效或缺失，请以 /status?token=你的令牌 访问</p></body></html>")
		return false
	}
	writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid_token"})
	return false
}

// statusPayload 组装监控页 JSON 数据
func (h *Handler) statusPayload(r *http.Request) map[string]any {
	hostname, _ := os.Hostname()
	snap := h.metrics.Snapshot()

	storage := map[string]any{"driver": h.cfg.Store.Driver, "ok": true}
	if hc, ok := h.st.(store.HealthChecker); ok {
		if err := hc.HealthCheck(r.Context()); err != nil {
			storage["ok"] = false
			storage["error"] = err.Error()
		}
	}

	rejects := map[string]int64{}
	var rejectTotal int64
	for _, event := range rejectEvents {
		n := snap.Today[event]
		rejects[event] = n
		rejectTotal += n
	}
	rejects["total"] = rejectTotal

	series := fillSeries(snap, h.now())

	recent := h.metrics.RecentLogins()
	recentLogins := make([]map[string]any, 0, len(recent))
	for _, rec := range recent {
		recentLogins = append(recentLogins, map[string]any{
			"time":   rec.Time.Format("2006-01-02 15:04:05"),
			"app":    rec.App,
			"userid": rec.Userid,
			"name":   rec.Name,
			"remote": rec.Remote,
		})
	}

	return map[string]any{
		"server": map[string]any{
			"hostname":      hostname,
			"listen":        h.cfg.Server.Listen,
			"externalURL":   h.cfg.Server.ExternalURL,
			"version":       buildinfo.Version,
			"commit":        buildinfo.Commit,
			"buildDate":     buildinfo.BuildDate,
			"goVersion":     runtime.Version(),
			"platform":      runtime.GOOS + "/" + runtime.GOARCH,
			"startedAt":     snap.StartedAt.Format(time.RFC3339),
			"uptimeSeconds": int64(h.now().Sub(snap.StartedAt).Seconds()),
			"mock":          h.cfg.Wecom.Mock,
			"wecomMode":     string(h.cfg.Wecom.Mode),
		},
		"storage":      storage,
		"cards":        statusCards(snap),
		"rejectsToday": rejects,
		"series":       series,
		"recentLogins": recentLogins,
		"generatedAt":  h.now().Format(time.RFC3339),
	}
}

// fillSeries 补齐近 7 天（含今日）完整日期序列，无数据的天填零，保证图表连续
func fillSeries(snap metrics.Snapshot, now time.Time) []map[string]any {
	byDate := map[string]metrics.DayPoint{}
	for _, day := range snap.Days {
		byDate[day.Date] = day
	}
	series := make([]map[string]any, 0, 7)
	for i := 6; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		var logins, tickets, rejects int64
		if day, ok := byDate[date]; ok {
			logins = day.Counts["login_start"]
			tickets = day.Counts["ticket_issue"]
			for _, ev := range rejectEvents {
				rejects += day.Counts[ev]
			}
		}
		series = append(series, map[string]any{"date": date, "logins": logins, "tickets": tickets, "rejects": rejects})
	}
	return series
}

// statusCards 汇总 4 张统计卡数据
func statusCards(snap metrics.Snapshot) map[string]int64 {
	return map[string]int64{
		"logins7d":      snap.Total["login_start"],
		"tickets7d":     snap.Total["ticket_issue"],
		"loginsToday":   snap.Today["login_start"],
		"ticketsToday":  snap.Today["ticket_issue"],
		"verifyOkToday": snap.Today["verify_ok"],
	}
}
