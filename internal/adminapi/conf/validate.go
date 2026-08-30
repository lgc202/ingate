// Package conf 定义并校验 ingate-admin-api 进程配置。
package conf

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lgc202/ingate/internal/pkg/appconfig"
	"github.com/lgc202/ingate/internal/pkg/controlplaneauth"
	"github.com/lgc202/ingate/internal/pkg/httpurl"
)

// Validate 校验 Admin API 的进程配置。
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
	grpc := c.GetServer().GetGrpc()
	if grpc == nil || strings.TrimSpace(grpc.GetAddr()) == "" {
		return errors.New("server gRPC address must not be empty")
	}
	if grpc.GetTimeout() == nil || grpc.GetTimeout().AsDuration() <= 0 {
		return errors.New("server gRPC timeout must be greater than zero")
	}
	if c.GetServer().GetShutdownTimeout() == nil || c.GetServer().GetShutdownTimeout().AsDuration() <= 0 {
		return errors.New("server shutdown timeout must be greater than zero")
	}
	if err := validateData(c.GetData()); err != nil {
		return err
	}
	logging := c.GetLogging()
	if logging == nil {
		return errors.New("logging config is required")
	}
	return appconfig.ValidateLogging(logging)
}

func validateData(data *Data) error {
	if data == nil || data.GetApiserver() == nil {
		return errors.New("apiserver config is required")
	}
	apiServer := data.GetApiserver()
	if strings.TrimSpace(apiServer.GetMaster()) == "" &&
		strings.TrimSpace(apiServer.GetKubeconfig()) == "" {
		return errors.New("apiserver master or kubeconfig must be configured")
	}
	if !controlplaneauth.IsValidBearerToken(apiServer.GetBearerToken()) {
		return errors.New("apiserver bearer token is invalid")
	}
	analytics := data.GetAnalytics()
	if analytics == nil {
		return errors.New("analytics config is required")
	}
	if strings.TrimSpace(analytics.GetAddr()) == "" {
		return errors.New("analytics address must not be empty")
	}
	if analytics.GetTimeout() == nil || analytics.GetTimeout().AsDuration() <= 0 {
		return errors.New("analytics timeout must be greater than zero")
	}
	tls := analytics.GetTls()
	if err := validateTLS("analytics", tls); err != nil {
		return err
	}
	aiExtProc := data.GetAiExtProc()
	if aiExtProc == nil || strings.TrimSpace(aiExtProc.GetAddr()) == "" {
		return errors.New("AI ExtProc address must not be empty")
	}
	if aiExtProc.GetTimeout() == nil || aiExtProc.GetTimeout().AsDuration() <= 0 {
		return errors.New("AI ExtProc timeout must be greater than zero")
	}
	if err := validateTLS("AI ExtProc", aiExtProc.GetTls()); err != nil {
		return err
	}
	pluginCatalog := data.GetPluginCatalog()
	if pluginCatalog == nil {
		return errors.New("plugin catalog config is required")
	}
	officialSourceURL := pluginCatalog.GetOfficialSourceUrl()
	if officialSourceURL != "" && !httpurl.IsValid(officialSourceURL) {
		return errors.New("official plugin source URL must be a valid HTTP or HTTPS URL")
	}
	if pluginCatalog.GetRefreshInterval() == nil || pluginCatalog.GetRefreshInterval().AsDuration() <= 0 {
		return errors.New("plugin catalog refresh interval must be greater than zero")
	}
	if pluginCatalog.GetTimeout() == nil || pluginCatalog.GetTimeout().AsDuration() <= 0 {
		return errors.New("plugin catalog timeout must be greater than zero")
	}
	return nil
}

func validateTLS(component string, tls *Data_TLS) error {
	if tls.GetEnabled() && (tls.GetCertFile() == "") != (tls.GetKeyFile() == "") {
		return fmt.Errorf("%s TLS certificate and key must be configured together", component)
	}
	return nil
}
