// Package redis 保存 AI ExtProc 的实时 Token 额度计数
package redis

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	redisclient "github.com/redis/go-redis/v9"

	"github.com/lgc202/ingate/internal/aiextproc/biz/tokenquota"
	"github.com/lgc202/ingate/internal/aiextproc/conf"
)

const quotaCounterRetention = 7 * 24 * time.Hour

var addTokensScript = redisclient.NewScript(`
local tokens = tonumber(ARGV[1])
for index, key in ipairs(KEYS) do
  redis.call('INCRBY', key, tokens)
  if redis.call('TTL', key) < 0 then
    redis.call('EXPIREAT', key, tonumber(ARGV[index + 1]))
  end
end
return 1
`)

// TokenCounter 使用 Redis 保存当前自然周期内的实时 Token 使用量
type TokenCounter struct {
	client           *redisclient.Client
	operationTimeout time.Duration

	ready   atomic.Bool
	started chan struct{}
	done    chan struct{}
	cancel  context.CancelFunc
}

// NewTokenCounter 创建 Redis Token 计数器
func NewTokenCounter(config *conf.Data_Redis) *TokenCounter {
	client := redisclient.NewClient(&redisclient.Options{
		Addr:         strings.TrimSpace(config.GetAddress()),
		Password:     config.GetPassword(),
		DB:           int(config.GetDatabase()),
		DialTimeout:  config.GetDialTimeout().AsDuration(),
		ReadTimeout:  config.GetOperationTimeout().AsDuration(),
		WriteTimeout: config.GetOperationTimeout().AsDuration(),
	})
	return &TokenCounter{
		client:           client,
		operationTimeout: config.GetOperationTimeout().AsDuration(),
		started:          make(chan struct{}),
		done:             make(chan struct{}),
	}
}

// Start 验证 Redis 可用后保持计数器生命周期
func (c *TokenCounter) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	close(c.started)
	defer close(c.done)
	defer cancel()
	defer c.ready.Store(false)
	defer func() {
		_ = c.client.Close()
	}()

	pingCtx, stop := context.WithTimeout(runCtx, c.operationTimeout)
	err := c.client.Ping(pingCtx).Err()
	stop()
	if err != nil {
		return fmt.Errorf("connect Redis token counter: %w", err)
	}
	c.ready.Store(true)
	<-runCtx.Done()
	return nil
}

// Stop 停止计数器并关闭 Redis 连接池
func (c *TokenCounter) Stop(ctx context.Context) error {
	select {
	case <-c.started:
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
	c.cancel()
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Ready 表示 Redis 首次连接已经建立且最近一次额度操作成功
func (c *TokenCounter) Ready() bool {
	return c.ready.Load()
}

// Read 批量读取一次调用命中的全部额度周期
func (c *TokenCounter) Read(ctx context.Context, buckets []tokenquota.Bucket) ([]int64, error) {
	keys := bucketKeys(buckets)
	operationCtx, cancel := context.WithTimeout(ctx, c.operationTimeout)
	values, err := c.client.MGet(operationCtx, keys...).Result()
	cancel()
	if err != nil {
		c.ready.Store(false)
		return nil, fmt.Errorf("read Redis token counters: %w", err)
	}
	c.ready.Store(true)
	used := make([]int64, len(values))
	for i, value := range values {
		if value == nil {
			continue
		}
		text, ok := value.(string)
		if !ok {
			c.ready.Store(false)
			return nil, fmt.Errorf("decode Redis token counter %q", keys[i])
		}
		used[i], err = strconv.ParseInt(text, 10, 64)
		if err != nil {
			c.ready.Store(false)
			return nil, fmt.Errorf("decode Redis token counter %q: %w", keys[i], err)
		}
	}
	return used, nil
}

// Add 原子累加一次调用命中的全部额度周期
func (c *TokenCounter) Add(ctx context.Context, buckets []tokenquota.Bucket, tokens int64) error {
	keys := bucketKeys(buckets)
	arguments := make([]any, 1, len(buckets)+1)
	arguments[0] = tokens
	for _, bucket := range buckets {
		arguments = append(arguments, bucket.End.Add(quotaCounterRetention).Unix())
	}
	operationCtx, cancel := context.WithTimeout(ctx, c.operationTimeout)
	err := addTokensScript.Run(operationCtx, c.client, keys, arguments...).Err()
	cancel()
	if err != nil {
		c.ready.Store(false)
		return fmt.Errorf("add Redis token counters: %w", err)
	}
	c.ready.Store(true)
	return nil
}

func bucketKeys(buckets []tokenquota.Bucket) []string {
	keys := make([]string, 0, len(buckets))
	for _, bucket := range buckets {
		// Caller ID 作为 Redis Cluster hash tag，保证一次 Lua 结算涉及的 Key 位于同一 slot
		keys = append(keys, fmt.Sprintf(
			"ingate:token-quota:{%s}:%s:%s:%d",
			bucket.CallerID,
			bucket.PolicyID,
			bucket.Period,
			bucket.Start.Unix(),
		))
	}
	return keys
}
