package ratelimit

import (
	"context"
	"errors"
	"time"

	dataplaneratelimit "github.com/lgc202/ingate/pkg/dataplane/ratelimit"
)

const defaultCommandTimeout = 50 * time.Millisecond

func (s *Service) executeCheck(ctx context.Context, check dataplaneratelimit.Check) (dataplaneratelimit.Result, error) {
	if err := validateCheck(check); err != nil {
		return dataplaneratelimit.Result{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, commandTimeout(check))
	defer cancel()

	client, err := s.clients.Client(redisConfig(check.RedisStore))
	if err != nil {
		return dataplaneratelimit.Result{}, err
	}
	switch check.Algorithm {
	case "", dataplaneratelimit.AlgorithmFixedWindow:
		return fixedWindow(ctx, client, check)
	case dataplaneratelimit.AlgorithmSlidingWindow:
		return slidingWindow(ctx, client, check)
	case dataplaneratelimit.AlgorithmTokenBucket:
		return tokenBucket(ctx, client, check)
	default:
		return dataplaneratelimit.Result{}, errors.New("unsupported rate limit algorithm")
	}
}

func validateCheck(check dataplaneratelimit.Check) error {
	if check.RedisKey == "" {
		return errors.New("redis key is required")
	}
	if check.Limit.Requests <= 0 || check.Limit.WindowSeconds <= 0 {
		return errors.New("limit requests and windowSeconds must be greater than zero")
	}
	return nil
}

func commandTimeout(check dataplaneratelimit.Check) time.Duration {
	if check.TimeoutMillis > 0 {
		return time.Duration(check.TimeoutMillis) * time.Millisecond
	}
	if check.RedisStore.CommandTimeoutMillis > 0 {
		return time.Duration(check.RedisStore.CommandTimeoutMillis) * time.Millisecond
	}
	return defaultCommandTimeout
}
