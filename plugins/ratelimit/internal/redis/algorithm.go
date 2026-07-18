package redis

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

const tokenBucketScript = `
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

// Request 表示一次 Redis 令牌桶检查
type Request struct {
	Key      string
	Requests int
	Window   time.Duration
	Capacity int
}

// Result 表示一次令牌桶检查结果
type Result struct {
	Allowed      bool
	Current      int
	Limit        int
	Remaining    int
	ResetSeconds int
}

// BuildCommand 生成可直接传给 Redis ABI 的 EVAL 命令
func BuildCommand(request Request) ([]byte, error) {
	if err := validateRequest(request); err != nil {
		return nil, err
	}

	return evalCommand(
		tokenBucketScript,
		request.Key,
		strconv.Itoa(request.Capacity),
		strconv.Itoa(request.Requests),
		strconv.FormatInt(request.Window.Milliseconds(), 10),
	)
}

// ParseResult 将 Redis Lua 返回值转换为稳定限流结果
func ParseResult(request Request, response []byte) (Result, error) {
	if err := validateRequest(request); err != nil {
		return Result{}, err
	}
	values, err := DecodeIntegers(response)
	if err != nil {
		return Result{}, fmt.Errorf("decode rate limit response: %w", err)
	}

	if len(values) != 5 {
		return Result{}, fmt.Errorf("token bucket returned %d values, want 5", len(values))
	}
	reset, err := resetSeconds(values[4])
	if err != nil {
		return Result{}, err
	}
	return newResult(values[0] == 1, values[1], values[2], reset)
}

func evalCommand(script, key string, args ...string) ([]byte, error) {
	parts := make([][]byte, 0, 4+len(args))
	parts = append(parts, []byte("EVAL"), []byte(script), []byte("1"), []byte(key))
	for _, arg := range args {
		parts = append(parts, []byte(arg))
	}
	return EncodeCommand(parts...)
}

func validateRequest(request Request) error {
	if request.Key == "" {
		return errors.New("redis key is required")
	}
	if request.Requests <= 0 {
		return errors.New("rate limit requests must be greater than zero")
	}
	if request.Window < time.Millisecond {
		return errors.New("rate limit window must be at least one millisecond")
	}
	if request.Capacity <= 0 {
		return errors.New("rate limit capacity must be greater than zero")
	}
	return nil
}

func newResult(allowed bool, current, limit int64, reset int) (Result, error) {
	if current < 0 || limit < 0 {
		return Result{}, errors.New("rate limit response contains a negative counter")
	}
	remaining := int64(0)
	if current < limit {
		remaining = limit - current
	}
	currentInt, err := checkedInt(current)
	if err != nil {
		return Result{}, fmt.Errorf("rate limit current: %w", err)
	}
	limitInt, err := checkedInt(limit)
	if err != nil {
		return Result{}, fmt.Errorf("rate limit limit: %w", err)
	}
	remainingInt, err := checkedInt(remaining)
	if err != nil {
		return Result{}, fmt.Errorf("rate limit remaining: %w", err)
	}
	return Result{
		Allowed:      allowed,
		Current:      currentInt,
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
