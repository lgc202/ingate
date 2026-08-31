// Package biz 编排声明式资源到 Envoy 配置的控制面用例。
package biz

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/controller/biz/delivery"
	"github.com/lgc202/ingate/internal/controller/conf"
)

// ProviderSet 汇总 Controller 的配置收敛与发布用例。
var ProviderSet = wire.NewSet(NewDelivery, NewController)

// NewDelivery 创建配置发布循环。
func NewDelivery(config *conf.Delivery, publisher delivery.Publisher) (*delivery.Delivery, error) {
	return delivery.New(publisher, delivery.Options{
		ACKTimeout:          config.GetCandidateAckTimeout().AsDuration(),
		NACKRollbackTimeout: config.GetNackRollbackTimeout().AsDuration(),
	})
}
