// Package apiserverclient 构造各控制面组件共享的声明式 API 客户端配置。
package apiserverclient

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/lgc202/ingate/internal/pkg/controlplaneauth"
)

const (
	defaultQPS   float32 = 50
	defaultBurst int     = 100
)

// Options 定义声明式 API 客户端的连接和身份参数。
type Options struct {
	MasterURL      string
	KubeconfigPath string
	BearerToken    string
	UserAgent      string
}

// NewConfig 加载 kubeconfig，并收敛 TLS、认证、限流和客户端身份配置。
func NewConfig(options Options) (*rest.Config, error) {
	if !controlplaneauth.IsValidBearerToken(options.BearerToken) {
		return nil, errors.New("API Server bearer token is invalid")
	}
	if options.UserAgent == "" || strings.TrimSpace(options.UserAgent) != options.UserAgent {
		return nil, errors.New("API Server user agent is invalid")
	}
	config, err := clientcmd.BuildConfigFromFlags(options.MasterURL, options.KubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("build API Server client config: %w", err)
	}
	serverURL, err := url.Parse(config.Host)
	if err != nil ||
		serverURL.Scheme != "https" ||
		serverURL.Host == "" ||
		serverURL.User != nil ||
		serverURL.RawQuery != "" ||
		serverURL.Fragment != "" {
		return nil, errors.New("API Server address must be a valid HTTPS URL")
	}
	if config.TLSClientConfig.Insecure {
		return nil, errors.New("API Server client must verify the server certificate")
	}

	// kubeconfig 只提供服务地址和 CA 信任，不允许额外的用户凭据、认证插件或自定义传输
	// 与 Ingate 的内部 Bearer Token 并存。
	config = rest.AnonymousClientConfig(config)
	config.BearerToken = options.BearerToken
	config.UserAgent = options.UserAgent
	config.QPS = defaultQPS
	config.Burst = defaultBurst
	return config, nil
}
