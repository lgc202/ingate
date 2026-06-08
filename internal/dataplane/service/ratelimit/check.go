package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"time"

	dataplaneratelimit "github.com/lgc202/ingate/pkg/dataplane/ratelimit"
)

const defaultCommandTimeout = 50 * time.Millisecond

var (
	errInvalidCheck         = errors.New("invalid rate limit check")
	errUnsupportedAlgorithm = errors.New("unsupported rate limit algorithm")
)

func (s *Service) executeCheck(ctx context.Context, check dataplaneratelimit.Check) (dataplaneratelimit.Result, error) {
	if err := validateCheck(check); err != nil {
		return dataplaneratelimit.Result{}, err
	}
	if err := validateAlgorithm(check.Algorithm); err != nil {
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
		return dataplaneratelimit.Result{}, errUnsupportedAlgorithm
	}
}

func validateCheck(check dataplaneratelimit.Check) error {
	if check.RedisKey == "" {
		return fmt.Errorf("%w: redis key is required", errInvalidCheck)
	}
	if check.RedisStore.ID == "" {
		return fmt.Errorf("%w: redis store id is required", errInvalidCheck)
	}
	if check.Limit.Requests <= 0 || check.Limit.WindowSeconds <= 0 {
		return fmt.Errorf("%w: limit requests and windowSeconds must be greater than zero", errInvalidCheck)
	}
	return nil
}

func validateAlgorithm(algorithm dataplaneratelimit.Algorithm) error {
	switch algorithm {
	case "", dataplaneratelimit.AlgorithmFixedWindow, dataplaneratelimit.AlgorithmSlidingWindow, dataplaneratelimit.AlgorithmTokenBucket:
		return nil
	default:
		return fmt.Errorf("%w: %s", errUnsupportedAlgorithm, algorithm)
	}
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
