package handler

import (
	gatewayhandler "github.com/lgc202/ingate/internal/adminapi/handler/gateway"
	"github.com/lgc202/ingate/internal/adminapi/service"
)

// Handler 聚合 admin-api HTTP handler
type Handler struct {
	Gateway *gatewayhandler.Handler
}

// New 创建 handler 聚合入口
func New(service *service.Service) *Handler {
	return &Handler{
		Gateway: gatewayhandler.New(service.Gateway),
	}
}
