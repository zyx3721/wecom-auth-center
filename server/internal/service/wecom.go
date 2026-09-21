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

	"github.com/jerion/wecom-auth-center/server/internal/store"
)

// UserInfo 企业微信换取到的用户身份。档案字段在未开启 fetch_profile 时均为零值。
type UserInfo struct {
	Userid         string
	Name           string // 可为空（未开启 fetch_name 时）
	Email          string // 成员邮箱，可为空
	BizMail        string // 企业邮箱，可为空
	JobNumber      string // 员工编码，取自扩展属性，可为空
	Alias          string // 成员别名（企微后台「账号」字段），可为空
	Departments    []store.Department
	MainDepartment int64
}

// WeCom 企业微信身份接口抽象，真实实现与 mock 实现均满足此接口。
type WeCom interface {
	GetUserInfo(ctx context.Context, code string) (*UserInfo, error)
}

const qyapiBase = "https://qyapi.weixin.qq.com"

// deptNameTTL 部门名称进程内缓存时长，过期后重新查询以跟进部门改名。
const deptNameTTL = 10 * time.Minute

// ClientOptions RealClient 的构造参数。
type ClientOptions struct {
	CorpID         string
	AgentID        int
	Secret         string // 自建应用 Secret，用于 gettoken 与 getuserinfo
	ContactSecret  string // 通讯录 Secret（可选）：配置后档案接口可返回邮箱等敏感字段
	FetchName      bool   // 登录时额外调用通讯录接口取姓名
	FetchProfile   bool   // 登录时额外获取成员档案（部门/邮箱/员工编码等，含姓名）
	JobNumberField string // 员工编码取自扩展属性的字段名，为空时默认「员工编码」
}

// RealClient 企业微信真实实现：gettoken 缓存 + getuserinfo（可选 user/get 补全姓名或档案）。
type RealClient struct {
	CorpID         string
	AgentID        int
	Secret         string
	FetchName      bool
	FetchProfile   bool
	ContactSecret  string
	JobNumberField string
	// Base 便于测试替换；生产为 qyapiBase
	Base string

	httpc *http.Client

	mu            sync.Mutex
	accessToken   string
	tokenExpiry   time.Time
	contactToken  string
	contactExpiry time.Time

	deptMu   sync.Mutex
	deptName map[int64]deptNameCache
	// Now 便于测试 token 刷新逻辑
	Now func() time.Time
}

type deptNameCache struct {
	name      string
	fetchedAt time.Time
}

