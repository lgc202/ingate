// Package data 提供 Authz 依赖的外部数据来源
package data

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/authz/biz"
	"github.com/lgc202/ingate/internal/authz/data/apiserver"
)

// ProviderSet 提供 API Server Caller 凭据缓存
var ProviderSet = wire.NewSet(
	apiserver.NewCredentialCache,
	wire.Bind(new(biz.CredentialSource), new(*apiserver.CredentialCache)),
)
