package metrics

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jerion/wecom-auth-center/server/internal/store"
)

// clock 可拨动的假时钟
type clock struct{ current time.Time }

func (c *clock) Now() time.Time { return c.current }

func newTestMetrics(t *testing.T) (*Metrics, *clock) {
	t.Helper()
	c := &clock{current: time.Date(2026, 9, 19, 10, 0, 0, 0, time.Local)}
	return New(c.Now), c
}

func TestIncAndSnapshot(t *testing.T) {
	m, c := newTestMetrics(t)

	m.Inc("login_start")
	m.Inc("login_start")
	m.Inc("ticket_issue")
	c.current = c.current.Add(24 * time.Hour)
	m.Inc("login_start")
	m.Inc("verify_ok")

	snap := m.Snapshot()
	if len(snap.Days) != 2 {
		t.Fatalf("应有两个日桶，实际 %d", len(snap.Days))
	}
	if snap.Days[0].Date != "2026-09-19" || snap.Days[0].Counts["login_start"] != 2 {
		t.Errorf("首日计数不符: %+v", snap.Days[0])
	}
	if snap.Days[1].Date != "2026-09-20" || snap.Days[1].Counts["login_start"] != 1 {
		t.Errorf("次日计数不符: %+v", snap.Days[1])
	}
	if snap.Total["login_start"] != 3 || snap.Total["ticket_issue"] != 1 || snap.Total["verify_ok"] != 1 {
		t.Errorf("累计计数不符: %+v", snap.Total)
	}
	if snap.Today["login_start"] != 1 || snap.Today["verify_ok"] != 1 {
		t.Errorf("今日计数不符: %+v", snap.Today)
	}
}

func TestPruneToSevenDays(t *testing.T) {
	m, c := newTestMetrics(t)

	m.Inc("login_start")
	c.current = c.current.Add(10 * 24 * time.Hour)
	m.Inc("login_start")

	snap := m.Snapshot()
	if len(snap.Days) != 1 {
		t.Fatalf("窗口外的旧日期桶应被裁剪，实际保留 %d 天", len(snap.Days))
	}
	if snap.Total["login_start"] != 1 {
		t.Errorf("裁剪掉的旧日期不应计入累计: %+v", snap.Total)
	}

	c.current = c.current.Add(3 * 24 * time.Hour)
	m.Inc("login_start")
	if got := len(m.Snapshot().Days); got != 2 {
		t.Fatalf("窗口内的两个日期桶都应保留，实际 %d", got)
	}
}

func TestTodayFollowsCalendar(t *testing.T) {
	m, c := newTestMetrics(t)

	m.Inc("login_start")
	c.current = c.current.Add(24 * time.Hour)
	snap := m.Snapshot()
	if len(snap.Today) != 0 {
		t.Errorf("跨零点后未产生新事件，今日计数应为空: %+v", snap.Today)
	}
	if snap.Total["login_start"] != 1 {
		t.Errorf("近 7 天累计不应受零点影响: %+v", snap.Total)
	}

	m.Inc("login_start")
	if m.Snapshot().Today["login_start"] != 1 {
		t.Errorf("新一天的今日计数不符: %+v", m.Snapshot().Today)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	m, c := newTestMetrics(t)
	m.Inc("login_start")
	m.Inc("ticket_issue")
	c.current = c.current.Add(24 * time.Hour)
	m.Inc("verify_ok")

	path := filepath.Join(t.TempDir(), "status-metrics.json")
	if err := m.Save(path); err != nil {
		t.Fatalf("落盘失败: %v", err)
	}

	restored := New(c.Now)
	if err := restored.Load(path); err != nil {
		t.Fatalf("恢复失败: %v", err)
	}
	orig, restoredSnap := m.Snapshot(), restored.Snapshot()
	if len(orig.Days) != len(restoredSnap.Days) {
		t.Fatalf("恢复后日桶数不符: %d vs %d", len(orig.Days), len(restoredSnap.Days))
	}
	for i := range orig.Days {
		if orig.Days[i].Date != restoredSnap.Days[i].Date {
			t.Errorf("第 %d 天日期不符: %s vs %s", i, orig.Days[i].Date, restoredSnap.Days[i].Date)
		}
		for event, n := range orig.Days[i].Counts {
			if restoredSnap.Days[i].Counts[event] != n {
				t.Errorf("第 %d 天 %s 计数不符: %d vs %d", i, event, n, restoredSnap.Days[i].Counts[event])
			}
		}
	}
}

func TestLoadMissingFile(t *testing.T) {
	m, _ := newTestMetrics(t)
	if err := m.Load(filepath.Join(t.TempDir(), "nope.json")); err != nil {
		t.Fatalf("文件不存在不应报错: %v", err)
	}
}

func TestConcurrentInc(t *testing.T) {
	m, _ := newTestMetrics(t)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Inc("login_start")
		}()
	}
	wg.Wait()
	if got := m.Snapshot().Total["login_start"]; got != 50 {
		t.Fatalf("并发计数应恰为 50，实际 %d", got)
	}
}

