// Package conf 定义并校验 ingate-assistant 进程配置。
package conf

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/lgc202/ingate/internal/pkg/appconfig"
)

// Validate 校验 Assistant 进程启动所需的配置。
func (c *Bootstrap) Validate() error {
	server := c.GetServer()
	if server == nil || server.GetHttp() == nil {
		return errors.New("server HTTP config is required")
	}
	if strings.TrimSpace(server.GetHttp().GetAddr()) == "" {
		return errors.New("server HTTP address must not be empty")
	}
	if server.GetHttp().GetReadinessTimeout() == nil ||
		server.GetHttp().GetReadinessTimeout().AsDuration() <= 0 {
		return errors.New("server HTTP readiness timeout must be greater than zero")
	}
	if server.GetShutdownTimeout() == nil || server.GetShutdownTimeout().AsDuration() <= 0 {
		return errors.New("server shutdown timeout must be greater than zero")
	}
	if err := validateMySQL(c.GetData()); err != nil {
		return fmt.Errorf("validate MySQL config: %w", err)
	}
	if err := validateTemporal(c.GetTemporal(), server); err != nil {
		return fmt.Errorf("validate Temporal config: %w", err)
	}
	if err := validateModel(c.GetModel()); err != nil {
		return fmt.Errorf("validate model config: %w", err)
	}
	if c.GetLogging() == nil {
		return errors.New("logging config is required")
	}
	return appconfig.ValidateLogging(c.GetLogging())
}

func validateMySQL(data *Data) error {
	if data == nil || data.GetMysql() == nil {
		return errors.New("MySQL config is required")
	}
	mysql := data.GetMysql()
	if strings.TrimSpace(mysql.GetAddress()) == "" ||
		strings.TrimSpace(mysql.GetDatabase()) == "" ||
		strings.TrimSpace(mysql.GetUsername()) == "" {
		return errors.New("MySQL address, database and username are required")
	}
	if mysql.GetDialTimeout() == nil || mysql.GetDialTimeout().AsDuration() <= 0 ||
		mysql.GetMaxOpenConnections() == 0 {
		return errors.New("MySQL timeout and max open connections must be greater than zero")
	}
	if mysql.GetMaxIdleConnections() > mysql.GetMaxOpenConnections() {
		return errors.New("MySQL max idle connections must not exceed max open connections")
	}
	if mysql.GetConnectionMaxLifetime() == nil ||
		mysql.GetConnectionMaxLifetime().AsDuration() <= 0 {
		return errors.New("MySQL connection max lifetime must be greater than zero")
	}
	return nil
}

func validateTemporal(config *Temporal, server *Server) error {
	if config == nil || strings.TrimSpace(config.GetAddress()) == "" ||
		strings.TrimSpace(config.GetNamespace()) == "" ||
		strings.TrimSpace(config.GetTaskQueue()) == "" {
		return errors.New("Temporal address, namespace and task queue are required")
	}
	if config.GetConnectTimeout() == nil || config.GetConnectTimeout().AsDuration() <= 0 {
		return errors.New("Temporal connect timeout must be greater than zero")
	}
	if config.GetWorkerStopTimeout() == nil || config.GetWorkerStopTimeout().AsDuration() <= 0 {
		return errors.New("Temporal worker stop timeout must be greater than zero")
	}
	if config.GetWorkerStopTimeout().AsDuration() >= server.GetShutdownTimeout().AsDuration() {
		return errors.New("Temporal worker stop timeout must be shorter than server shutdown timeout")
	}
	return nil
}

func validateModel(config *Model) error {
	if config == nil || strings.TrimSpace(config.GetHealthUrl()) == "" {
		return errors.New("model health URL is required")
	}
	healthURL, err := url.Parse(config.GetHealthUrl())
	if err != nil || healthURL.Host == "" ||
		(healthURL.Scheme != "http" && healthURL.Scheme != "https") || healthURL.User != nil {
		return errors.New("model health URL must be an HTTP or HTTPS URL without credentials")
	}
	if config.GetHealthTimeout() == nil || config.GetHealthTimeout().AsDuration() <= 0 {
		return errors.New("model health timeout must be greater than zero")
	}
	return nil
}
