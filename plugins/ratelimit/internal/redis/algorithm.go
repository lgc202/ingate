package redis

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	"github.com/lgc202/ingate/plugins/internal/redisresp"
)

const (
	maxWindowSeconds  = int64(^uint64(0)>>1) / int64(time.Second)
	tokenBucketScript = `
local capacity = tonumber(ARGV[1])
local requests = tonumber(ARGV[2])
local window = tonumber(ARGV[3])
local redis_time = redis.call("TIME")
local now = redis_time[1] * 1000 + math.floor(redis_time[2] / 1000)
local tokens = tonumber(redis.call("HGET", KEYS[1], "tokens"))
local last = tonumber(redis.call("HGET", KEYS[1], "last"))
if tokens == nil then
  tokens = capacity
end
if last == nil then
  last = now
end
now = math.max(now, last)
local delta = math.max(0, now - last)
local refill = delta * requests / window
tokens = math.min(capacity, tokens + refill)
local allowed = 0
if tokens >= 1 then
  tokens = tokens - 1
  allowed = 1
end
redis.call("HSET", KEYS[1], "tokens", tokens, "last", now)
local ttl = math.max(window * 2, math.ceil(capacity * window / requests) * 2)
redis.call("PEXPIRE", KEYS[1], ttl)
local current = capacity - math.floor(tokens)
local reset = math.ceil((capacity - tokens) * window / requests)
return {allowed, current, capacity, math.floor(tokens), reset}
`
)

// TokenBucket 表示一条可以交给系统 Redis 执行的令牌桶检查
type TokenBucket struct {
	key      string
	requests int
	window   time.Duration
	capacity int
}

// BucketState 表示系统 Redis 返回的令牌桶状态
type BucketState struct {
	Allowed      bool
	Limit        int
	Remaining    int
	ResetSeconds int
}

// NewTokenBucket 根据策略额度构造一条 Redis 令牌桶检查
func NewTokenBucket(key string, quota config.Quota) (TokenBucket, error) {
	if key == "" {
		return TokenBucket{}, errors.New("redis key is required")
	}
	if quota.Requests <= 0 {
		return TokenBucket{}, errors.New("rate limit requests must be greater than zero")
	}
	windowSeconds := int64(quota.WindowSeconds)
	if windowSeconds <= 0 || windowSeconds > maxWindowSeconds {
		return TokenBucket{}, fmt.Errorf("rate limit window seconds %d is out of range", windowSeconds)
	}
	capacity := quota.Burst
	if capacity == 0 {
		capacity = quota.Requests
	}
	if capacity <= 0 {
		return TokenBucket{}, errors.New("rate limit capacity must be greater than zero")
	}
	return TokenBucket{
		key:      key,
		requests: quota.Requests,
		window:   time.Duration(windowSeconds) * time.Second,
		capacity: capacity,
	}, nil
}

// Command 生成可直接传给 Redis ABI 的 EVAL 命令
func (b TokenBucket) Command() ([]byte, error) {
	return evalCommand(
		tokenBucketScript,
		b.key,
		strconv.Itoa(b.capacity),
		strconv.Itoa(b.requests),
		strconv.FormatInt(b.window.Milliseconds(), 10),
	)
}

// ParseBucketState 将 Redis Lua 返回值转换为稳定的令牌桶状态
func ParseBucketState(response []byte) (BucketState, error) {
	values, err := redisresp.DecodeIntegers(response)
	if err != nil {
		return BucketState{}, fmt.Errorf("decode rate limit response: %w", err)
	}

	if len(values) != 5 {
		return BucketState{}, fmt.Errorf("token bucket returned %d values, want 5", len(values))
	}
	reset, err := resetSeconds(values[4])
	if err != nil {
		return BucketState{}, err
	}
	return newBucketState(values[0] == 1, values[1], values[2], reset)
}

func evalCommand(script, key string, args ...string) ([]byte, error) {
	parts := make([][]byte, 0, 4+len(args))
	parts = append(parts, []byte("EVAL"), []byte(script), []byte("1"), []byte(key))
	for _, arg := range args {
		parts = append(parts, []byte(arg))
	}
	return redisresp.EncodeCommand(parts...)
}

func newBucketState(allowed bool, current, limit int64, reset int) (BucketState, error) {
	if current < 0 || limit < 0 {
		return BucketState{}, errors.New("rate limit response contains a negative counter")
	}
	remaining := int64(0)
	if current < limit {
		remaining = limit - current
	}
	limitInt, err := checkedInt(limit)
	if err != nil {
		return BucketState{}, fmt.Errorf("rate limit limit: %w", err)
	}
	remainingInt, err := checkedInt(remaining)
	if err != nil {
		return BucketState{}, fmt.Errorf("rate limit remaining: %w", err)
	}
	return BucketState{
		Allowed:      allowed,
		Limit:        limitInt,
		Remaining:    remainingInt,
		ResetSeconds: reset,
	}, nil
}

func resetSeconds(milliseconds int64) (int, error) {
	if milliseconds <= 0 {
		return 0, nil
	}
	seconds := milliseconds / int64(time.Second/time.Millisecond)
	if milliseconds%int64(time.Second/time.Millisecond) != 0 {
		seconds++
	}
	return checkedInt(seconds)
}

func checkedInt(value int64) (int, error) {
	if value < int64(minInt()) || value > int64(maxInt()) {
		return 0, fmt.Errorf("value %d overflows int", value)
	}
	return int(value), nil
}

func maxInt() int {
	return int(^uint(0) >> 1)
}

func minInt() int {
	return -maxInt() - 1
}
