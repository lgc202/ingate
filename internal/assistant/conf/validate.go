// Package conf 定义并校验 ingate-assistant 进程配置
package conf

import (
	"errors"
	"strings"
)

// Validate 校验 Assistant 进程启动所需的配置
func (c *Bootstrap) Validate() error {
	if c.GetServer() == nil || c.GetServer().GetHttp() == nil {
		return errors.New("server HTTP config is required")
	}
	if strings.TrimSpace(c.GetServer().GetHttp().GetAddr()) == "" {
		return errors.New("server HTTP address must not be empty")
	}
	if c.GetServer().GetHttp().GetTimeout().AsDuration() <= 0 {
		return errors.New("server HTTP timeout must be greater than zero")
	}
	if c.GetServer().GetShutdownTimeout().AsDuration() <= 0 {
		return errors.New("server shutdown timeout must be greater than zero")
	}
	if err := validateData(c.GetData()); err != nil {
		return err
	}
	if c.GetStream() == nil || c.GetStream().GetRetention().AsDuration() <= 0 || c.GetStream().GetMaxEvents() == 0 {
		return errors.New("stream retention and max events must be greater than zero")
	}
	if c.GetStream().GetReadBlock().AsDuration() <= 0 {
		return errors.New("stream read block must be greater than zero")
	}
	if c.GetLogging() == nil {
		return errors.New("logging config is required")
	}
	switch strings.ToLower(c.GetLogging().GetFormat()) {
	case "json", "text":
	default:
		return errors.New("logging format must be json or text")
	}
	switch strings.ToLower(c.GetLogging().GetLevel()) {
	case "debug", "info", "warn", "error":
	default:
		return errors.New("logging level must be debug, info, warn or error")
	}
	return nil
}

func validateData(config *Data) error {
	if config == nil || config.GetMysql() == nil || config.GetRedis() == nil {
		return errors.New("MySQL and Redis config are required")
	}
	mysql := config.GetMysql()
	if strings.TrimSpace(mysql.GetAddress()) == "" || strings.TrimSpace(mysql.GetDatabase()) == "" || strings.TrimSpace(mysql.GetUsername()) == "" {
		return errors.New("MySQL address, database and username are required")
	}
	if mysql.GetDialTimeout().AsDuration() <= 0 || mysql.GetMaxOpenConnections() == 0 {
		return errors.New("MySQL timeout and max open connections must be greater than zero")
	}
	if mysql.GetMaxIdleConnections() > mysql.GetMaxOpenConnections() {
		return errors.New("MySQL max idle connections must not exceed max open connections")
	}
	if mysql.GetConnectionMaxLifetime().AsDuration() <= 0 {
		return errors.New("MySQL connection max lifetime must be greater than zero")
	}
	redis := config.GetRedis()
	if strings.TrimSpace(redis.GetAddress()) == "" || redis.GetDatabase() < 0 {
		return errors.New("redis address is required and database must not be negative")
	}
	if redis.GetDialTimeout().AsDuration() <= 0 || redis.GetReadTimeout().AsDuration() <= 0 || redis.GetWriteTimeout().AsDuration() <= 0 || redis.GetPoolSize() == 0 {
		return errors.New("redis timeouts and pool size must be greater than zero")
	}
	return nil
}
