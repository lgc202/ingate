package conf

import (
	"errors"
	"net"
	"net/url"
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
	if err := validateAuthentication(c.GetAuthentication()); err != nil {
		return err
	}
	return nil
}

func validateAuthentication(authentication *Authentication) error {
	if authentication == nil {
		return errors.New("authentication config is required")
	}
	if !authentication.GetEnabled() {
		return nil
	}
	issuer, err := url.Parse(authentication.GetIssuer())
	if err != nil || issuer.Host == "" {
		return errors.New("authentication issuer must be an absolute URL")
	}
	if issuer.Scheme != "https" && !(issuer.Scheme == "http" && isLoopbackHost(issuer.Hostname())) {
		return errors.New("authentication issuer must use HTTPS unless it is loopback")
	}
	if strings.TrimSpace(authentication.GetAudience()) == "" {
		return errors.New("authentication audience must not be empty")
	}
	if strings.TrimSpace(authentication.GetClientId()) == "" {
		return errors.New("authentication client ID must not be empty")
	}
	hasOpenIDScope := false
	for _, scope := range authentication.GetScopes() {
		if scope == "openid" {
			hasOpenIDScope = true
			break
		}
	}
	if !hasOpenIDScope {
		return errors.New("authentication scopes must include openid")
	}
	if strings.TrimSpace(authentication.GetRolesClaim()) == "" {
		return errors.New("authentication roles claim must not be empty")
	}
	if len(authentication.GetAdminRoles()) == 0 {
		return errors.New("authentication admin roles must not be empty")
	}
	if authentication.GetDiscoveryTimeout() == nil || authentication.GetDiscoveryTimeout().AsDuration() <= 0 {
		return errors.New("authentication discovery timeout must be greater than zero")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
