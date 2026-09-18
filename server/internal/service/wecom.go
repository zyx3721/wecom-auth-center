package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// UserInfo 企业微信换取到的用户身份。Name 可为空。
type UserInfo struct {
	Userid string
	Name   string
}

// WeCom 企业微信身份接口抽象，真实实现与 mock 实现均满足此接口。
type WeCom interface {
	GetUserInfo(ctx context.Context, code string) (*UserInfo, error)
}

const qyapiBase = "https://qyapi.weixin.qq.com"

// RealClient 企业微信真实实现：gettoken 缓存 + getuserinfo（可选 user/get 补全姓名）。
type RealClient struct {
	CorpID    string
	AgentID   int
	Secret    string
	FetchName bool
	// Base 便于测试替换；生产为 qyapiBase
	Base string

	httpc *http.Client

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
	// Now 便于测试 token 刷新逻辑
	Now func() time.Time
}

func NewRealClient(corpID string, agentID int, secret string, fetchName bool) *RealClient {
	return &RealClient{
		CorpID:    corpID,
		AgentID:   agentID,
		Secret:    secret,
		FetchName: fetchName,
		Base:      qyapiBase,
		httpc:     &http.Client{Timeout: 10 * time.Second},
		Now:       time.Now,
	}
}

// GetUserInfo 用 code 换取用户身份。
func (c *RealClient) GetUserInfo(ctx context.Context, code string) (*UserInfo, error) {
	token, err := c.token(ctx)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
		Userid  string `json:"userid"`
	}
	if err := c.getJSON(ctx,
		fmt.Sprintf("%s/cgi-bin/auth/getuserinfo?access_token=%s&code=%s",
			c.Base, url.QueryEscape(token), url.QueryEscape(code)), &resp); err != nil {
		return nil, err
	}
	if resp.Errcode != 0 {
		return nil, fmt.Errorf("getuserinfo errcode=%d errmsg=%s", resp.Errcode, resp.Errmsg)
	}
	if resp.Userid == "" {
		return nil, errors.New("getuserinfo 未返回 userid（可能为非企业成员）")
	}

	ui := &UserInfo{Userid: resp.Userid}
	if c.FetchName {
		ui.Name, _ = c.fetchName(ctx, token, resp.Userid) // 姓名属于展示信息，取不到不阻断登录
	}
	return ui, nil
}

func (c *RealClient) fetchName(ctx context.Context, token, userid string) (string, error) {
	var resp struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
		Name    string `json:"name"`
	}
	if err := c.getJSON(ctx,
		fmt.Sprintf("%s/cgi-bin/user/get?access_token=%s&userid=%s",
			c.Base, url.QueryEscape(token), url.QueryEscape(userid)), &resp); err != nil {
		return "", err
	}
	if resp.Errcode != 0 {
		return "", fmt.Errorf("user/get errcode=%d errmsg=%s", resp.Errcode, resp.Errmsg)
	}
	return resp.Name, nil
}

// token 返回缓存的 access_token；到期前 5 分钟主动刷新，并发调用仅触发一次请求。
func (c *RealClient) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken != "" && c.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}

	var resp struct {
		Errcode     int    `json:"errcode"`
		Errmsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	u := fmt.Sprintf("%s/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		c.Base, url.QueryEscape(c.CorpID), url.QueryEscape(c.Secret))
	if err := c.getJSON(ctx, u, &resp); err != nil {
		return "", err
	}
	if resp.Errcode != 0 {
		return "", fmt.Errorf("gettoken errcode=%d errmsg=%s", resp.Errcode, resp.Errmsg)
	}
	if resp.AccessToken == "" {
		return "", errors.New("gettoken 未返回 access_token")
	}
	c.accessToken = resp.AccessToken
	// 提前 5 分钟过期，避免临界点失效
	c.tokenExpiry = c.Now().Add(time.Duration(resp.ExpiresIn-300) * time.Second)
	return c.accessToken, nil
}

func (c *RealClient) getJSON(ctx context.Context, u string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("构造请求 %s: %w", u, err)
	}
	httpResp, err := c.httpc.Do(req)
	if err != nil {
		return fmt.Errorf("请求企业微信接口失败: %w", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("企业微信接口 HTTP %d: %s", httpResp.StatusCode, u)
	}
	if err := json.NewDecoder(httpResp.Body).Decode(out); err != nil {
		return fmt.Errorf("解析企业微信响应失败: %w", err)
	}
	return nil
}

// MockClient 本地演练用：任何 code 都返回固定的模拟用户。
type MockClient struct{}

func (MockClient) GetUserInfo(_ context.Context, _ string) (*UserInfo, error) {
	return &UserInfo{Userid: "mockuser", Name: "模拟用户"}, nil
}