func NewRealClient(o ClientOptions) *RealClient {
	if o.JobNumberField == "" {
		o.JobNumberField = "员工编码"
	}
	return &RealClient{
		CorpID:         o.CorpID,
		AgentID:        o.AgentID,
		Secret:         o.Secret,
		FetchName:      o.FetchName,
		FetchProfile:   o.FetchProfile,
		ContactSecret:  o.ContactSecret,
		JobNumberField: o.JobNumberField,
		Base:           qyapiBase,
		httpc:          &http.Client{Timeout: 10 * time.Second},
		deptName:       make(map[int64]deptNameCache),
		Now:            time.Now,
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
	if c.FetchProfile {
		c.fetchProfile(ctx, ui) // 档案属增强信息，取不到不阻断登录
	} else if c.FetchName {
		ui.Name, _ = c.fetchName(ctx, token, resp.Userid) // 姓名属展示信息，取不到不阻断登录
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

// fetchProfile 调用通讯录成员详情接口补全档案；任何失败都静默降级为空字段。
func (c *RealClient) fetchProfile(ctx context.Context, ui *UserInfo) {
	token, err := c.profileToken(ctx)
	if err != nil {
		return
	}
	var resp struct {
		Errcode        int     `json:"errcode"`
		Errmsg         string  `json:"errmsg"`
		Name           string  `json:"name"`
		Email          string  `json:"email"`
		BizMail        string  `json:"biz_mail"`
		Alias          string  `json:"alias"`
		Department     []int64 `json:"department"`
		MainDepartment int64   `json:"main_department"`
		ExtAttr        struct {
			Attrs []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"attrs"`
		} `json:"extattr"`
	}
	if err := c.getJSON(ctx,
		fmt.Sprintf("%s/cgi-bin/user/get?access_token=%s&userid=%s",
			c.Base, url.QueryEscape(token), url.QueryEscape(ui.Userid)), &resp); err != nil {
		return
	}
	if resp.Errcode != 0 {
		return
	}
	ui.Name = resp.Name
	ui.Email = resp.Email
	ui.BizMail = resp.BizMail
	ui.Alias = resp.Alias
	ui.MainDepartment = resp.MainDepartment
	for _, id := range resp.Department {
		ui.Departments = append(ui.Departments, store.Department{ID: id, Name: c.departmentName(ctx, id)})
	}
	for _, attr := range resp.ExtAttr.Attrs {
		if attr.Name == c.JobNumberField {
			ui.JobNumber = attr.Value
			break
		}
	}
}

// departmentName 换算部门名称，带进程内缓存；应用可见范围外的部门返回空名。
func (c *RealClient) departmentName(ctx context.Context, id int64) string {
	c.deptMu.Lock()
	if ent, ok := c.deptName[id]; ok && c.Now().Sub(ent.fetchedAt) < deptNameTTL {
		c.deptMu.Unlock()
		return ent.name
	}
	c.deptMu.Unlock()

	name := c.lookupDepartmentName(ctx, id)
	c.deptMu.Lock()
	c.deptName[id] = deptNameCache{name: name, fetchedAt: c.Now()}
	c.deptMu.Unlock()
	return name
}

func (c *RealClient) lookupDepartmentName(ctx context.Context, id int64) string {
	token, err := c.token(ctx)
	if err != nil {
		return ""
	}
	var resp struct {
		Errcode    int    `json:"errcode"`
		Errmsg     string `json:"errmsg"`
		Department struct {
			Name string `json:"name"`
		} `json:"department"`
	}
	if err := c.getJSON(ctx,
		fmt.Sprintf("%s/cgi-bin/department/get?access_token=%s&id=%d",
			c.Base, url.QueryEscape(token), id), &resp); err != nil {
		return ""
	}
	if resp.Errcode != 0 {
		return ""
	}
	return resp.Department.Name
}

// profileToken 档案接口优先使用通讯录 Secret 的 token（可返回邮箱等敏感字段），未配置时退回应用 token。
func (c *RealClient) profileToken(ctx context.Context) (string, error) {
	if c.ContactSecret == "" {
		return c.token(ctx)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.contactToken != "" && c.Now().Before(c.contactExpiry) {
		return c.contactToken, nil
	}
	tok, expiry, err := c.requestToken(ctx, c.ContactSecret)
	if err != nil {
		return "", err
	}
	c.contactToken, c.contactExpiry = tok, expiry
	return tok, nil
}

// token 返回缓存的 access_token；到期前 5 分钟主动刷新，并发调用仅触发一次请求。
func (c *RealClient) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken != "" && c.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}
	tok, expiry, err := c.requestToken(ctx, c.Secret)
	if err != nil {
		return "", err
	}
	c.accessToken, c.tokenExpiry = tok, expiry
	return tok, nil
}

// requestToken 用指定 Secret 调用 gettoken；过期时点提前 5 分钟，避免临界点失效。
func (c *RealClient) requestToken(ctx context.Context, secret string) (string, time.Time, error) {
	var resp struct {
		Errcode     int    `json:"errcode"`
		Errmsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	u := fmt.Sprintf("%s/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		c.Base, url.QueryEscape(c.CorpID), url.QueryEscape(secret))
	if err := c.getJSON(ctx, u, &resp); err != nil {
		return "", time.Time{}, err
	}
	if resp.Errcode != 0 {
		return "", time.Time{}, fmt.Errorf("gettoken errcode=%d errmsg=%s", resp.Errcode, resp.Errmsg)
	}
	if resp.AccessToken == "" {
		return "", time.Time{}, errors.New("gettoken 未返回 access_token")
	}
	return resp.AccessToken, c.Now().Add(time.Duration(resp.ExpiresIn-300) * time.Second), nil
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

// MockClient 本地演练用：任何 code 都返回固定的模拟用户与档案。
type MockClient struct{}

func (MockClient) GetUserInfo(_ context.Context, _ string) (*UserInfo, error) {
	return &UserInfo{
		Userid:         "mockuser",
		Name:           "模拟用户",
		Email:          "mockuser@example.com",
		BizMail:        "mockuser@example.cn",
		JobNumber:      "10001",
		Alias:          "mockuser",
		Departments:    []store.Department{{ID: 2, Name: "研发部"}},
		MainDepartment: 2,
	}, nil
}