func TestRecordLoginCapAndOrder(t *testing.T) {
	m, c := newTestMetrics(t)

	for i := 0; i < recentCap+5; i++ {
		m.RecordLogin(LoginRecord{Time: c.Now(), App: "oa", Userid: "user" + itoa(i), Remote: "10.0.0." + itoa(i%255)})
		c.current = c.current.Add(time.Second)
	}
	list := m.RecentLogins()
	if len(list) != recentCap {
		t.Fatalf("流水应保留最近 %d 条，实际 %d", recentCap, len(list))
	}
	if list[0].Userid != "user"+itoa(recentCap+4) {
		t.Errorf("最新记录应在最前: %+v", list[0])
	}
	if list[len(list)-1].Userid != "user5" {
		t.Errorf("最旧记录应为 user5: %+v", list[len(list)-1])
	}
}

func TestLoginPersistRoundtrip(t *testing.T) {
	m, _ := newTestMetrics(t)
	m.RecordLogin(LoginRecord{
		Time: time.Now(), App: "oa", Userid: "zhangsan", Name: "张三", Remote: "172.18.0.9",
		JobNumber:   "10001",
		Departments: []store.Department{{ID: 2, Name: "研发中心/研发部"}},
	})

	path := filepath.Join(t.TempDir(), "status-metrics.json")
	if err := m.Save(path); err != nil {
		t.Fatalf("落盘失败: %v", err)
	}
	restored := New(time.Now)
	if err := restored.Load(path); err != nil {
		t.Fatalf("恢复失败: %v", err)
	}
	list := restored.RecentLogins()
	if len(list) != 1 || list[0].Userid != "zhangsan" || list[0].Name != "张三" || list[0].Remote != "172.18.0.9" {
		t.Fatalf("登录流水恢复不符: %+v", list)
	}
	rec := list[0]
	if rec.JobNumber != "10001" ||
		len(rec.Departments) != 1 || rec.Departments[0].Name != "研发中心/研发部" {
		t.Fatalf("登录流水档案字段恢复不符: %+v", rec)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestNilReceiverSafe(t *testing.T) {
	var m *Metrics
	m.Inc("login_start")
	snap := m.Snapshot()
	if len(snap.Days) != 0 || len(snap.Today) != 0 {
		t.Errorf("nil 接收者应返回空快照: %+v", snap)
	}
	if err := m.Save(filepath.Join(t.TempDir(), "x.json")); err != nil {
		t.Errorf("nil 接收者 Save 不应报错: %v", err)
	}
	if err := m.Load(filepath.Join(t.TempDir(), "x.json")); err != nil {
		t.Errorf("nil 接收者 Load 不应报错: %v", err)
	}
}
