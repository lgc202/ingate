// Package biz 实现 ingate-admin-api 的业务用例和数据访问边界
package biz

import "github.com/google/wire"

// ProviderSet 汇总管理 API 的业务用例
var ProviderSet = wire.NewSet(
	NewAccessKeyUsecase,
	NewCertificateUsecase,
	NewConfigurationUsecase,
	NewGatewayUsecase,
	NewRouteUsecase,
	NewUpstreamUsecase,
	NewAccessControlPolicyUsecase,
	NewRateLimitPolicyUsecase,
	NewTokenQuotaPolicyUsecase,
	NewPolicyUsageFinder,
)
