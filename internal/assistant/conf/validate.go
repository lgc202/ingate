// Package conf 定义并校验 ingate-assistant 进程配置。
package conf

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lgc202/ingate/internal/pkg/appconfig"
)

const (
	maxWorkerConcurrency   = 256
	minWorkerPollInterval  = 100 * time.Millisecond
	minWorkerLeaseDuration = 3 * time.Second
)

// Validate 校验 Assistant 进程启动所需的配置。
func (c *Bootstrap) Validate() error {
	server := c.GetServer()
	if server == nil || server.GetHttp() == nil {
		return errors.New("server HTTP config is required")
	}
	httpServer := server.GetHttp()
	if strings.TrimSpace(httpServer.GetAddr()) == "" {
		return errors.New("server HTTP address must not be empty")
	}
	if httpServer.GetReadinessTimeout() == nil || httpServer.GetReadinessTimeout().AsDuration() <= 0 {
		return errors.New("server HTTP readiness timeout must be greater than zero")
	}
	if server.GetShutdownTimeout() == nil || server.GetShutdownTimeout().AsDuration() <= 0 {
		return errors.New("server shutdown timeout must be greater than zero")
	}
	if err := validateData(c.GetData()); err != nil {
		return err
	}
	stream := c.GetStream()
	if stream == nil || stream.GetRetention() == nil || stream.GetMaxEvents() == 0 ||
		stream.GetRetention().AsDuration() <= 0 {
		return errors.New("stream retention and max events must be greater than zero")
	}
	if stream.GetReadBlock() == nil || stream.GetReadBlock().AsDuration() <= 0 {
		return errors.New("stream read block must be greater than zero")
	}
	worker := c.GetWorker()
	if worker == nil || worker.GetConcurrency() == 0 ||
		worker.GetConcurrency() > maxWorkerConcurrency {
		return fmt.Errorf(
			"worker concurrency must be between 1 and %d",
			maxWorkerConcurrency,
		)
	}
	if worker.GetPollInterval() == nil ||
		worker.GetPollInterval().AsDuration() < minWorkerPollInterval {
		return fmt.Errorf("worker poll interval must be at least %s", minWorkerPollInterval)
	}
	if worker.GetLeaseDuration() == nil ||
		worker.GetLeaseDuration().AsDuration() < minWorkerLeaseDuration {
		return fmt.Errorf("worker lease duration must be at least %s", minWorkerLeaseDuration)
	}
	adminAPI := c.GetAdminApi()
	if adminAPI == nil || strings.TrimSpace(adminAPI.GetAddr()) == "" {
		return errors.New("admin API gRPC address is required")
	}
	if adminAPI.GetTimeout() == nil || adminAPI.GetTimeout().AsDuration() <= 0 {
		return errors.New("admin API timeout must be greater than zero")
	}
	logging := c.GetLogging()
	if logging == nil {
		return errors.New("logging config is required")
	}
	return appconfig.ValidateLogging(logging)
}

func validateData(config *Data) error {
	if config == nil || config.GetMysql() == nil || config.GetRedis() == nil {
		return errors.New("MySQL and Redis config are required")
	}
	mysql := config.GetMysql()
	if strings.TrimSpace(mysql.GetAddress()) == "" ||
		strings.TrimSpace(mysql.GetDatabase()) == "" ||
		strings.TrimSpace(mysql.GetUsername()) == "" {
		return errors.New("MySQL address, database and username are required")
	}
	if mysql.GetDialTimeout() == nil ||
		mysql.GetDialTimeout().AsDuration() <= 0 ||
		mysql.GetMaxOpenConnections() == 0 {
		return errors.New("MySQL timeout and max open connections must be greater than zero")
	}
	if mysql.GetMaxIdleConnections() > mysql.GetMaxOpenConnections() {
		return errors.New("MySQL max idle connections must not exceed max open connections")
	}
	if mysql.GetConnectionMaxLifetime() == nil || mysql.GetConnectionMaxLifetime().AsDuration() <= 0 {
		return errors.New("MySQL connection max lifetime must be greater than zero")
	}
	redis := config.GetRedis()
	if strings.TrimSpace(redis.GetAddress()) == "" || redis.GetDatabase() < 0 {
		return errors.New("redis address is required and database must not be negative")
	}
	if redis.GetDialTimeout() == nil || redis.GetDialTimeout().AsDuration() <= 0 ||
		redis.GetReadTimeout() == nil || redis.GetReadTimeout().AsDuration() <= 0 ||
		redis.GetWriteTimeout() == nil || redis.GetWriteTimeout().AsDuration() <= 0 ||
		redis.GetPoolSize() == 0 {
		return errors.New("redis timeouts and pool size must be greater than zero")
	}
	return nil
}
