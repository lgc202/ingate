package server

import (
	"log/slog"

	kratosgrpc "github.com/go-kratos/kratos/v3/transport/grpc"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/conf"
)

// NewGRPCServer 创建供 Assistant 等内部组件访问的 Admin API gRPC transport。
func NewGRPCServer(config *conf.Server, logger *slog.Logger, services *Services) *kratosgrpc.Server {
	grpcConfig := config.GetGrpc()
	server := kratosgrpc.NewServer(
		kratosgrpc.Network("tcp"),
		kratosgrpc.Address(grpcConfig.GetAddr()),
		kratosgrpc.Timeout(grpcConfig.GetTimeout().AsDuration()),
		kratosgrpc.Middleware(serverMiddleware(logger)...),
	)
	services.registerGRPC(server)
	return server
}

func (s *Services) registerGRPC(server *kratosgrpc.Server) {
	adminv1.RegisterAIUsageServiceServer(server, s.aiUsage)
	adminv1.RegisterCallerServiceServer(server, s.caller)
	adminv1.RegisterGatewayServiceServer(server, s.gateway)
	adminv1.RegisterRouteServiceServer(server, s.route)
	adminv1.RegisterUpstreamServiceServer(server, s.upstream)
	adminv1.RegisterCertificateServiceServer(server, s.certificate)
	adminv1.RegisterRateLimitPolicyServiceServer(server, s.rateLimit)
	adminv1.RegisterIPRestrictionPolicyServiceServer(server, s.ipRestriction)
	adminv1.RegisterRequestRecordServiceServer(server, s.request)
	adminv1.RegisterTrafficAnalysisServiceServer(server, s.traffic)
	adminv1.RegisterTokenQuotaPolicyServiceServer(server, s.tokenQuota)
	adminv1.RegisterHealthServiceServer(server, s.health)
	adminv1.RegisterHeaderTransformationPolicyServiceServer(server, s.headerTransformation)
	adminv1.RegisterMockResponsePolicyServiceServer(server, s.mockResponse)
	adminv1.RegisterWasmPluginServiceServer(server, s.wasmPlugin)
	adminv1.RegisterPluginSourceServiceServer(server, s.pluginSource)
}
