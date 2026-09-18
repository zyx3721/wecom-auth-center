package store

import (
	"context"
	"sync"
	"time"
)

// Memory 单实例内存实现：互斥锁 + 惰性过期。
// 重启会丢失进行中的登录流程，用户重扫即可；多实例部署时切换 Redis 实现。
type Memory struct {
	mu      sync.Mutex
	states  map[string]memoryEntry[StateRecord]
	tickets map[string]memoryEntry[TicketRecord]
	now     func() time.Time
}

type memoryEntry[T any] struct {
	rec      T
	expireAt time.Time
}

func NewMemory(now func() time.Time) *Memory {
	if now == nil {
		now = time.Now
	}
	return &Memory{
		states:  make(map[string]memoryEntry[StateRecord]),
		tickets: make(map[string]memoryEntry[TicketRecord]),
		now:     now,
	}
}

func (m *Memory) SaveState(_ context.Context, state string, rec StateRecord, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[state] = memoryEntry[StateRecord]{rec: rec, expireAt: m.now().Add(ttl)}
	return nil
}

func (m *Memory) TakeState(_ context.Context, state string) (StateRecord, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return take(m.states, state, m.now)
}

func (m *Memory) SaveTicket(_ context.Context, ticket string, rec TicketRecord, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tickets[ticket] = memoryEntry[TicketRecord]{rec: rec, expireAt: m.now().Add(ttl)}
	return nil
}

func (m *Memory) TakeTicket(_ context.Context, ticket string) (TicketRecord, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return take(m.tickets, ticket, m.now)
}

// take 取出即删；过期条目同样删除并视为不存在。
func take[T any](m map[string]memoryEntry[T], key string, now func() time.Time) (T, bool) {
	e, ok := m[key]
	var zero T
	if !ok {
		return zero, false
	}
	delete(m, key)
	if !now().Before(e.expireAt) {
		return zero, false
	}
	return e.rec, true
}
