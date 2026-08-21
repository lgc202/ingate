// Package biz 装配 Analytics 的请求记录和流量分析用例
//
// biz 只依赖消费者侧存储接口，不感知 Kafka、ClickHouse 或 gRPC 实现
package biz

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/analytics/biz/request"
	"github.com/lgc202/ingate/internal/analytics/biz/traffic"
)

// ProviderSet 汇总 Analytics 的业务用例
var ProviderSet = wire.NewSet(
	request.NewRecorder,
	request.NewQuery,
	traffic.NewQuery,
)
