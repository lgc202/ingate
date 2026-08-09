// Package accesskey 实现 AI 请求访问密钥的执行态认证
package accesskey

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"

	sharedaccesskey "github.com/lgc202/ingate/internal/pkg/accesskey"
)

var authenticationScript = redis.NewScript(`
local values = redis.call('HMGET', KEYS[1], 'id', 'allowed_models', 'expires_at')
if not values[1] then
  return {false}
end
local now = redis.call('TIME')
if values[3] and values[3] ~= '' and tonumber(values[3]) <= tonumber(now[1]) then
  return {false}
end
redis.call('HSET', KEYS[2], values[1], now[1])
return {values[1], values[2]}
`)

// Authenticator 使用 Admin API 发布到 Redis 的执行索引认证访问密钥
type Authenticator struct {
	rdb *redis.Client
}

// Grant 表示访问密钥允许访问的公开模型范围
type Grant struct {
	allowedModels map[string]struct{}
}

// NewAuthenticator 创建访问密钥认证器
func NewAuthenticator(rdb *redis.Client) *Authenticator {
	return &Authenticator{rdb: rdb}
}

// Authenticate 校验访问密钥并原子更新最近使用时间
func (a *Authenticator) Authenticate(ctx context.Context, secret string) (Grant, bool, error) {
	hash := sharedaccesskey.Hash(secret)
	values, err := authenticationScript.Run(
		ctx,
		a.rdb,
		[]string{sharedaccesskey.CredentialKey(hash), sharedaccesskey.LastUsedKey},
	).Slice()
	if err != nil {
		return Grant{}, false, fmt.Errorf("run access key authentication script: %w", err)
	}
	if len(values) == 1 && values[0] == nil {
		return Grant{}, false, nil
	}
	if len(values) != 2 {
		return Grant{}, false, fmt.Errorf("access key authentication returned %d values", len(values))
	}

	id, err := redisString(values[0])
	if err != nil || id == "" {
		return Grant{}, false, errors.New("access key authentication returned an invalid ID")
	}
	modelsJSON, err := redisString(values[1])
	if err != nil {
		return Grant{}, false, errors.New("access key authentication returned an invalid model scope")
	}
	var models []string
	if err := json.Unmarshal([]byte(modelsJSON), &models); err != nil {
		return Grant{}, false, fmt.Errorf("decode access key model scope: %w", err)
	}
	allowedModels := make(map[string]struct{}, len(models))
	for _, model := range models {
		if model == "" {
			return Grant{}, false, errors.New("access key model scope contains an empty model")
		}
		allowedModels[model] = struct{}{}
	}
	return Grant{allowedModels: allowedModels}, true, nil
}

// Allows 判断访问密钥是否允许访问公开模型，空范围表示允许全部模型
func (g Grant) Allows(model string) bool {
	if len(g.allowedModels) == 0 {
		return true
	}
	_, exists := g.allowedModels[model]
	return exists
}

func redisString(value any) (string, error) {
	switch value := value.(type) {
	case string:
		return value, nil
	case []byte:
		return string(value), nil
	default:
		return "", fmt.Errorf("Redis value has type %T", value)
	}
}
