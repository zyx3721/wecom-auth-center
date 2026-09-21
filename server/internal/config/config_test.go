package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("写临时配置: %v", err)
	}
	return path
}

const validYAML = `
server:
  external_url: "https://auth.example.com"
wecom:
  corpid: "wwX"
  agentid: 1
  secret: "s"
  mode: "qrcode"
apps:
  oa:
    domain: "https://oa.example.com"
    callback_path: "/sso/login"
    app_secret: "0123456789abcdef0123456789abcdef"
`

func TestLoadValid(t *testing.T) {
	cfg, err := Load(writeTemp(t, validYAML))
	if err != nil {
		t.Fatalf("合法配置应加载成功: %v", err)
	}
	if cfg.TTL.Ticket != 60*time.Second {
		t.Fatalf("TTL 默认值应为 60s，实际 %v", cfg.TTL.Ticket)
	}
	if cfg.RateLimit.LoginPerMinute != 30 {
		t.Fatalf("限流默认值应为 30，实际 %d", cfg.RateLimit.LoginPerMinute)
	}
	if cfg.Store.Driver != "memory" {
		t.Fatalf("存储驱动默认值应为 memory，实际 %s", cfg.Store.Driver)
	}
	if cfg.Audit.Path != "audit.log" {
		t.Fatalf("审计文件默认路径应为 audit.log，实际 %s", cfg.Audit.Path)
	}
	if cfg.Status.DataPath != "status-metrics.json" {
		t.Fatalf("监控统计默认路径应为 status-metrics.json，实际 %s", cfg.Status.DataPath)
	}
	if cfg.Wecom.JobNumberExtattr != "员工编码" {
		t.Fatalf("员工编码扩展属性字段默认值应为「员工编码」，实际 %s", cfg.Wecom.JobNumberExtattr)
	}
}

func TestLoadRejects(t *testing.T) {
	cases := map[string]string{
		"external_url 非法": strings.Replace(validYAML, `https://auth.example.com`, "auth.example.com", 1),
		"缺 corpid":        strings.Replace(validYAML, `corpid: "wwX"`, `corpid: ""`, 1),
		"app_secret 太短": strings.Replace(validYAML,
			"0123456789abcdef0123456789abcdef", "short", 1),
		"无业务系统": strings.Replace(validYAML, "apps:", "apps_empty:", 1),
		"存储驱动非法": strings.Replace(validYAML, "apps:", `store:
  driver: etcd
apps:`, 1),
		"redis 缺地址": strings.Replace(validYAML, "apps:", `store:
  driver: redis
apps:`, 1),
		"domain 协议非法": strings.Replace(validYAML, `https://oa.example.com`, "ftp://oa.example.com", 1),
		"status 缺 token": strings.Replace(validYAML, "apps:", `status:
  enabled: true
apps:`, 1),
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeTemp(t, y)); err == nil {
				t.Fatalf("%s 应被拒绝", name)
			}
		})
	}
}

func TestLoadRedisDriverValid(t *testing.T) {
	y := strings.Replace(validYAML, "apps:", `store:
  driver: redis
  redis:
    addr: "127.0.0.1:6379"
    db: 1
audit:
  enabled: true
  path: "/var/log/wecom-audit.log"
apps:`, 1)
	cfg, err := Load(writeTemp(t, y))
	if err != nil {
		t.Fatalf("redis 存储与审计配置应加载成功: %v", err)
	}
	if cfg.Store.Driver != "redis" || cfg.Store.Redis.Addr != "127.0.0.1:6379" || cfg.Store.Redis.DB != 1 {
		t.Fatalf("store 配置解析不符: %+v", cfg.Store)
	}
	if !cfg.Audit.Enabled || cfg.Audit.Path != "/var/log/wecom-audit.log" {
		t.Fatalf("audit 配置解析不符: %+v", cfg.Audit)
	}
}

func TestLoadMockAllowsEmptyCorpID(t *testing.T) {
	y := strings.Replace(validYAML, `  mode: "qrcode"`, `  mode: "qrcode"
  mock: true`, 1)
	y = strings.Replace(y, `corpid: "wwX"`, `corpid: ""`, 1)
	if _, err := Load(writeTemp(t, y)); err != nil {
		t.Fatalf("mock 模式应允许空 corpid: %v", err)
	}
}

func TestLoadHTTPDomainAllowed(t *testing.T) {
	y := strings.Replace(validYAML, `https://oa.example.com`, "http://oa.example.com", 1)
	cfg, err := Load(writeTemp(t, y))
	if err != nil {
		t.Fatalf("内网 HTTP 域名应允许配置: %v", err)
	}
	if cfg.Apps["oa"].Domain != "http://oa.example.com" {
		t.Fatalf("HTTP 域名解析不符: %s", cfg.Apps["oa"].Domain)
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("缺失文件应报错")
	}
}
