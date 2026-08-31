package adminapi

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/adminapi/service/aiusage"
	"github.com/lgc202/ingate/internal/adminapi/service/caller"
	"github.com/lgc202/ingate/internal/adminapi/service/certificate"
	"github.com/lgc202/ingate/internal/adminapi/service/gateway"
	"github.com/lgc202/ingate/internal/adminapi/service/headertransformation"
	"github.com/lgc202/ingate/internal/adminapi/service/health"
	"github.com/lgc202/ingate/internal/adminapi/service/iprestriction"
	"github.com/lgc202/ingate/internal/adminapi/service/mockresponse"
	"github.com/lgc202/ingate/internal/adminapi/service/pluginsource"
	"github.com/lgc202/ingate/internal/adminapi/service/ratelimit"
	"github.com/lgc202/ingate/internal/adminapi/service/request"
	"github.com/lgc202/ingate/internal/adminapi/service/route"
	"github.com/lgc202/ingate/internal/adminapi/service/servicemanagement"
	"github.com/lgc202/ingate/internal/adminapi/service/tokenquota"
	"github.com/lgc202/ingate/internal/adminapi/service/traffic"
	"github.com/lgc202/ingate/internal/adminapi/service/wasmplugin"
)

// ServiceProviderSet 汇总 Admin API 的产品协议实现。
var ServiceProviderSet = wire.NewSet(
	aiusage.NewService,
	caller.NewService,
	certificate.NewService,
	gateway.NewService,
	headertransformation.NewService,
	health.NewService,
	iprestriction.NewService,
	mockresponse.NewService,
	pluginsource.NewService,
	ratelimit.NewService,
	request.NewService,
	route.NewService,
	servicemanagement.NewService,
	tokenquota.NewService,
	traffic.NewService,
	wasmplugin.NewService,
)
