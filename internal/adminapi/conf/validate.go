package conf

import (
	"errors"
	"strings"

	"github.com/lgc202/ingate/pkg/mysqlx"
	"github.com/lgc202/ingate/pkg/redisx"
)

// Validate 校验 Admin API 的进程配置
func (c *Bootstrap) Validate() error {
	if c.GetServer() == nil || c.GetServer().GetHttp() == nil {
		return errors.New("server http config is required")
	}
	http := c.GetServer().GetHttp()
	if http.GetNetwork() == "" {
		return errors.New("server http network must not be empty")
	}
	if strings.TrimSpace(http.GetAddr()) == "" {
		return errors.New("server http address must not be empty")
	}
	if http.GetTimeout() == nil || http.GetTimeout().AsDuration() <= 0 {
		return errors.New("server http timeout must be greater than zero")
	}
	if c.GetData() == nil || c.GetData().GetApiserver() == nil || c.GetData().GetMysql() == nil || c.GetData().GetRedis() == nil {
		return errors.New("data config is incomplete")
	}
	mysql := c.GetData().GetMysql()
	if mysql.GetConnectionMaxLifetime() == nil {
		return errors.New("mysql connection max lifetime is required")
	}
	if err := (mysqlx.Config{
		DSN:                   mysql.GetDsn(),
		MaxOpenConnections:    int(mysql.GetMaxOpenConnections()),
		MaxIdleConnections:    int(mysql.GetMaxIdleConnections()),
		ConnectionMaxLifetime: mysql.GetConnectionMaxLifetime().AsDuration(),
	}).Validate(); err != nil {
		return err
	}
	redis := c.GetData().GetRedis()
	if err := (redisx.Config{
		Address:  redis.GetAddress(),
		Password: redis.GetPassword(),
		Database: int(redis.GetDatabase()),
	}).Validate(); err != nil {
		return err
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
