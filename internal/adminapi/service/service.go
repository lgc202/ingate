// Package service 实现 Admin API 的传输协议适配
package service

import "github.com/google/wire"

// ProviderSet 汇总 Admin API 的协议服务
var ProviderSet = wire.NewSet(
	NewGatewayService,
	NewRouteService,
	NewUpstreamService,
	NewCertificateService,
	NewAccessKeyService,
	NewRateLimitPolicyService,
	NewAccessControlPolicyService,
	NewTokenQuotaPolicyService,
	NewConfigurationService,
	NewHealthService,
)
