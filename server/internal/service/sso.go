package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jerion/wecom-auth-center/server/internal/store"
)

// SSO 负责 state 与 ticket 的生成和一次性消费。
type SSO struct {
	store     store.Store
	stateTTL  time.Duration
	ticketTTL time.Duration
}

func NewSSO(st store.Store, stateTTL, ticketTTL time.Duration) *SSO {
	return &SSO{store: st, stateTTL: stateTTL, ticketTTL: ticketTTL}
}

// NewState 生成 128 位随机 state 并登记来源。
func (s *SSO) NewState(ctx context.Context, rec store.StateRecord) (string, error) {
	state, err := randomToken()
	if err != nil {
		return "", fmt.Errorf("生成 state: %w", err)
	}
	if err := s.store.SaveState(ctx, state, rec, s.stateTTL); err != nil {
		return "", fmt.Errorf("保存 state: %w", err)
	}
	return state, nil
}

// ConsumeState 消费 state：不存在、过期或已用过均返回 false（取出即删，防重放）。
func (s *SSO) ConsumeState(ctx context.Context, state string) (store.StateRecord, bool) {
	return s.store.TakeState(ctx, state)
}

// NewTicket 生成 128 位随机 ticket 并登记身份与去向。
func (s *SSO) NewTicket(ctx context.Context, rec store.TicketRecord) (string, error) {
	ticket, err := randomToken()
	if err != nil {
		return "", fmt.Errorf("生成 ticket: %w", err)
	}
	if err := s.store.SaveTicket(ctx, ticket, rec, s.ticketTTL); err != nil {
		return "", fmt.Errorf("保存 ticket: %w", err)
	}
	return ticket, nil
}

// ConsumeTicket 消费 ticket：一次性，验证通过后立即作废。
func (s *SSO) ConsumeTicket(ctx context.Context, ticket string) (store.TicketRecord, bool) {
	return s.store.TakeTicket(ctx, ticket)
}

func randomToken() (string, error) {
	b := make([]byte, 16) // 128 位
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
