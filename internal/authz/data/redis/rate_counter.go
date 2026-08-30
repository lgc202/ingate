// Package redis 保存 Authz 的共享请求限流计数。
package redis

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	redisrate "github.com/go-redis/redis_rate/v10"
	redisclient "github.com/redis/go-redis/v9"

	"github.com/lgc202/ingate/internal/authz/biz/ratelimit"
	"github.com/lgc202/ingate/internal/authz/conf"
)

const readinessProbeInterval = 5 * time.Second

// RateCounter 使用 Redis GCRA 实现跨 Authz 实例共享的请求限流。
type RateCounter struct {
	client           *redisclient.Client
	limiter          *redisrate.Limiter
	operationTimeout time.Duration

	ready   atomic.Bool
	running atomic.Bool
	done    chan struct{}

	lifecycleMu sync.Mutex
	cancel      context.CancelFunc
	stopping    bool
}

// NewRateCounter 创建请求限流计数器。
func NewRateCounter(config *conf.Data_Redis) *RateCounter {
	client := redisclient.NewClient(&redisclient.Options{
		Addr:         strings.TrimSpace(config.GetAddress()),
		Password:     config.GetPassword(),
		DB:           int(config.GetDatabase()),
		DialTimeout:  config.GetDialTimeout().AsDuration(),
		ReadTimeout:  config.GetOperationTimeout().AsDuration(),
		WriteTimeout: config.GetOperationTimeout().AsDuration(),
	})
	return &RateCounter{
		client:           client,
		limiter:          redisrate.NewLimiter(client),
		operationTimeout: config.GetOperationTimeout().AsDuration(),
		done:             make(chan struct{}),
	}
}

// Start 验证 Redis 可用后持续探测连接状态。
func (c *RateCounter) Start(ctx context.Context) error {
	if !c.running.CompareAndSwap(false, true) {
		return errors.New("redis rate counter is already running")
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer close(c.done)
	defer cancel()
	c.lifecycleMu.Lock()
	c.cancel = cancel
	stopping := c.stopping
	c.lifecycleMu.Unlock()
	if stopping {
		cancel()
	}
	defer c.ready.Store(false)
	defer func() {
		_ = c.client.Close()
	}()

	if err := c.ping(runCtx); err != nil {
		return fmt.Errorf("connect Redis rate counter: %w", err)
	}
	c.ready.Store(true)

	ticker := time.NewTicker(readinessProbeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-runCtx.Done():
			return nil
		case <-ticker.C:
			c.ready.Store(c.ping(runCtx) == nil)
		}
	}
}

// Stop 停止计数器并关闭 Redis 连接池。
func (c *RateCounter) Stop(ctx context.Context) error {
	c.lifecycleMu.Lock()
	c.stopping = true
	cancel := c.cancel
	c.lifecycleMu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop Redis rate counter: %w", ctx.Err())
	}
}

// Ready 表示 Redis 首次连接以及最近一次探测或计数操作均成功。
func (c *RateCounter) Ready() bool {
	return c.ready.Load()
}

// Allow 原子检查并消费一次请求额度。
func (c *RateCounter) Allow(ctx context.Context, limit ratelimit.Limit) (ratelimit.Decision, error) {
	operationCtx, cancel := context.WithTimeout(ctx, c.operationTimeout)
	defer cancel()
	result, err := c.limiter.Allow(operationCtx, limitKey(limit), redisrate.Limit{
		Rate:   limit.Requests,
		Burst:  limit.Requests,
		Period: limit.Period,
	})
	if err != nil {
		// 单个客户端请求取消不代表 Redis 不可用，不能让正常取消污染进程就绪状态。
		if ctx.Err() == nil {
			c.ready.Store(false)
		}
		return ratelimit.Decision{}, fmt.Errorf("execute Redis rate counter: %w", err)
	}
	if result.Allowed != 0 && result.Allowed != 1 {
		c.ready.Store(false)
		return ratelimit.Decision{}, fmt.Errorf("decode Redis rate counter allowed value %d", result.Allowed)
	}
	if result.Allowed == 0 && result.RetryAfter <= 0 {
		c.ready.Store(false)
		return ratelimit.Decision{}, fmt.Errorf("decode Redis rate counter retry delay %s", result.RetryAfter)
	}
	c.ready.Store(true)
	return ratelimit.Decision{
		Allowed:    result.Allowed == 1,
		RetryAfter: result.RetryAfter,
	}, nil
}

func (c *RateCounter) ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, c.operationTimeout)
	defer cancel()
	return c.client.Ping(pingCtx).Err()
}

func limitKey(limit ratelimit.Limit) string {
	scope := sha256.Sum256([]byte(limit.Scope))
	subject := sha256.Sum256([]byte(limit.Subject))
	return fmt.Sprintf(
		"ingate:request-rate:{%s}:%x:%x",
		limit.PolicyID,
		scope[:8],
		subject[:8],
	)
}
