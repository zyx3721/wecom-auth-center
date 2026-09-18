package store

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func newTestRedis(t *testing.T) (*Redis, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	s := NewRedis(mr.Addr(), "", 0, slog.Default(), time.Now)
	t.Cleanup(func() { _ = s.Close() })
	return s, mr
}

func TestRedisSaveTakeRoundtrip(t *testing.T) {
	s, _ := newTestRedis(t)
	ctx := context.Background()

	if err := s.SaveState(ctx, "st1", StateRecord{App: "oa", Redirect: "/console"}, time.Minute); err != nil {
		t.Fatalf("写入 state 失败: %v", err)
	}
	rec, ok := s.TakeState(ctx, "st1")
	if !ok || rec.App != "oa" || rec.Redirect != "/console" {
		t.Fatalf("state 往返结果不符: ok=%v rec=%+v", ok, rec)
	}

	if err := s.SaveTicket(ctx, "tk1", TicketRecord{App: "oa", Userid: "zhangsan", Name: "张三"}, time.Minute); err != nil {
		t.Fatalf("写入 ticket 失败: %v", err)
	}
	trec, ok := s.TakeTicket(ctx, "tk1")
	if !ok || trec.Userid != "zhangsan" || trec.Name != "张三" {
		t.Fatalf("ticket 往返结果不符: ok=%v rec=%+v", ok, trec)
	}
}

func TestRedisTakeDeletesReplayDenied(t *testing.T) {
	s, _ := newTestRedis(t)
	ctx := context.Background()

	_ = s.SaveState(ctx, "st1", StateRecord{App: "oa"}, time.Minute)
	if _, ok := s.TakeState(ctx, "st1"); !ok {
		t.Fatal("首次取出应成功")
	}
	if _, ok := s.TakeState(ctx, "st1"); ok {
		t.Fatal("取出即删，重放应被拒绝")
	}
}

func TestRedisExpiry(t *testing.T) {
	s, mr := newTestRedis(t)
	ctx := context.Background()

	_ = s.SaveTicket(ctx, "tk1", TicketRecord{App: "oa"}, 30*time.Second)
	mr.FastForward(31 * time.Second)
	if _, ok := s.TakeTicket(ctx, "tk1"); ok {
		t.Fatal("过期 ticket 不应可消费")
	}
}

func TestRedisDownFailsClosed(t *testing.T) {
	mr := miniredis.RunT(t)
	addr := mr.Addr()
	mr.Close()
	s := NewRedis(addr, "", 0, slog.Default(), time.Now)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	if err := s.SaveState(ctx, "st1", StateRecord{App: "oa"}, time.Minute); err == nil {
		t.Fatal("Redis 不可用时写入应返回错误")
	}
	if _, ok := s.TakeState(ctx, "st1"); ok {
		t.Fatal("Redis 不可用时取出应失败关闭")
	}
}
