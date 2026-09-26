package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const sessionKeyPrefix = "ingate:admin-api:session:v1:"

// ErrNotFound 表示登录态不存在或已经过期。
var ErrNotFound = errors.New("session not found")

// Session 是 Redis 中保存的登录态。
type Session struct {
	AccountID      uint64 `json:"account_id"`
	AccountVersion uint64 `json:"account_version"`
}

// CreateSession 将登录态及其绝对过期时间原子写入 Redis。
func (s *Store) CreateSession(ctx context.Context, id string, session Session, expireTime time.Time) error {
	value, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}

	if err := s.client.SetArgs(ctx, sessionKey(id), value, goredis.SetArgs{
		ExpireAt: expireTime,
	}).Err(); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	return nil
}

// GetSession 读取登录态。
func (s *Store) GetSession(ctx context.Context, id string) (Session, error) {
	value, err := s.client.Get(ctx, sessionKey(id)).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return Session{}, ErrNotFound
		}
		return Session{}, fmt.Errorf("get session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(value, &session); err != nil {
		return Session{}, fmt.Errorf("decode session: %w", err)
	}

	return session, nil
}

// DeleteSession 删除登录态；不存在时也视为成功。
func (s *Store) DeleteSession(ctx context.Context, id string) error {
	if err := s.client.Del(ctx, sessionKey(id)).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

func sessionKey(id string) string {
	return sessionKeyPrefix + id
}
