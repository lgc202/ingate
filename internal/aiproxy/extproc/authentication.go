package extproc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"

	sharedaccesskey "github.com/lgc202/ingate/internal/pkg/accesskey"
)

var accessKeyAuthenticationScript = redis.NewScript(`
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

type authenticator struct {
	redis *redis.Client
}

type grant struct {
	allowedModels map[string]struct{}
}

func newAuthenticator(client *redis.Client) *authenticator {
	return &authenticator{redis: client}
}

func (a *authenticator) authenticate(ctx context.Context, secret string) (grant, bool, error) {
	hash := sharedaccesskey.Hash(secret)
	values, err := accessKeyAuthenticationScript.Run(
		ctx,
		a.redis,
		[]string{sharedaccesskey.CredentialKey(hash), sharedaccesskey.LastUsedKey},
	).Slice()
	if err != nil {
		return grant{}, false, fmt.Errorf("run access key authentication script: %w", err)
	}
	if len(values) == 1 && values[0] == nil {
		return grant{}, false, nil
	}
	if len(values) != 2 {
		return grant{}, false, fmt.Errorf("access key authentication returned %d values", len(values))
	}

	id, err := redisString(values[0])
	if err != nil || id == "" {
		return grant{}, false, errors.New("access key authentication returned an invalid ID")
	}
	modelsJSON, err := redisString(values[1])
	if err != nil {
		return grant{}, false, errors.New("access key authentication returned an invalid model scope")
	}
	var models []string
	if err := json.Unmarshal([]byte(modelsJSON), &models); err != nil {
		return grant{}, false, fmt.Errorf("decode access key model scope: %w", err)
	}
	allowedModels := make(map[string]struct{}, len(models))
	for _, model := range models {
		if model == "" {
			return grant{}, false, errors.New("access key model scope contains an empty model")
		}
		allowedModels[model] = struct{}{}
	}
	return grant{allowedModels: allowedModels}, true, nil
}

func (g grant) allows(model string) bool {
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
