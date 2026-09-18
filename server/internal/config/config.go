// Package config 负责加载并校验认证中心的 YAML 配置。
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Mode 企业微信授权入口：qrcode 为 PC 扫码（wwlogin），inside 为企业微信内置浏览器网页授权。
type Mode string

const (
	ModeQrcode Mode = "qrcode"
	ModeInside Mode = "inside"
)

// AppConfig 描述一个接入的业务系统。
type AppConfig struct {
	Domain       string `yaml:"domain"`        // 如 https://oa.example.com
	CallbackPath string `yaml:"callback_path"` // 如 /sso/login
	AppSecret    string `yaml:"app_secret"`    // verify 签名密钥，与企业微信 secret 无关
}

// Config 认证中心总配置。
type Config struct {
	Server struct {
		Listen      string `yaml:"listen"`
		ExternalURL string `yaml:"external_url"` // 对外地址，用于拼接 redirect_uri
		StaticDir   string `yaml:"static_dir"`
		TrustProxy  bool   `yaml:"trust_proxy"` // 部署在反代后时从 X-Real-IP 取客户端 IP
	} `yaml:"server"`

	Wecom struct {
		CorpID    string `yaml:"corpid"`
		AgentID   int    `yaml:"agentid"`
		Secret    string `yaml:"secret"`
		Mode      Mode   `yaml:"mode"`
		Mock      bool   `yaml:"mock"`       // true 时用假企业微信实现，本地演练全流程
		FetchName bool   `yaml:"fetch_name"` // true 时额外调用通讯录接口取姓名
	} `yaml:"wecom"`

	TTL struct {
		State        time.Duration `yaml:"state"`
		Ticket       time.Duration `yaml:"ticket"`
		VerifyTSSkew time.Duration `yaml:"verify_ts_skew"`
	} `yaml:"ttl"`

	Apps map[string]*AppConfig `yaml:"apps"`

	RateLimit struct {
		LoginPerMinute  int `yaml:"login_per_minute"`
		VerifyPerMinute int `yaml:"verify_per_minute"`
	} `yaml:"ratelimit"`
}

// Load 从 path 读取并校验配置。
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置 %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置 %s: %w", path, err)
	}
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Server.Listen == "" {
		c.Server.Listen = ":8700"
	}
	if c.Server.StaticDir == "" {
		c.Server.StaticDir = "web/static"
	}
	if c.Wecom.Mode == "" {
		c.Wecom.Mode = ModeQrcode
	}
	if c.TTL.State == 0 {
		c.TTL.State = 5 * time.Minute
	}
	if c.TTL.Ticket == 0 {
		c.TTL.Ticket = 60 * time.Second
	}
	if c.TTL.VerifyTSSkew == 0 {
		c.TTL.VerifyTSSkew = 60 * time.Second
	}
	if c.RateLimit.LoginPerMinute == 0 {
		c.RateLimit.LoginPerMinute = 30
	}
	if c.RateLimit.VerifyPerMinute == 0 {
		c.RateLimit.VerifyPerMinute = 120
	}
}

func (c *Config) validate() error {
	switch {
	case !strings.HasPrefix(c.Server.ExternalURL, "http://") &&
		!strings.HasPrefix(c.Server.ExternalURL, "https://"):
		return fmt.Errorf("server.external_url 必须以 http(s):// 开头")
	case strings.HasSuffix(c.Server.ExternalURL, "/"):
		return fmt.Errorf("server.external_url 不能以 / 结尾")
	case !c.Wecom.Mock && c.Wecom.CorpID == "":
		return fmt.Errorf("wecom.corpid 不能为空")
	case !c.Wecom.Mock && c.Wecom.Secret == "":
		return fmt.Errorf("wecom.secret 不能为空")
	case c.Wecom.Mode != ModeQrcode && c.Wecom.Mode != ModeInside:
		return fmt.Errorf("wecom.mode 只能是 qrcode 或 inside")
	case len(c.Apps) == 0:
		return fmt.Errorf("至少需要登记一个 apps 业务系统")
	}
	for name, app := range c.Apps {
		switch {
		case app == nil:
			return fmt.Errorf("apps.%s 配置为空", name)
		case !strings.HasPrefix(app.Domain, "https://"):
			return fmt.Errorf("apps.%s.domain 必须为 https:// 开头", name)
		case !strings.HasPrefix(app.CallbackPath, "/"):
			return fmt.Errorf("apps.%s.callback_path 必须以 / 开头", name)
		case len(app.AppSecret) < 32:
			return fmt.Errorf("apps.%s.app_secret 长度不得小于 32（openssl rand -hex 32 生成）", name)
		}
	}
	return nil
}
