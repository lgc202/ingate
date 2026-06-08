package ratelimit

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	dataplaneratelimit "github.com/lgc202/ingate/pkg/dataplane/ratelimit"
)

var (
	fixedWindowScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
local ttl = redis.call("PTTL", KEYS[1])
return {current, ttl}
`)

	slidingWindowScript = redis.NewScript(`
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
`)

	tokenBucketScript = redis.NewScript(`
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
`)
)

func fixedWindow(ctx context.Context, client redis.UniversalClient, check dataplaneratelimit.Check) (dataplaneratelimit.Result, error) {
	windowMillis := check.Limit.WindowSeconds * 1000
	values, err := intSlice(fixedWindowScript.Run(ctx, client, []string{check.RedisKey}, windowMillis).Result())
	if err != nil {
		return dataplaneratelimit.Result{}, fmt.Errorf("execute fixed window script: %w", err)
	}
	if len(values) < 2 {
		return dataplaneratelimit.Result{}, fmt.Errorf("fixed window script returned %d values", len(values))
	}

	current := values[0]
	reset := resetSeconds(values[1])
	return result(check, current <= check.Limit.Requests, current, check.Limit.Requests, reset), nil
}

func slidingWindow(ctx context.Context, client redis.UniversalClient, check dataplaneratelimit.Check) (dataplaneratelimit.Result, error) {
	windowMillis := check.Limit.WindowSeconds * 1000
	nowMillis := time.Now().UnixMilli()
	member := strconv.FormatInt(time.Now().UnixNano(), 10)
	values, err := intSlice(slidingWindowScript.Run(ctx, client, []string{check.RedisKey}, windowMillis, check.Limit.Requests, member, nowMillis).Result())
	if err != nil {
		return dataplaneratelimit.Result{}, fmt.Errorf("execute sliding window script: %w", err)
	}
	if len(values) < 3 {
		return dataplaneratelimit.Result{}, fmt.Errorf("sliding window script returned %d values", len(values))
	}

	allowed := values[0] == 1
	current := values[1]
	reset := resetSeconds(values[2])
	return result(check, allowed, current, check.Limit.Requests, reset), nil
}

func tokenBucket(ctx context.Context, client redis.UniversalClient, check dataplaneratelimit.Check) (dataplaneratelimit.Result, error) {
	windowMillis := check.Limit.WindowSeconds * 1000
	capacity := check.Limit.Requests + check.Limit.Burst
	values, err := intSlice(tokenBucketScript.Run(ctx, client, []string{check.RedisKey}, capacity, check.Limit.Requests, windowMillis, time.Now().UnixMilli()).Result())
	if err != nil {
		return dataplaneratelimit.Result{}, fmt.Errorf("execute token bucket script: %w", err)
	}
	if len(values) < 5 {
		return dataplaneratelimit.Result{}, fmt.Errorf("token bucket script returned %d values", len(values))
	}

	allowed := values[0] == 1
	current := values[1]
	limit := values[2]
	reset := int(math.Ceil(float64(values[4]) / 1000))
	return result(check, allowed, current, limit, reset), nil
}

func result(check dataplaneratelimit.Check, allowed bool, current, limit, reset int) dataplaneratelimit.Result {
	remaining := max(limit-current, 0)
	retryAfter := 0
	if !allowed {
		retryAfter = reset
	}
	return dataplaneratelimit.Result{
		PolicyName:        check.PolicyName,
		RuleName:          check.RuleName,
		Allowed:           allowed,
		Current:           current,
		Limit:             limit,
		Remaining:         remaining,
		ResetSeconds:      reset,
		RetryAfterSeconds: retryAfter,
	}
}

func resetSeconds(ttlMillis int) int {
	if ttlMillis <= 0 {
		return 0
	}
	return int(math.Ceil(float64(ttlMillis) / 1000))
}

func intSlice(value any, err error) ([]int, error) {
	if err != nil {
		return nil, err
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected redis script result %T", value)
	}
	result := make([]int, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case int64:
			result = append(result, int(v))
		case string:
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("parse redis script value %q: %w", v, err)
			}
			result = append(result, n)
		default:
			return nil, fmt.Errorf("unexpected redis script value %T", item)
		}
	}
	return result, nil
}
