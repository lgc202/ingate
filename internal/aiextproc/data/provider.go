// Package data 提供 AI ExtProc 依赖的外部数据来源
package data

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/aiextproc/biz/tokenquota"
	"github.com/lgc202/ingate/internal/aiextproc/data/apiserver"
	dataredis "github.com/lgc202/ingate/internal/aiextproc/data/redis"
	aiextprocservice "github.com/lgc202/ingate/internal/aiextproc/service"
)

// ProviderSet 提供 API Server 配置缓存和 Redis Token 计数器
var ProviderSet = wire.NewSet(
	apiserver.NewConfigCache,
	dataredis.NewTokenCounter,
	NewReadiness,
	wire.Bind(new(aiextprocservice.ModelAPIKeySource), new(*apiserver.ConfigCache)),
	wire.Bind(new(tokenquota.PolicySource), new(*apiserver.ConfigCache)),
	wire.Bind(new(tokenquota.Counter), new(*dataredis.TokenCounter)),
)
