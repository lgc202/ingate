package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lgc202/ingate/internal/adminapi/domain"
	"github.com/lgc202/ingate/internal/adminapi/repository/redis"
)

// ErrSessionNotFound 表示登录态不存在或已过期。
var ErrSessionNotFound = errors.New("session not found")

// Session 提供登录态的持久化操作。
type Session struct {
	store *redis.Store
}

// NewSession 创建 Session Repository。
func NewSession(store *redis.Store) *Session {
	return &Session{store: store}
}

// Create 创建登录态。
func (r *Session) Create(ctx context.Context, id string, session domain.Session, expireTime time.Time) error {
	return r.store.CreateSession(ctx, id, redis.Session{
		AccountID:      session.AccountID,
		AccountVersion: session.AccountVersion,
	}, expireTime)
}

// Get 读取登录态。
func (r *Session) Get(ctx context.Context, id string) (domain.Session, error) {
	session, err := r.store.GetSession(ctx, id)
	if err != nil {
		if errors.Is(err, redis.ErrNotFound) {
			return domain.Session{}, fmt.Errorf("%w: %w", ErrSessionNotFound, err)
		}

		return domain.Session{}, err
	}

	return domain.Session{
		AccountID:      session.AccountID,
		AccountVersion: session.AccountVersion,
	}, nil
}

// Delete 删除登录态。
func (r *Session) Delete(ctx context.Context, id string) error {
	return r.store.DeleteSession(ctx, id)
}
