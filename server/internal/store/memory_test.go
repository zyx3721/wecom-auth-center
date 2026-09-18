package store

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestMemory() (*Memory, *time.Time) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return NewMemory(func() time.Time { return base }), &base
}

func TestTakeStateRemovesEntry(t *testing.T) {
	m, _ := newTestMemory()
	ctx := context.Background()

	if err := m.SaveState(ctx, "s1", StateRecord{App: "oa"}, time.Minute); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	rec, ok := m.TakeState(ctx, "s1")
	if !ok || rec.App != "oa" {
		t.Fatalf("首次取出应成功: rec=%v ok=%v", rec, ok)
	}
	if _, ok := m.TakeState(ctx, "s1"); ok {
		t.Fatal("state 为一次性，第二次取出应失败")
	}
}

func TestStateExpiry(t *testing.T) {
	m, clock := newTestMemory()
	ctx := context.Background()

	_ = m.SaveState(ctx, "s1", StateRecord{App: "oa"}, 5*time.Minute)
	*clock = clock.Add(5 * time.Minute) // 恰好到期
	if _, ok := m.TakeState(ctx, "s1"); ok {
		t.Fatal("过期 state 不应取出成功")
	}
}

func TestTakeTicketOnce(t *testing.T) {
	m, _ := newTestMemory()
	ctx := context.Background()

	_ = m.SaveTicket(ctx, "t1", TicketRecord{App: "oa", Userid: "zhangsan"}, time.Minute)
	rec, ok := m.TakeTicket(ctx, "t1")
	if !ok || rec.Userid != "zhangsan" {
		t.Fatalf("首次取出应成功: rec=%v ok=%v", rec, ok)
	}
	if _, ok := m.TakeTicket(ctx, "t1"); ok {
		t.Fatal("ticket 重放应失败")
	}
}

func TestConcurrentTakeOnlyOneWins(t *testing.T) {
	m, _ := newTestMemory()
	ctx := context.Background()
	_ = m.SaveTicket(ctx, "t1", TicketRecord{App: "oa"}, time.Minute)

	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := m.TakeTicket(ctx, "t1"); ok {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("并发消费应恰好成功一次，实际 %d", wins.Load())
	}
}
