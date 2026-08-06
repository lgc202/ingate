// Package accesskey 编解码 AI Proxy 与系统 Redis 之间的访问密钥认证协议
package accesskey

import (
	"encoding/json"
	"errors"
	"fmt"

	sharedaccesskey "github.com/lgc202/ingate/internal/pkg/accesskey"
	"github.com/lgc202/ingate/plugins/internal/redisresp"
)

const authenticateScript = `
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
`

// Grant 是 Redis 认证成功后返回给 AI 请求处理链路的模型权限
type Grant struct {
	allowedModels map[string]struct{}
}

// Command 构建访问密钥认证和最后使用时间更新命令
func Command(secret string) ([]byte, error) {
	hash := sharedaccesskey.Hash(secret)
	return redisresp.EncodeCommand(
		[]byte("EVAL"),
		[]byte(authenticateScript),
		[]byte("2"),
		[]byte(sharedaccesskey.CredentialKey(hash)),
		[]byte(sharedaccesskey.LastUsedKey),
	)
}

// Decode 解析认证结果；不存在、停用和过期的密钥都表现为未授权
func Decode(data []byte) (Grant, bool, error) {
	value, err := redisresp.Decode(data)
	if err != nil {
		return Grant{}, false, err
	}
	if value.Kind != redisresp.KindArray || len(value.Values) == 0 {
		return Grant{}, false, errors.New("access key Redis response must be a non-empty array")
	}
	if value.Values[0].Kind == redisresp.KindNull {
		return Grant{}, false, nil
	}
	if len(value.Values) != 2 {
		return Grant{}, false, fmt.Errorf("access key Redis response has %d values", len(value.Values))
	}
	id, err := stringValue(value.Values[0])
	if err != nil || id == "" {
		return Grant{}, false, errors.New("access key Redis response has invalid ID")
	}
	modelsJSON, err := stringValue(value.Values[1])
	if err != nil {
		return Grant{}, false, errors.New("access key Redis response has invalid model scope")
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

// Allows 判断访问密钥是否允许公开模型；空范围表示允许全部公开模型
func (g Grant) Allows(model string) bool {
	if len(g.allowedModels) == 0 {
		return true
	}
	_, exists := g.allowedModels[model]
	return exists
}

func stringValue(value redisresp.Value) (string, error) {
	if value.Kind != redisresp.KindBulkString && value.Kind != redisresp.KindSimpleString {
		return "", fmt.Errorf("Redis value kind is %d", value.Kind)
	}
	return string(value.Bytes), nil
}
