// Package main 通过真实 Console 会话验证核心 Admin API 旅程。
package main

import (
	"context"
	"fmt"
	"os"
	"time"
)

const defaultTimeout = 3 * time.Minute

type config struct {
	baseURL  string
	username string
	password string
	timeout  time.Duration
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "admin API E2E failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("admin API E2E passed")
}

func run() error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), config.timeout)
	defer cancel()

	client, err := newAPIClient(config.baseURL)
	if err != nil {
		return err
	}
	if err := client.login(ctx, config.username, config.password); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	prefix, err := uniquePrefix()
	if err != nil {
		return err
	}
	journey := newJourney(client, prefix)
	defer journey.cleanup()
	return journey.run(ctx)
}

func loadConfig() (config, error) {
	value := config{
		baseURL:  envOrDefault("INGATE_E2E_BASE_URL", "http://127.0.0.1:8001"),
		username: envOrDefault("INGATE_E2E_USERNAME", "admin"),
		password: os.Getenv("INGATE_E2E_PASSWORD"),
		timeout:  defaultTimeout,
	}
	if value.password == "" {
		return config{}, fmt.Errorf("INGATE_E2E_PASSWORD is required")
	}
	if rawTimeout := os.Getenv("INGATE_E2E_TIMEOUT"); rawTimeout != "" {
		timeout, err := time.ParseDuration(rawTimeout)
		if err != nil || timeout <= 0 {
			return config{}, fmt.Errorf("INGATE_E2E_TIMEOUT must be a positive duration")
		}
		value.timeout = timeout
	}
	return value, nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
