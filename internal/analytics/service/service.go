// Package service 装配 Analytics 的协议服务。
package service

import (
	"github.com/google/wire"

	aiusageservice "github.com/lgc202/ingate/internal/analytics/service/aiusage"
	requestservice "github.com/lgc202/ingate/internal/analytics/service/request"
	trafficservice "github.com/lgc202/ingate/internal/analytics/service/traffic"
)

// ProviderSet 汇总请求明细、流量分析和 AI 用量三组 gRPC 协议实现。
var ProviderSet = wire.NewSet(
	aiusageservice.NewService,
	requestservice.NewService,
	trafficservice.NewService,
)
