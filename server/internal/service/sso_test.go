package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jerion/wecom-auth-center/server/internal/store"
)

func TestStateRoundTrip(t *testing.T) {
	s := NewSSO(store.NewMemory(time.Now), 5*time.Minute, time.Minute)
	ctx := context.Background()

	state, err := s.NewState(ctx, store.StateRecord{App: "oa", Redirect: "/a"})
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	if len(state) != 32 { // 128 位 hex
		t.Fatalf("state 长度应为 32 hex 字符，实际 %d", len(state))
	}
	rec, ok := s.ConsumeState(ctx, state)
	if !ok || rec.App != "oa" || rec.Redirect != "/a" {
		t.Fatalf("消费 state 失败: %v %v", rec, ok)
	}
	if _, ok := s.ConsumeState(ctx, state); ok {
		t.Fatal("state 重放应失败")
	}
}

func TestStateRandom(t *testing.T) {
	s := NewSSO(store.NewMemory(time.Now), 5*time.Minute, time.Minute)
	ctx := context.Background()

	a, _ := s.NewState(ctx, store.StateRecord{App: "oa"})
	b, _ := s.NewState(ctx, store.StateRecord{App: "oa"})
	if a == b {
		t.Fatal("两次生成的 state 不应相同")
	}
}

func TestTicketRoundTrip(t *testing.T) {
	s := NewSSO(store.NewMemory(time.Now), 5*time.Minute, time.Minute)
	ctx := context.Background()

	ticket, err := s.NewTicket(ctx, store.TicketRecord{App: "oa", Userid: "zhangsan", Name: "张三"})
	if err != nil {
		t.Fatalf("NewTicket: %v", err)
	}
	rec, ok := s.ConsumeTicket(ctx, ticket)
	if !ok || rec.Userid != "zhangsan" || rec.Name != "张三" {
		t.Fatalf("消费 ticket 失败: %v %v", rec, ok)
	}
	if _, ok := s.ConsumeTicket(ctx, ticket); ok {
		t.Fatal("ticket 重放应失败")
	}
}

func TestRandomTokenFormat(t *testing.T) {
	tok, err := randomToken()
	if err != nil {
		t.Fatalf("randomToken: %v", err)
	}
	if len(tok) != 32 || strings.ToLower(tok) != tok {
		t.Fatalf("token 应为 32 位小写 hex，实际 %q", tok)
	}
}
