package store

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis 分布式存储实现：SET EX 写入、GETDEL 原子取出即删，多实例部署时共享 state/ticket。
// 取值失败按不存在处理（fail closed），并记录 Warn 日志。
type Redis struct {
	client *redis.Client
	now    func() time.Time
	log    *slog.Logger
}

const redisKeyPrefix = "wecom-auth-center:"

// NewRedis 创建 Redis 存储客户端。
func NewRedis(addr, password string, db int, log *slog.Logger, now func() time.Time) *Redis {
	if now == nil {
		now = time.Now
	}
	if log == nil {
		log = slog.Default()
	}
	return &Redis{
		client: redis.NewClient(&redis.Options{Addr: addr, Password: password, DB: db}),
		now:    now,
		log:    log,
	}
}

// Ping 验证 Redis 连通性，供启动时快速失败。
func (s *Redis) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return s.client.Ping(ctx).Err()
}

// Close 释放底层连接。
func (s *Redis) Close() error {
	return s.client.Close()
}

// SaveState 以 SET EX 写入 state，ttl 到期由 Redis 自动清除。
func (s *Redis) SaveState(ctx context.Context, state string, rec StateRecord, ttl time.Duration) error {
	return s.save(ctx, redisKeyPrefix+"state:"+state, rec, ttl)
}

// TakeState 以 GETDEL 原子取出并删除 state，天然防重放。
func (s *Redis) TakeState(ctx context.Context, state string) (StateRecord, bool) {
	var rec StateRecord
	ok := s.take(ctx, redisKeyPrefix+"state:"+state, &rec)
	return rec, ok
}

// SaveTicket 以 SET EX 写入 ticket。
func (s *Redis) SaveTicket(ctx context.Context, ticket string, rec TicketRecord, ttl time.Duration) error {
	return s.save(ctx, redisKeyPrefix+"ticket:"+ticket, rec, ttl)
}

// TakeTicket 以 GETDEL 原子取出并删除 ticket。
func (s *Redis) TakeTicket(ctx context.Context, ticket string) (TicketRecord, bool) {
	var rec TicketRecord
	ok := s.take(ctx, redisKeyPrefix+"ticket:"+ticket, &rec)
	return rec, ok
}

func (s *Redis) save(ctx context.Context, key string, rec any, ttl time.Duration) error {
	payload, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, payload, ttl).Err()
}

func (s *Redis) take(ctx context.Context, key string, rec any) bool {
	payload, err := s.client.GetDel(ctx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			s.log.Warn("Redis 读取失败，按凭证不存在处理", "key", key, "error", err)
		}
		return false
	}
	if err := json.Unmarshal(payload, rec); err != nil {
		s.log.Warn("Redis 凭证数据损坏，按不存在处理", "key", key, "error", err)
		return false
	}
	return true
}
