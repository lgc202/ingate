// Package data 提供 Authz 依赖的外部数据来源。
package data

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/authz/biz"
	"github.com/lgc202/ingate/internal/authz/biz/ratelimit"
	"github.com/lgc202/ingate/internal/authz/data/apiserver"
	dataredis "github.com/lgc202/ingate/internal/authz/data/redis"
)

// ProviderSet 提供 Caller 凭据缓存和共享请求计数器。
var ProviderSet = wire.NewSet(
	apiserver.NewCredentialCache,
	dataredis.NewRateCounter,
	NewReadiness,
	wire.Bind(new(biz.CredentialSource), new(*apiserver.CredentialCache)),
	wire.Bind(new(ratelimit.Counter), new(*dataredis.RateCounter)),
)
