package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeWecom 模拟企微接口，记录全部请求 URI 供断言。
type fakeWecom struct {
	mu          sync.Mutex
	requests    []string
	failUserGet bool
}

func (f *fakeWecom) uriList() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requests...)
}

func (f *fakeWecom) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, r.URL.RequestURI())
	failUserGet := f.failUserGet
	f.mu.Unlock()

	switch r.URL.Path {
	case "/cgi-bin/gettoken":
		fmt.Fprint(w, `{"errcode":0,"access_token":"tok-appsecret","expires_in":7200}`)
	case "/cgi-bin/auth/getuserinfo":
		fmt.Fprint(w, `{"errcode":0,"userid":"zhangsan"}`)
	case "/cgi-bin/user/get":
		if failUserGet {
			fmt.Fprint(w, `{"errcode":48009,"errmsg":"api forbidden for contact assistant"}`)
			return
		}
		fmt.Fprint(w, `{"errcode":0,"name":"张三",`+
			`"department":[2,3],"main_department":2,`+
			`"extattr":{"attrs":[{"name":"员工编码","value":"10001"},{"name":"职级","value":"P7"}]}}`)
	case "/cgi-bin/department/get":
		switch r.URL.Query().Get("id") {
		case "2":
			fmt.Fprint(w, `{"errcode":0,"department":{"id":2,"name":"研发部","parentid":10}}`)
		case "10":
			fmt.Fprint(w, `{"errcode":0,"department":{"id":10,"name":"研发中心","parentid":1}}`)
		default: // 其余部门：应用可见范围外
			fmt.Fprint(w, `{"errcode":60003,"errmsg":"不在可见范围"}`)
		}
	default:
		http.NotFound(w, r)
	}
}

// newProfileClient 基于假企微服务构造 RealClient。
func newProfileClient(t *testing.T, mutate func(*ClientOptions)) (*RealClient, *fakeWecom) {
	t.Helper()
	fake := &fakeWecom{}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	opts := ClientOptions{
		CorpID:  "wwX",
		AgentID: 1,
		Secret:  "appsecret",
	}
	if mutate != nil {
		mutate(&opts)
	}
	c := NewRealClient(opts)
	c.Base = srv.URL
	return c, fake
}

func TestGetUserInfoProfile(t *testing.T) {
	c, fake := newProfileClient(t, func(o *ClientOptions) { o.FetchProfile = true })
	ui, err := c.GetUserInfo(context.Background(), "code")
	if err != nil {
		t.Fatalf("GetUserInfo 不应报错: %v", err)
	}
	if ui.Name != "张三" || ui.JobNumber != "10001" {
		t.Fatalf("档案字段不符: %+v", ui)
	}
	if len(ui.Departments) != 2 || ui.Departments[0].Name != "研发中心/研发部" || ui.Departments[1].Name != "" {
		t.Fatalf("部门完整路径解析不符: %+v", ui.Departments)
	}

	uris := strings.Join(fake.uriList(), "\n")
	if !strings.Contains(uris, "user/get?access_token=tok-appsecret") {
		t.Fatalf("user/get 应使用应用 token: %s", uris)
	}
	if !strings.Contains(uris, "department/get?access_token=tok-appsecret") {
		t.Fatalf("部门路径换算应使用应用 token: %s", uris)
	}
}

func TestGetUserInfoFetchNameOnly(t *testing.T) {
	c, fake := newProfileClient(t, func(o *ClientOptions) { o.FetchName = true })
	ui, err := c.GetUserInfo(context.Background(), "code")
	if err != nil {
		t.Fatalf("GetUserInfo 不应报错: %v", err)
	}
	if ui.Name != "张三" || ui.JobNumber != "" || ui.Departments != nil {
		t.Fatalf("仅取姓名时不应填充档案字段: %+v", ui)
	}
	if strings.Contains(strings.Join(fake.uriList(), "\n"), "department/get") {
		t.Fatalf("仅取姓名不应调用部门接口")
	}
}

func TestGetUserInfoProfileDegradesOnError(t *testing.T) {
	c, fake := newProfileClient(t, func(o *ClientOptions) { o.FetchProfile = true })
	fake.failUserGet = true // 档案请求失败应记 Warn 并降级，不影响登录
	ui, err := c.GetUserInfo(context.Background(), "code")
	if err != nil {
		t.Fatalf("档案失败不应阻断登录: %v", err)
	}
	if ui.Userid != "zhangsan" || ui.Name != "" {
		t.Fatalf("降级后仅应有 userid: %+v", ui)
	}
}

func TestDepartmentPathCache(t *testing.T) {
	c, fake := newProfileClient(t, func(o *ClientOptions) { o.FetchProfile = true })
	if _, err := c.GetUserInfo(context.Background(), "code"); err != nil {
		t.Fatalf("GetUserInfo: %v", err)
	}
	deptCalls := 0
	for _, uri := range fake.uriList() {
		if strings.Contains(uri, "department/get") {
			deptCalls++
		}
	}
	if deptCalls != 3 { // 部门 2 的路径需查 2 与 10；部门 3 可见范围外查 1 次，均入缓存
		t.Fatalf("首轮应共请求 3 次部门接口，实际 %d", deptCalls)
	}
	if _, err := c.GetUserInfo(context.Background(), "code"); err != nil {
		t.Fatalf("GetUserInfo: %v", err)
	}
	for _, uri := range fake.uriList() {
		if strings.Contains(uri, "department/get") {
			deptCalls--
		}
	}
	if deptCalls != 0 {
		t.Fatalf("缓存生效时不应再次请求部门接口")
	}
}

func TestJobNumberFieldConfigurable(t *testing.T) {
	c, _ := newProfileClient(t, func(o *ClientOptions) {
		o.FetchProfile = true
		o.JobNumberField = "职级"
	})
	ui, err := c.GetUserInfo(context.Background(), "code")
	if err != nil {
		t.Fatalf("GetUserInfo: %v", err)
	}
	if ui.JobNumber != "P7" {
		t.Fatalf("自定义扩展属性字段应生效，实际 %q", ui.JobNumber)
	}
}

func TestJobNumberFieldMismatch(t *testing.T) {
	c, _ := newProfileClient(t, func(o *ClientOptions) {
		o.FetchProfile = true
		o.JobNumberField = "不存在的字段"
	})
	ui, err := c.GetUserInfo(context.Background(), "code")
	if err != nil {
		t.Fatalf("字段名不匹配不应报错: %v", err)
	}
	if ui.JobNumber != "" {
		t.Fatalf("字段名不匹配时 job_number 应为空串，实际 %q", ui.JobNumber)
	}
}
