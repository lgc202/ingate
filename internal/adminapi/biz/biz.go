// Package biz 装配 Admin API 的领域用例。
package biz

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/adminapi/biz/aiusage"
	"github.com/lgc202/ingate/internal/adminapi/biz/caller"
	"github.com/lgc202/ingate/internal/adminapi/biz/certificate"
	"github.com/lgc202/ingate/internal/adminapi/biz/gateway"
	"github.com/lgc202/ingate/internal/adminapi/biz/headertransformation"
	"github.com/lgc202/ingate/internal/adminapi/biz/iprestriction"
	"github.com/lgc202/ingate/internal/adminapi/biz/mockresponse"
	"github.com/lgc202/ingate/internal/adminapi/biz/plugin"
	"github.com/lgc202/ingate/internal/adminapi/biz/pluginsource"
	"github.com/lgc202/ingate/internal/adminapi/biz/policy"
	"github.com/lgc202/ingate/internal/adminapi/biz/ratelimit"
	"github.com/lgc202/ingate/internal/adminapi/biz/request"
	"github.com/lgc202/ingate/internal/adminapi/biz/route"
	"github.com/lgc202/ingate/internal/adminapi/biz/service"
	"github.com/lgc202/ingate/internal/adminapi/biz/tokenquota"
	"github.com/lgc202/ingate/internal/adminapi/biz/traffic"
	"github.com/lgc202/ingate/internal/adminapi/biz/wasmplugin"
)

// ProviderSet 汇总 Admin API 各领域用例及跨领域检查能力。
var ProviderSet = wire.NewSet(
	policy.NewUsageFinder,
	plugin.NewUsageFinder,
	plugin.NewInstallationChecker,
	wire.Bind(new(wasmplugin.PolicyUsageLister), new(*plugin.UsageFinder)),
	aiusage.NewUsecase,
	caller.NewUsecase,
	certificate.NewUsecase,
	gateway.NewUsecase,
	headertransformation.NewUsecase,
	iprestriction.NewUsecase,
	mockresponse.NewUsecase,
	pluginsource.NewUsecase,
	ratelimit.NewUsecase,
	request.NewUsecase,
	route.NewUsecase,
	service.NewUsecase,
	tokenquota.NewUsecase,
	traffic.NewUsecase,
	wasmplugin.NewUsecase,
)
