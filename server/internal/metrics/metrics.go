// Package metrics 提供监控页的按日事件计数、7 天留存与 JSON 文件持久化。
package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/jerion/wecom-auth-center/server/internal/store"
)

// retainedDays 统计保留的天数（含今日）
const retainedDays = 7

// recentCap 最近登录记录保留条数
const recentCap = 50

// LoginRecord 单次扫码登录成功的流水记录。档案字段在未开启 fetch_profile 时为零值。
type LoginRecord struct {
	Time           time.Time          `json:"time"`
	App            string             `json:"app"`
	Userid         string             `json:"userid"`
	Name           string             `json:"name,omitempty"`
	Remote         string             `json:"remote,omitempty"`
	Email          string             `json:"email,omitempty"`
	BizMail        string             `json:"biz_mail,omitempty"`
	JobNumber      string             `json:"job_number,omitempty"`
	Alias          string             `json:"alias,omitempty"`
	Departments    []store.Department `json:"departments,omitempty"`
	MainDepartment int64              `json:"main_department,omitempty"`
}

// Metrics 按日分桶的事件计数器，事件名与审计事件一致。
type Metrics struct {
	mu      sync.Mutex
	days    map[string]map[string]int64
	recent  []LoginRecord
	started time.Time
	now     func() time.Time
}

// DayPoint 单日各事件计数。
type DayPoint struct {
	Date   string           `json:"date"`
	Counts map[string]int64 `json:"counts"`
}

// Snapshot 监控页所需的计数快照。
type Snapshot struct {
	StartedAt time.Time
	Days      []DayPoint       // 旧→新，仅含窗口内有数据的天（接口层负责补零）
	Today     map[string]int64 // 今日各事件计数
	Total     map[string]int64 // 近 7 天累计
}

// New 创建计数器并记录进程启动时间
func New(now func() time.Time) *Metrics {
	if now == nil {
		now = time.Now
	}
	return &Metrics{days: map[string]map[string]int64{}, started: now(), now: now}
}

// StartedAt 进程启动时间
func (m *Metrics) StartedAt() time.Time {
	return m.started
}

// Inc 指定事件今日计数加一，nil 接收者为空操作
func (m *Metrics) Inc(event string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	day := m.now().Format("2006-01-02")
	if m.days[day] == nil {
		m.days[day] = map[string]int64{}
	}
	m.days[day][event]++
}

// RecordLogin 记录一条扫码登录成功的流水，nil 接收者为空操作
func (m *Metrics) RecordLogin(rec LoginRecord) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if rec.Time.IsZero() {
		rec.Time = m.now()
	}
	m.recent = append([]LoginRecord{rec}, m.recent...)
	if len(m.recent) > recentCap {
		m.recent = m.recent[:recentCap]
	}
}

// RecentLogins 返回最近的登录流水副本，新的在前，nil 接收者返回空切片
func (m *Metrics) RecentLogins() []LoginRecord {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]LoginRecord, len(m.recent))
	copy(out, m.recent)
	return out
}

// Snapshot 输出逐日序列（旧→新）、今日计数与近 7 天累计，nil 接收者返回空快照
func (m *Metrics) Snapshot() Snapshot {
	if m == nil {
		return Snapshot{Today: map[string]int64{}, Total: map[string]int64{}}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked()
	days := make([]DayPoint, 0, len(m.days))
	total := map[string]int64{}
	for _, date := range m.sortedDaysLocked() {
		counts := map[string]int64{}
		for event, n := range m.days[date] {
			counts[event] = n
			total[event] += n
		}
		days = append(days, DayPoint{Date: date, Counts: counts})
	}
	today := map[string]int64{}
	if counts, ok := m.days[m.now().Format("2006-01-02")]; ok {
		for event, n := range counts {
			today[event] = n
		}
	}
	return Snapshot{StartedAt: m.started, Days: days, Today: today, Total: total}
}

// Save 将计数原子落盘（临时文件 + rename），空目录自动创建
func (m *Metrics) Save(path string) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	m.pruneLocked()
	model := persistModel{UpdatedAt: m.now(), Days: m.days, Recent: m.recent}
	m.mu.Unlock()

	payload, err := json.Marshal(model)
	if err != nil {
		return fmt.Errorf("序列化监控统计: %w", err)
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("创建统计目录: %w", err)
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return fmt.Errorf("写入统计临时文件: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("替换统计文件: %w", err)
	}
	return nil
}

// Load 从文件恢复计数；文件不存在返回 nil 不视为错误
func (m *Metrics) Load(path string) error {
	if m == nil {
		return nil
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取监控统计: %w", err)
	}
	var model persistModel
	if err := json.Unmarshal(payload, &model); err != nil {
		return fmt.Errorf("解析监控统计 %s: %w", path, err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if model.Days == nil {
		model.Days = map[string]map[string]int64{}
	}
	if len(model.Recent) > recentCap {
		model.Recent = model.Recent[:recentCap]
	}
	m.days = model.Days
	m.recent = model.Recent
	m.pruneLocked()
	return nil
}

// pruneLocked 删除超出 7 天窗口（今日与之前 6 天）的旧桶
func (m *Metrics) pruneLocked() {
	cutoff := m.now().AddDate(0, 0, -(retainedDays - 1)).Format("2006-01-02")
	for date := range m.days {
		if date < cutoff {
			delete(m.days, date)
		}
	}
}

func (m *Metrics) sortedDaysLocked() []string {
	days := make([]string, 0, len(m.days))
	for date := range m.days {
		days = append(days, date)
	}
	sort.Strings(days)
	return days
}

type persistModel struct {
	UpdatedAt time.Time                   `json:"updated_at"`
	Days      map[string]map[string]int64 `json:"days"`
	Recent    []LoginRecord               `json:"recent,omitempty"`
}
