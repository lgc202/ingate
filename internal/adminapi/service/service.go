// Package service 装配 Admin API 的产品协议实现。
package service

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/adminapi/service/analytics/aiusage"
	"github.com/lgc202/ingate/internal/adminapi/service/analytics/requestrecord"
	"github.com/lgc202/ingate/internal/adminapi/service/analytics/traffic"
	"github.com/lgc202/ingate/internal/adminapi/service/caller"
	"github.com/lgc202/ingate/internal/adminapi/service/health"
	"github.com/lgc202/ingate/internal/adminapi/service/plugin/source"
	"github.com/lgc202/ingate/internal/adminapi/service/plugin/wasm"
	"github.com/lgc202/ingate/internal/adminapi/service/policy/headertransformation"
	"github.com/lgc202/ingate/internal/adminapi/service/policy/iprestriction"
	"github.com/lgc202/ingate/internal/adminapi/service/policy/mockresponse"
	"github.com/lgc202/ingate/internal/adminapi/service/policy/ratelimit"
	"github.com/lgc202/ingate/internal/adminapi/service/policy/tokenquota"
	"github.com/lgc202/ingate/internal/adminapi/service/routing/certificate"
	"github.com/lgc202/ingate/internal/adminapi/service/routing/gateway"
	"github.com/lgc202/ingate/internal/adminapi/service/routing/route"
	routingservice "github.com/lgc202/ingate/internal/adminapi/service/routing/service"
)

// ProviderSet 汇总 Admin API 的产品协议实现。
var ProviderSet = wire.NewSet(
	aiusage.NewService,
	caller.NewService,
	certificate.NewService,
	gateway.NewService,
	headertransformation.NewService,
	health.NewService,
	iprestriction.NewService,
	mockresponse.NewService,
	source.NewService,
	ratelimit.NewService,
	requestrecord.NewService,
	route.NewService,
	routingservice.NewService,
	tokenquota.NewService,
	traffic.NewService,
	wasm.NewService,
)
