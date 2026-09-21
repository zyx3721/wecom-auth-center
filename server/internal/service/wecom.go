package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jerion/wecom-auth-center/server/internal/store"
)

// UserInfo 企业微信换取到的用户身份。档案字段在未开启 fetch_profile 时均为零值。
type UserInfo struct {
	Userid      string
	Name        string             // 可为空（未开启 fetch_name 时）
	JobNumber   string             // 员工编码，取自扩展属性，可为空
	Departments []store.Department // Name 为完整层级路径，可为空（应用可见范围外）
}

// WeCom 企业微信身份接口抽象，真实实现与 mock 实现均满足此接口。
type WeCom interface {
	GetUserInfo(ctx context.Context, code string) (*UserInfo, error)
}

const qyapiBase = "https://qyapi.weixin.qq.com"

// deptNameTTL 部门信息进程内缓存时长，过期后重新查询以跟进部门调整。
const deptNameTTL = 10 * time.Minute

// maxDeptDepth 部门路径向上追溯的最大层数，防御异常数据成环。
const maxDeptDepth = 20

// ClientOptions RealClient 的构造参数。
type ClientOptions struct {
	CorpID         string
	AgentID        int
	Secret         string       // 自建应用 Secret，用于 gettoken 与 getuserinfo
	FetchName      bool         // 登录时额外调用通讯录接口取姓名
	FetchProfile   bool         // 登录时额外获取成员信息（部门/员工编码等，含姓名）
	JobNumberField string       // 员工编码取自扩展属性的字段名，为空时默认「员工编码」
	Log            *slog.Logger // 可选：档案获取失败等事件的告警日志，为 nil 时不输出
}

// RealClient 企业微信真实实现：gettoken 缓存 + getuserinfo（可选 user/get 补全姓名或档案）。
type RealClient struct {
	CorpID         string
	AgentID        int
	Secret         string
	FetchName      bool
	FetchProfile   bool
	JobNumberField string
	Log            *slog.Logger
	// Base 便于测试替换；生产为 qyapiBase
	Base string

	httpc *http.Client

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time

	deptMu   sync.Mutex
	deptInfo map[int64]deptCacheEntry
	// Now 便于测试 token 刷新逻辑
	Now func() time.Time
}

// deptInfo 单个部门的名称与父部门 ID
type deptInfo struct {
	name     string
	parentID int64
}

type deptCacheEntry struct {
	info      deptInfo
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
		JobNumberField: o.JobNumberField,
		Log:            o.Log,
		Base:           qyapiBase,
		httpc:          &http.Client{Timeout: 10 * time.Second},
		deptInfo:       make(map[int64]deptCacheEntry),
		Now:            time.Now,
	}
}

// warn 输出档案获取的降级告警，未注入 logger 时为空操作
func (c *RealClient) warn(msg string, args ...any) {
	if c.Log != nil {
		c.Log.Warn(msg, args...)
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

// userDetail 通讯录读取成员接口的响应体
type userDetail struct {
	Errcode    int     `json:"errcode"`
	Errmsg     string  `json:"errmsg"`
	Name       string  `json:"name"`
	Department []int64 `json:"department"`
	ExtAttr    struct {
		Attrs []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"attrs"`
	} `json:"extattr"`
}

// fetchProfile 调用通讯录成员详情接口补全档案；失败时记 Warn 并降级为空，不阻断登录。
func (c *RealClient) fetchProfile(ctx context.Context, ui *UserInfo) {
	token, err := c.token(ctx)
	if err != nil {
		c.warn("应用 Secret 换取 token 失败，档案降级为空", "error", err)
		return
	}
	detail, err := c.userDetail(ctx, token, ui.Userid)
	if err != nil {
		c.warn("获取成员信息请求失败", "error", err)
		return
	}
	if detail.Errcode != 0 {
		c.warn("获取成员信息被企微拒绝", "errcode", detail.Errcode, "errmsg", detail.Errmsg)
		return
	}
	ui.Name = detail.Name
	for _, id := range detail.Department {
		ui.Departments = append(ui.Departments, store.Department{ID: id, Name: c.departmentPath(ctx, id)})
	}
	for _, attr := range detail.ExtAttr.Attrs {
		if attr.Name == c.JobNumberField {
			ui.JobNumber = attr.Value
			break
		}
	}
}

// userDetail 用指定 token 调用读取成员接口
func (c *RealClient) userDetail(ctx context.Context, token, userid string) (userDetail, error) {
	var resp userDetail
	err := c.getJSON(ctx,
		fmt.Sprintf("%s/cgi-bin/user/get?access_token=%s&userid=%s",
			c.Base, url.QueryEscape(token), url.QueryEscape(userid)), &resp)
	return resp, err
}

// departmentPath 沿 parentid 向上追溯拼出完整部门路径（不含根部门），如「研发中心/研发部」。
// 追溯中查不到名称（如应用可见范围外）时止于能查到的部分。
func (c *RealClient) departmentPath(ctx context.Context, id int64) string {
	var parts []string
	seen := make(map[int64]bool)
	cur := id
	for cur != 0 && cur != 1 && !seen[cur] && len(parts) < maxDeptDepth {
		seen[cur] = true
		info := c.departmentInfo(ctx, cur)
		if info.name == "" {
			break
		}
		parts = append([]string{info.name}, parts...)
		cur = info.parentID
	}
	return strings.Join(parts, "/")
}

// departmentInfo 查询部门名称与父部门 ID，带进程内缓存；可见范围外的部门返回零值并同样入缓存。
func (c *RealClient) departmentInfo(ctx context.Context, id int64) deptInfo {
	c.deptMu.Lock()
	if ent, ok := c.deptInfo[id]; ok && c.Now().Sub(ent.fetchedAt) < deptNameTTL {
		c.deptMu.Unlock()
		return ent.info
	}
	c.deptMu.Unlock()

	info := c.lookupDepartmentInfo(ctx, id)
	c.deptMu.Lock()
	c.deptInfo[id] = deptCacheEntry{info: info, fetchedAt: c.Now()}
	c.deptMu.Unlock()
	return info
}

func (c *RealClient) lookupDepartmentInfo(ctx context.Context, id int64) deptInfo {
	token, err := c.token(ctx)
	if err != nil {
		return deptInfo{}
	}
	var resp struct {
		Errcode    int    `json:"errcode"`
		Errmsg     string `json:"errmsg"`
		Department struct {
			Name     string `json:"name"`
			ParentID int64  `json:"parentid"`
		} `json:"department"`
	}
	if err := c.getJSON(ctx,
		fmt.Sprintf("%s/cgi-bin/department/get?access_token=%s&id=%d",
			c.Base, url.QueryEscape(token), id), &resp); err != nil {
		return deptInfo{}
	}
	if resp.Errcode != 0 {
		c.warn("部门信息查询失败", "department_id", id, "errcode", resp.Errcode, "errmsg", resp.Errmsg)
		return deptInfo{}
	}
	return deptInfo{name: resp.Department.Name, parentID: resp.Department.ParentID}
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
		Userid:      "mockuser",
		Name:        "模拟用户",
		JobNumber:   "10001",
		Departments: []store.Department{{ID: 2, Name: "研发中心/研发部"}},
	}, nil
}
