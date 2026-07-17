package redis

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// Algorithm 表示 Redis 限流算法
type Algorithm string

const (
	// AlgorithmFixedWindow 表示固定窗口限流
	AlgorithmFixedWindow Algorithm = "FixedWindow"
	// AlgorithmSlidingWindow 表示滑动窗口限流
	AlgorithmSlidingWindow Algorithm = "SlidingWindow"
	// AlgorithmTokenBucket 表示令牌桶限流
	AlgorithmTokenBucket Algorithm = "TokenBucket"
)

const fixedWindowScript = `
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
local ttl = redis.call("PTTL", KEYS[1])
return {current, ttl}
`

const slidingWindowScript = `
local window = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local member = ARGV[3]
local now = tonumber(ARGV[4])
redis.call("ZREMRANGEBYSCORE", KEYS[1], 0, now - window)
local current = redis.call("ZCARD", KEYS[1])
local allowed = 0
if current < limit then
  redis.call("ZADD", KEYS[1], now, member)
  current = current + 1
  allowed = 1
end
redis.call("PEXPIRE", KEYS[1], window)
local ttl = redis.call("PTTL", KEYS[1])
return {allowed, current, ttl}
`

const tokenBucketScript = `
local capacity = tonumber(ARGV[1])
local requests = tonumber(ARGV[2])
local window = tonumber(ARGV[3])
local now = tonumber(ARGV[4])
local tokens = tonumber(redis.call("HGET", KEYS[1], "tokens"))
local last = tonumber(redis.call("HGET", KEYS[1], "last"))
if tokens == nil then
  tokens = capacity
end
if last == nil then
  last = now
end
local delta = math.max(0, now - last)
local refill = delta * requests / window
tokens = math.min(capacity, tokens + refill)
local allowed = 0
if tokens >= 1 then
  tokens = tokens - 1
  allowed = 1
end
redis.call("HSET", KEYS[1], "tokens", tokens, "last", now)
redis.call("PEXPIRE", KEYS[1], window * 2)
local current = capacity - math.floor(tokens)
local reset = 0
if allowed == 0 then
  reset = math.ceil((1 - tokens) * window / requests)
end
return {allowed, current, capacity, math.floor(tokens), reset}
`

// Request 表示一次 Redis 限流算法调用
type Request struct {
	Algorithm Algorithm
	Key       string
	Requests  int
	Window    time.Duration
	Burst     int
	Now       time.Time
	Member    string
}

// Result 表示一次限流算法结果
type Result struct {
	Allowed           bool
	Current           int
	Limit             int
	Remaining         int
	ResetSeconds      int
	RetryAfterSeconds int
}

// BuildCommand 生成可直接传给 Redis ABI 的 EVAL 命令
func BuildCommand(request Request) ([]byte, error) {
	if err := validateRequest(request); err != nil {
		return nil, err
	}

	windowMillis := request.Window.Milliseconds()
	switch normalizeAlgorithm(request.Algorithm) {
	case AlgorithmFixedWindow:
		return evalCommand(fixedWindowScript, request.Key, strconv.FormatInt(windowMillis, 10))
	case AlgorithmSlidingWindow:
		if request.Member == "" {
			return nil, errors.New("sliding window member is required")
		}
		return evalCommand(
			slidingWindowScript,
			request.Key,
			strconv.FormatInt(windowMillis, 10),
			strconv.Itoa(request.Requests),
			request.Member,
			strconv.FormatInt(request.Now.UnixMilli(), 10),
		)
	case AlgorithmTokenBucket:
		if request.Burst > maxInt()-request.Requests {
			return nil, errors.New("rate limit token bucket capacity overflows int")
		}
		return evalCommand(
			tokenBucketScript,
			request.Key,
			strconv.Itoa(request.Requests+request.Burst),
			strconv.Itoa(request.Requests),
			strconv.FormatInt(windowMillis, 10),
			strconv.FormatInt(request.Now.UnixMilli(), 10),
		)
	default:
		return nil, fmt.Errorf("unsupported rate limit algorithm %q", request.Algorithm)
	}
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

	switch normalizeAlgorithm(request.Algorithm) {
	case AlgorithmFixedWindow:
		if len(values) != 2 {
			return Result{}, fmt.Errorf("fixed window returned %d values, want 2", len(values))
		}
		reset, err := resetSeconds(values[1])
		if err != nil {
			return Result{}, err
		}
		return newResult(values[0] <= int64(request.Requests), values[0], int64(request.Requests), reset)
	case AlgorithmSlidingWindow:
		if len(values) != 3 {
			return Result{}, fmt.Errorf("sliding window returned %d values, want 3", len(values))
		}
		reset, err := resetSeconds(values[2])
		if err != nil {
			return Result{}, err
		}
		return newResult(values[0] == 1, values[1], int64(request.Requests), reset)
	case AlgorithmTokenBucket:
		if len(values) != 5 {
			return Result{}, fmt.Errorf("token bucket returned %d values, want 5", len(values))
		}
		reset, err := resetSeconds(values[4])
		if err != nil {
			return Result{}, err
		}
		return newResult(values[0] == 1, values[1], values[2], reset)
	default:
		return Result{}, fmt.Errorf("unsupported rate limit algorithm %q", request.Algorithm)
	}
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
	if request.Burst < 0 {
		return errors.New("rate limit burst must not be negative")
	}
	return nil
}

func normalizeAlgorithm(algorithm Algorithm) Algorithm {
	if algorithm == "" {
		return AlgorithmFixedWindow
	}
	return algorithm
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
	retryAfter := 0
	if !allowed {
		retryAfter = reset
	}
	return Result{
		Allowed:           allowed,
		Current:           currentInt,
		Limit:             limitInt,
		Remaining:         remainingInt,
		ResetSeconds:      reset,
		RetryAfterSeconds: retryAfter,
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
