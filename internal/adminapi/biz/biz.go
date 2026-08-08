// Package biz 实现 ingate-admin-api 的业务用例和数据访问边界
package biz

import "github.com/google/wire"

// ProviderSet 提供跨领域的业务能力
var ProviderSet = wire.NewSet(
	NewPolicyUsageFinder,
)
