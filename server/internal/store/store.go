// Package store 定义 state 与 ticket 的短时效存储抽象。
//
// Take 语义均为「取出即删」，天然防重放；Redis 实现阶段三接入，多实例部署时替换。
package store

import (
	"context"
	"time"
)

// StateRecord /login 生成 state 时登记的来源信息。
type StateRecord struct {
	App      string // 业务系统标识（白名单键）
	Redirect string // 登录后业务系统内的相对路径，可为空
	Remote   string // 登录发起时浏览器来源 IP，用于监控页最近登录展示
}

// Department 成员所属部门；Name 可为空（应用可见范围外的部门解析不到名称）。
type Department struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// TicketRecord /callback 生成 ticket 时登记的身份与去向。
// 档案字段在未开启 fetch_profile 时均为零值。
type TicketRecord struct {
	App            string
	Redirect       string
	Userid         string
	Name           string // 可为空（未开启 fetch_name 时）
	Email          string // 成员邮箱；新自建应用需配置通讯录 Secret 才能取到
	BizMail        string // 企业邮箱；同上
	JobNumber      string // 员工编码，取自扩展属性（字段名可配置）
	Alias          string // 成员别名（企微后台「账号」字段）
	Departments    []Department
	MainDepartment int64 // 主部门 ID，可为 0
}

// Store state/ticket 存储接口。
type Store interface {
	SaveState(ctx context.Context, state string, rec StateRecord, ttl time.Duration) error
	TakeState(ctx context.Context, state string) (StateRecord, bool)
	SaveTicket(ctx context.Context, ticket string, rec TicketRecord, ttl time.Duration) error
	TakeTicket(ctx context.Context, ticket string) (TicketRecord, bool)
}

// HealthChecker 支持健康探测的存储实现，监控页用于展示在线状态。
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}
