// Package redis 实现 Token 配额固定窗口检查和实际用量记账协议
package redis

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	config "github.com/lgc202/ingate/pkg/plugin/tokenquota"
	"github.com/lgc202/ingate/plugins/internal/redisresp"
)

const (
	maxWindowSeconds = int64(^uint64(0)>>1) / int64(time.Second)
	checkScript      = `
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local redis_time = redis.call("TIME")
local now = redis_time[1] * 1000 + math.floor(redis_time[2] / 1000)
local window_id = math.floor(now / window)
local key = KEYS[1] .. ":" .. window_id
local used = tonumber(redis.call("GET", key)) or 0
local allowed = 0
if used < limit then
  allowed = 1
end
local reset = math.max(1, math.ceil(((window_id + 1) * window - now) / 1000))
return {allowed, used, limit, reset}
`
	addScript = `
local window = tonumber(ARGV[1])
local amount = ARGV[2]
local redis_time = redis.call("TIME")
local now = redis_time[1] * 1000 + math.floor(redis_time[2] / 1000)
local window_id = math.floor(now / window)
local key = KEYS[1] .. ":" .. window_id
local used = redis.call("INCRBY", key, amount)
local ttl = math.max(1, (window_id + 1) * window - now)
redis.call("PEXPIRE", key, ttl)
return {used}
`
)

// Window 表示一个由 Redis 服务端时间驱动的固定配额窗口
type Window struct {
	key      string
	limit    int64
	duration time.Duration
}

// CheckState 表示 Redis 返回的当前窗口额度状态
type CheckState struct {
	Allowed      bool
	Used         int64
	Limit        int64
	ResetSeconds int64
}

// NewWindow 根据策略额度构造 Redis 固定窗口操作
func NewWindow(key string, quota config.Quota) (Window, error) {
	if key == "" {
		return Window{}, errors.New("redis key is required")
	}
	if quota.Tokens <= 0 {
		return Window{}, errors.New("token quota limit must be greater than zero")
	}
	if quota.WindowSeconds <= 0 || quota.WindowSeconds > maxWindowSeconds {
		return Window{}, fmt.Errorf("token quota window seconds %d is out of range", quota.WindowSeconds)
	}
	return Window{
		key:      key,
		limit:    quota.Tokens,
		duration: time.Duration(quota.WindowSeconds) * time.Second,
	}, nil
}

// CheckCommand 生成当前窗口已用额度检查命令
func (w Window) CheckCommand() ([]byte, error) {
	return evalCommand(
		checkScript,
		w.key,
		strconv.FormatInt(w.limit, 10),
		strconv.FormatInt(w.duration.Milliseconds(), 10),
	)
}

// AddCommand 生成把实际 Token 用量记入响应结束时所在窗口的命令
func (w Window) AddCommand(tokens int64) ([]byte, error) {
	if tokens <= 0 {
		return nil, errors.New("token usage must be greater than zero")
	}
	return evalCommand(
		addScript,
		w.key,
		strconv.FormatInt(w.duration.Milliseconds(), 10),
		strconv.FormatInt(tokens, 10),
	)
}

// ParseCheckState 解析 Redis Lua 返回的固定窗口状态
func ParseCheckState(response []byte) (CheckState, error) {
	values, err := redisresp.DecodeIntegers(response)
	if err != nil {
		return CheckState{}, fmt.Errorf("decode token quota check response: %w", err)
	}
	if len(values) != 4 {
		return CheckState{}, fmt.Errorf("token quota check returned %d values, want 4", len(values))
	}
	if values[1] < 0 || values[2] <= 0 || values[3] < 0 {
		return CheckState{}, errors.New("token quota check returned an invalid counter")
	}
	return CheckState{
		Allowed:      values[0] == 1,
		Used:         values[1],
		Limit:        values[2],
		ResetSeconds: values[3],
	}, nil
}

// ParseAddedUsage 校验 Redis Lua 返回的最新累计用量
func ParseAddedUsage(response []byte) (int64, error) {
	values, err := redisresp.DecodeIntegers(response)
	if err != nil {
		return 0, fmt.Errorf("decode token quota usage response: %w", err)
	}
	if len(values) != 1 || values[0] < 0 {
		return 0, errors.New("token quota usage returned an invalid counter")
	}
	return values[0], nil
}

func evalCommand(script, key string, args ...string) ([]byte, error) {
	parts := make([][]byte, 0, 4+len(args))
	parts = append(parts, []byte("EVAL"), []byte(script), []byte("1"), []byte(key))
	for _, arg := range args {
		parts = append(parts, []byte(arg))
	}
	return redisresp.EncodeCommand(parts...)
}
