// Package service 装配 Analytics 的协议服务
package service

import (
	"github.com/google/wire"

	requestservice "github.com/lgc202/ingate/internal/analytics/service/request"
	trafficservice "github.com/lgc202/ingate/internal/analytics/service/traffic"
)

// ProviderSet 汇总 Analytics 的协议服务
var ProviderSet = wire.NewSet(
	requestservice.NewService,
	trafficservice.NewService,
)
