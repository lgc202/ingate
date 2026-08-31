// Package conf 定义并校验 ingate-apiserver 进程配置。
package conf

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"path"
	"strconv"
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"

	"github.com/lgc202/ingate/internal/pkg/appconfig"
	"github.com/lgc202/ingate/internal/pkg/controlplaneauth"
)

const (
	maxEtcdEndpoints     = 16
	maxEtcdEndpointBytes = 2_048
	maxEtcdPrefixBytes   = 1_024
)

// Validate 校验 API Server 进程启动所需的配置。
func (c *Bootstrap) Validate() error {
	if c.GetServer() == nil || c.GetServer().GetHttp() == nil {
		return errors.New("server HTTP config is required")
	}
	host, portText, err := net.SplitHostPort(c.GetServer().GetHttp().GetAddr())
	if err != nil || net.ParseIP(host) == nil {
		return errors.New("server HTTP address must contain a valid IP and port")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("server HTTP port must be between 1 and 65535")
	}
	if strings.TrimSpace(c.GetServer().GetHttp().GetCertDirectory()) == "" {
		return errors.New("server certificate directory must not be empty")
	}
	if c.GetServer().GetShutdownTimeout() == nil || c.GetServer().GetShutdownTimeout().AsDuration() <= 0 {
		return errors.New("server shutdown timeout must be greater than zero")
	}
	authentication := c.GetServer().GetAuthentication()
	if authentication == nil || !controlplaneauth.IsValidBearerToken(authentication.GetBearerToken()) {
		return fmt.Errorf(
			"server bearer token must contain %d to %d valid characters",
			controlplaneauth.MinBearerTokenBytes,
			controlplaneauth.MaxBearerTokenBytes,
		)
	}

	if c.GetData() == nil || c.GetData().GetEtcd() == nil {
		return errors.New("etcd config is required")
	}
	if err := validateEtcd(c.GetData().GetEtcd()); err != nil {
		return fmt.Errorf("validate etcd config: %w", err)
	}

	logging := c.GetLogging()
	if logging == nil {
		return errors.New("logging config is required")
	}
	return appconfig.ValidateLogging(logging)
}

func validateEtcd(config *Data_Etcd) error {
	endpoints := config.GetEndpoints()
	if len(endpoints) == 0 {
		return errors.New("at least one etcd endpoint is required")
	}
	if len(endpoints) > maxEtcdEndpoints {
		return fmt.Errorf("etcd endpoint count must not exceed %d", maxEtcdEndpoints)
	}

	seen := make(map[string]bool, len(endpoints))
	for i, endpoint := range endpoints {
		if err := validateEtcdEndpoint(endpoint); err != nil {
			return fmt.Errorf("etcd endpoint %d: %w", i+1, err)
		}
		if seen[endpoint] {
			return fmt.Errorf("etcd endpoint %d duplicates an earlier endpoint", i+1)
		}
		seen[endpoint] = true
	}

	prefix := config.GetPrefix()
	if prefix == "/" ||
		len(prefix) > maxEtcdPrefixBytes ||
		strings.TrimSpace(prefix) != prefix ||
		!strings.HasPrefix(prefix, "/") ||
		path.Clean(prefix) != prefix {
		return errors.New("etcd prefix must be a canonical non-root absolute path")
	}
	return nil
}

func validateEtcdEndpoint(endpoint string) error {
	if endpoint == "" ||
		len(endpoint) > maxEtcdEndpointBytes ||
		strings.TrimSpace(endpoint) != endpoint {
		return errors.New("must not be empty, too long, or contain surrounding whitespace")
	}

	parsed, err := url.ParseRequestURI(endpoint)
	if err != nil ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Host == "" ||
		parsed.User != nil ||
		parsed.Path != "" ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return errors.New("must use HTTP or HTTPS and contain only a host and port")
	}
	host, portText, err := net.SplitHostPort(parsed.Host)
	if err != nil || host == "" {
		return errors.New("must contain a host and port")
	}
	if net.ParseIP(host) == nil &&
		len(k8svalidation.IsDNS1123Subdomain(strings.ToLower(host))) != 0 {
		return errors.New("host must be an IP address or DNS name")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65_535 {
		return errors.New("port must be between 1 and 65535")
	}
	return nil
}
