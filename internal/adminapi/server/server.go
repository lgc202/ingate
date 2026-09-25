package server

import (
	"net/http"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	adminhandler "github.com/lgc202/ingate/internal/adminapi/handler"
	gatewayhandler "github.com/lgc202/ingate/internal/adminapi/handler/gateway"
	"github.com/lgc202/ingate/internal/adminapi/middleware"
)

// Handlers 保存 Admin API 需要注册的业务 Handler。
type Handlers struct {
	Gateway *gatewayhandler.Handler
}

// New 构造并注册 Admin API 的入站服务，但不开始监听端口。
func New(config *conf.Server, handlers Handlers) (*kratoshttp.Server, *kratosgrpc.Server) {
	validate := middleware.ValidateRequests()
	httpServer := kratoshttp.NewServer(
		kratoshttp.Network(config.GetHttp().GetNetwork()),
		kratoshttp.Address(config.GetHttp().GetAddr()),
		kratoshttp.Timeout(config.GetHttp().GetTimeout().AsDuration()),
		kratoshttp.Middleware(validate),
		kratoshttp.RequestDecoder(decodeHTTPRequest),
		kratoshttp.ResponseEncoder(encodeHTTPResponse),
		kratoshttp.ErrorEncoder(encodeHTTPError),
	)
	grpcServer := kratosgrpc.NewServer(
		kratosgrpc.Network(config.GetGrpc().GetNetwork()),
		kratosgrpc.Address(config.GetGrpc().GetAddr()),
		kratosgrpc.Timeout(config.GetGrpc().GetTimeout().AsDuration()),
		kratosgrpc.Middleware(validate),
	)

	adminv1.RegisterGatewayServiceHTTPServer(httpServer, handlers.Gateway)
	adminv1.RegisterGatewayServiceServer(grpcServer, handlers.Gateway)

	return httpServer, grpcServer
}

func encodeHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	// 生成的 HTTP 入口先解码再执行中间件，解码错误需在此处统一成公开错误。
	if kratoserrors.Code(err) == http.StatusBadRequest && kratoserrors.Reason(err) == "CODEC" {
		err = adminhandler.ToInvalidArgument(err)
	}
	kratoshttp.DefaultErrorEncoder(w, r, err)
}
