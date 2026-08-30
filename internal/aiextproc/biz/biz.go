// Package biz 提供 AI 请求处理所需的业务服务。
package biz

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/aiextproc/biz/tokenquota"
)

// ProviderSet 汇总 AI 请求处理的业务服务。
var ProviderSet = wire.NewSet(tokenquota.NewLimiter)
