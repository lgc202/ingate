// Package conf 定义并校验 ingate-ai-extproc 进程配置。
package conf

import (
	"errors"
	"strings"

	"github.com/lgc202/ingate/internal/pkg/appconfig"
	"github.com/lgc202/ingate/internal/pkg/controlplaneauth"
)

// Validate 校验 AI ExtProc 进程启动所需的配置。
func (c *Bootstrap) Validate() error {
	server := c.GetServer()
	if server == nil || server.GetGrpc() == nil || server.GetHttp() == nil {
		return errors.New("server gRPC and HTTP config are required")
	}
	grpcServer := server.GetGrpc()
	if strings.TrimSpace(grpcServer.GetAddr()) == "" {
		return errors.New("server gRPC address must not be empty")
	}
	if err := validateServerTLS(grpcServer.GetTls()); err != nil {
		return err
	}
	httpServer := server.GetHttp()
	if strings.TrimSpace(httpServer.GetAddr()) == "" {
		return errors.New("server HTTP address must not be empty")
	}
	if httpServer.GetTimeout() == nil || httpServer.GetTimeout().AsDuration() <= 0 {
		return errors.New("server HTTP timeout must be greater than zero")
	}
	if server.GetShutdownTimeout() == nil || server.GetShutdownTimeout().AsDuration() <= 0 {
		return errors.New("server shutdown timeout must be greater than zero")
	}
	data := c.GetData()
	if data == nil || data.GetApiserver() == nil {
		return errors.New("data apiserver config is required")
	}
	apiServer := data.GetApiserver()
	if strings.TrimSpace(apiServer.GetMaster()) == "" &&
		strings.TrimSpace(apiServer.GetKubeconfig()) == "" {
		return errors.New("data apiserver master or kubeconfig must be configured")
	}
	if !controlplaneauth.IsValidBearerToken(apiServer.GetBearerToken()) {
		return errors.New("data apiserver bearer token is invalid")
	}
	redis := data.GetRedis()
	if redis == nil || strings.TrimSpace(redis.GetAddress()) == "" {
		return errors.New("data redis address must not be empty")
	}
	if redis.GetDatabase() < 0 {
		return errors.New("data redis database must not be negative")
	}
	if redis.GetDialTimeout() == nil || redis.GetDialTimeout().AsDuration() <= 0 {
		return errors.New("data redis dial timeout must be greater than zero")
	}
	if redis.GetOperationTimeout() == nil || redis.GetOperationTimeout().AsDuration() <= 0 {
		return errors.New("data redis operation timeout must be greater than zero")
	}
	logging := c.GetLogging()
	if logging == nil {
		return errors.New("logging config is required")
	}
	return appconfig.ValidateLogging(logging)
}

func validateServerTLS(config *Server_GRPC_TLS) error {
	if config == nil || !config.GetEnabled() {
		return nil
	}
	if config.GetCertFile() == "" || config.GetKeyFile() == "" {
		return errors.New("server gRPC TLS certificate and key are required")
	}
	return nil
}
