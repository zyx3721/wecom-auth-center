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
}

func TestLoadRejects(t *testing.T) {
	cases := map[string]string{
		"external_url 非法": strings.Replace(validYAML, `https://auth.example.com`, "auth.example.com", 1),
		"缺 corpid":        strings.Replace(validYAML, `corpid: "wwX"`, `corpid: ""`, 1),
		"app_secret 太短": strings.Replace(validYAML,
			"0123456789abcdef0123456789abcdef", "short", 1),
		"无业务系统": strings.Replace(validYAML, "apps:", "apps_empty:", 1),
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeTemp(t, y)); err == nil {
				t.Fatalf("%s 应被拒绝", name)
			}
		})
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

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("缺失文件应报错")
	}
}
