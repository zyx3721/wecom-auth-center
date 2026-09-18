package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
)

// Healthz GET /healthz
func (h *Handler) Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

// MockScan GET /mock/scan?code=xxx&state=xxx
// 仅 mock 模式注册：模拟「扫码确认」页，点击后进入 /callback 走完整后端流程。
func (h *Handler) MockScan(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, mockScanHTML,
		q.Get("code"), q.Get("state"),
		q.Get("code"), q.Get("state"))
}

// renderError 统一错误页：不泄露内部细节。
func (h *Handler) renderError(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(errorHTML))
}

func mockCode() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b) // crypto/rand.Read 按文档保证成功
	return "mock-" + hex.EncodeToString(b)
}

func itoa(n int) string { return strconv.Itoa(n) }

func slogErr(err error) slog.Attr { return slog.Any("error", err) }

const errorHTML = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<title>登录失败</title>
<style>body{font-family:system-ui,sans-serif;display:flex;justify-content:center;padding-top:15vh;color:#333}
.box{text-align:center}.box a{color:#0082ef}</style></head>
<body><div class="box"><h2>登录失败</h2>
<p>登录流程未完成或已过期，请返回业务系统重新发起登录。</p>
<p style="color:#999">如有疑问请联系管理员。</p></div></body></html>`

const mockScanHTML = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>模拟扫码（mock）</title>
<style>body{font-family:system-ui,sans-serif;display:flex;justify-content:center;padding-top:15vh}
.box{text-align:center;padding:2rem 3rem;border:1px dashed #bbb;border-radius:8px}
.btn{display:inline-block;margin-top:1rem;padding:.6rem 1.4rem;background:#0082ef;color:#fff;
text-decoration:none;border-radius:4px}</style></head>
<body><div class="box"><h2>企业微信扫码（模拟）</h2>
<p>code: <code>%s</code> · state: <code>%s</code></p>
<a class="btn" href="/callback?code=%s&state=%s">模拟扫码成功</a></div></body></html>`
