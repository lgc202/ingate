package service

import (
	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	"github.com/lgc202/ingate/internal/adminapi/store"
)

// Service 聚合 admin-api 面向控制台的查询用例
type Service struct {
	Gateway *gatewayservice.Service
}

// New 创建 service 聚合入口
func New(store *store.Store) *Service {
	return &Service{
		Gateway: gatewayservice.New(store.Gateway, store.Route, store.Runtime, store.Upstream),
	}
}
