package compiler

import (
	"time"

	accesslogv3 "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	grpcaccesslogv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/access_loggers/grpc/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	httpGRPCAccessLoggerName = "envoy.access_loggers.http_grpc"
	alsClusterName           = "ingate-system-als"
	alsLogName               = "ingate"
	alsBufferSizeBytes       = 64 * 1024
	alsFlushInterval         = time.Second
)

// buildHTTPAccessLog 只发送 Envoy 标准请求元数据，不额外采集 Header 或正文
//
// Envoy 先在内存中小批量聚合记录，再通过长连接交给 ALS；磁盘兜底和 Kafka
// 重投由 ALS 负责，避免把消息系统与本地文件语义放入数据面配置
func buildHTTPAccessLog() (*accesslogv3.AccessLog, error) {
	configuration := &grpcaccesslogv3.HttpGrpcAccessLogConfig{
		CommonConfig: &grpcaccesslogv3.CommonGrpcAccessLogConfig{
			LogName: alsLogName,
			GrpcService: &corev3.GrpcService{
				TargetSpecifier: &corev3.GrpcService_EnvoyGrpc_{
					EnvoyGrpc: &corev3.GrpcService_EnvoyGrpc{ClusterName: alsClusterName},
				},
			},
			TransportApiVersion: corev3.ApiVersion_V3,
			BufferFlushInterval: durationpb.New(alsFlushInterval),
			BufferSizeBytes:     wrapperspb.UInt32(alsBufferSizeBytes),
		},
	}
	typedConfig, err := anypb.New(configuration)
	if err != nil {
		return nil, err
	}
	return &accesslogv3.AccessLog{
		Name:       httpGRPCAccessLoggerName,
		ConfigType: &accesslogv3.AccessLog_TypedConfig{TypedConfig: typedConfig},
	}, nil
}
