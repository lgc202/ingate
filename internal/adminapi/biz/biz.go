// Package biz 装配 Admin API 的领域用例。
package biz

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/adminapi/biz/analytics/aiusage"
	"github.com/lgc202/ingate/internal/adminapi/biz/analytics/requestrecord"
	"github.com/lgc202/ingate/internal/adminapi/biz/analytics/traffic"
	"github.com/lgc202/ingate/internal/adminapi/biz/caller"
	"github.com/lgc202/ingate/internal/adminapi/biz/plugin"
	"github.com/lgc202/ingate/internal/adminapi/biz/plugin/source"
	"github.com/lgc202/ingate/internal/adminapi/biz/plugin/wasm"
	"github.com/lgc202/ingate/internal/adminapi/biz/policy"
	"github.com/lgc202/ingate/internal/adminapi/biz/policy/headertransformation"
	"github.com/lgc202/ingate/internal/adminapi/biz/policy/iprestriction"
	"github.com/lgc202/ingate/internal/adminapi/biz/policy/mockresponse"
	"github.com/lgc202/ingate/internal/adminapi/biz/policy/ratelimit"
	"github.com/lgc202/ingate/internal/adminapi/biz/policy/tokenquota"
	"github.com/lgc202/ingate/internal/adminapi/biz/routing/certificate"
	"github.com/lgc202/ingate/internal/adminapi/biz/routing/gateway"
	"github.com/lgc202/ingate/internal/adminapi/biz/routing/route"
	"github.com/lgc202/ingate/internal/adminapi/biz/routing/service"
)

// ProviderSet 汇总 Admin API 各领域用例及跨领域检查能力。
var ProviderSet = wire.NewSet(
	policy.NewUsageFinder,
	plugin.NewUsageFinder,
	plugin.NewInstallationChecker,
	wire.Bind(new(wasm.PolicyUsageLister), new(*plugin.UsageFinder)),
	aiusage.NewUsecase,
	caller.NewUsecase,
	certificate.NewUsecase,
	gateway.NewUsecase,
	headertransformation.NewUsecase,
	iprestriction.NewUsecase,
	mockresponse.NewUsecase,
	source.NewUsecase,
	ratelimit.NewUsecase,
	requestrecord.NewUsecase,
	route.NewUsecase,
	service.NewUsecase,
	tokenquota.NewUsecase,
	traffic.NewUsecase,
	wasm.NewUsecase,
)
