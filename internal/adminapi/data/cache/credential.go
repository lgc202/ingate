// Package cache 管理 Admin API 在 Redis 中的可重建执行数据
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	sharedaccesskey "github.com/lgc202/ingate/internal/pkg/accesskey"
)

// Credential 是发布给 AI 数据面的最小访问密钥执行配置
type Credential struct {
	ID            string
	SecretHash    [32]byte
	Enabled       bool
	AllowedModels []string
	ExpiresAt     *time.Time
}

// CredentialIndex 管理 Redis 中可执行的访问密钥视图
type CredentialIndex struct {
	redis *redis.Client
}

// NewCredentialIndex 创建访问密钥执行索引
func NewCredentialIndex(client *redis.Client) *CredentialIndex {
	return &CredentialIndex{redis: client}
}

// Reconcile 用 MySQL 当前事实重建 Redis 访问密钥索引
func (i *CredentialIndex) Reconcile(ctx context.Context, credentials []Credential) error {
	currentKeys, err := i.redis.SMembers(ctx, sharedaccesskey.CredentialSetKey).Result()
	if err != nil {
		return fmt.Errorf("list published access keys: %w", err)
	}
	_, err = i.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, key := range currentKeys {
			pipe.Del(ctx, key)
		}
		pipe.Del(ctx, sharedaccesskey.CredentialSetKey)
		for _, credential := range credentials {
			if credential.active() {
				if err := publish(pipe, ctx, credential); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("reconcile access key index: %w", err)
	}
	return nil
}

// Save 发布当前记录；停用或过期记录会从数据面删除
func (i *CredentialIndex) Save(ctx context.Context, credential Credential) error {
	key := sharedaccesskey.CredentialKey(credential.SecretHash)
	_, err := i.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		if credential.active() {
			if err := publish(pipe, ctx, credential); err != nil {
				return err
			}
		} else {
			pipe.Del(ctx, key)
			pipe.SRem(ctx, sharedaccesskey.CredentialSetKey, key)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("save access key index: %w", err)
	}
	return nil
}

// Delete 撤销访问密钥并删除它的最后使用时间
func (i *CredentialIndex) Delete(ctx context.Context, credential Credential) error {
	key := sharedaccesskey.CredentialKey(credential.SecretHash)
	_, err := i.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(ctx, key)
		pipe.SRem(ctx, sharedaccesskey.CredentialSetKey, key)
		pipe.HDel(ctx, sharedaccesskey.LastUsedKey, credential.ID)
		return nil
	})
	if err != nil {
		return fmt.Errorf("delete access key index: %w", err)
	}
	return nil
}

// LastUsed 返回数据面记录的最后认证时间
func (i *CredentialIndex) LastUsed(ctx context.Context, accessKeyIDs []string) (map[string]time.Time, error) {
	if len(accessKeyIDs) == 0 {
		return map[string]time.Time{}, nil
	}
	fields := make([]string, len(accessKeyIDs))
	copy(fields, accessKeyIDs)
	values, err := i.redis.HMGet(ctx, sharedaccesskey.LastUsedKey, fields...).Result()
	if err != nil {
		return nil, fmt.Errorf("read access key last used time: %w", err)
	}
	lastUsed := make(map[string]time.Time, len(values))
	for index, value := range values {
		if value == nil {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("access key %q last used time has type %T", accessKeyIDs[index], value)
		}
		seconds, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse access key %q last used time: %w", accessKeyIDs[index], err)
		}
		lastUsed[accessKeyIDs[index]] = time.Unix(seconds, 0).UTC()
	}
	return lastUsed, nil
}

func publish(pipe redis.Pipeliner, ctx context.Context, credential Credential) error {
	models, err := json.Marshal(credential.AllowedModels)
	if err != nil {
		return fmt.Errorf("encode access key %q model scope: %w", credential.ID, err)
	}
	expiresAt := ""
	if credential.ExpiresAt != nil {
		expiresAt = strconv.FormatInt(credential.ExpiresAt.Unix(), 10)
	}
	key := sharedaccesskey.CredentialKey(credential.SecretHash)
	pipe.HSet(ctx, key, map[string]any{
		sharedaccesskey.FieldID:            credential.ID,
		sharedaccesskey.FieldAllowedModels: string(models),
		sharedaccesskey.FieldExpiresAt:     expiresAt,
	})
	pipe.SAdd(ctx, sharedaccesskey.CredentialSetKey, key)
	return nil
}

func (c Credential) active() bool {
	return c.Enabled && (c.ExpiresAt == nil || c.ExpiresAt.After(time.Now()))
}
