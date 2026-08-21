package compiler

import (
	"fmt"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	upstreamcodecv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/upstream_codec/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	upstreamhttpv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	aiprotocol "github.com/lgc202/ingate/internal/pkg/aiextproc"
)

const (
	httpAIDownstreamExtProcFilterName = "envoy.filters.http.ext_proc/ingate-ai-downstream"
	httpAIUpstreamExtProcFilterName   = "envoy.filters.http.ext_proc/ingate-ai-upstream"
	httpUpstreamCodecFilterName       = "envoy.filters.http.upstream_codec"
	httpProtocolOptionsName           = "envoy.extensions.upstreams.http.v3.HttpProtocolOptions"
	aiExtProcClusterName              = "ingate-system-ai-extproc"
	aiExtProcMessageTimeout           = 10 * time.Second
)

// buildAIDownstreamExtProcHTTPFilter 构造客户端 downstream 过滤器
// 默认关闭后只由 AI Route 的兜底路由启用，普通 API 不会发送 gRPC 或缓冲正文
func buildAIDownstreamExtProcHTTPFilter() (*hcmv3.HttpFilter, error) {
	configuration := &extprocv3.ExternalProcessor{
		GrpcService: aiExtProcGRPCService(),
		ProcessingMode: &extprocv3.ProcessingMode{
			RequestHeaderMode:   extprocv3.ProcessingMode_SEND,
			RequestBodyMode:     extprocv3.ProcessingMode_BUFFERED,
			RequestTrailerMode:  extprocv3.ProcessingMode_SKIP,
			ResponseHeaderMode:  extprocv3.ProcessingMode_SEND,
			ResponseBodyMode:    extprocv3.ProcessingMode_BUFFERED,
			ResponseTrailerMode: extprocv3.ProcessingMode_SKIP,
		},
		MetadataOptions: aiExtProcMetadataOptions(),
		MessageTimeout:  durationpb.New(aiExtProcMessageTimeout),
		// AI Route 的协议转换属于请求正确性的组成部分，组件故障时不能把未转换请求发给厂商
		FailureModeAllow:  false,
		AllowModeOverride: true,
	}
	typedConfig, err := marshalAIExtProcConfig("downstream", configuration)
	if err != nil {
		return nil, err
	}
	return &hcmv3.HttpFilter{
		Name:       httpAIDownstreamExtProcFilterName,
		Disabled:   true,
		ConfigType: &hcmv3.HttpFilter_TypedConfig{TypedConfig: typedConfig},
	}, nil
}

// buildAIUpstreamProtocolOptions 构造只挂在模型 Service Cluster 上的上游过滤链
// Envoy 完成加权选路后才执行该链，因此 ExtProc 能读取最终端点元数据并转换厂商协议
func buildAIUpstreamProtocolOptions() (*anypb.Any, error) {
	configuration := &extprocv3.ExternalProcessor{
		GrpcService: aiExtProcGRPCService(),
		ProcessingMode: &extprocv3.ProcessingMode{
			RequestHeaderMode:  extprocv3.ProcessingMode_SEND,
			RequestBodyMode:    extprocv3.ProcessingMode_NONE,
			ResponseHeaderMode: extprocv3.ProcessingMode_SKIP,
			ResponseBodyMode:   extprocv3.ProcessingMode_NONE,
		},
		RequestAttributes: []string{
			aiprotocol.ServiceIDAttribute,
			aiprotocol.ServiceProtocolAttribute,
		},
		MetadataOptions:   aiExtProcMetadataOptions(),
		MessageTimeout:    durationpb.New(aiExtProcMessageTimeout),
		FailureModeAllow:  false,
		AllowModeOverride: true,
	}
	extProc, err := marshalAIExtProcConfig("upstream", configuration)
	if err != nil {
		return nil, err
	}
	codec, err := anypb.New(&upstreamcodecv3.UpstreamCodec{})
	if err != nil {
		return nil, fmt.Errorf("encode AI upstream codec filter: %w", err)
	}

	options := &upstreamhttpv3.HttpProtocolOptions{
		UpstreamProtocolOptions: &upstreamhttpv3.HttpProtocolOptions_ExplicitHttpConfig_{
			ExplicitHttpConfig: &upstreamhttpv3.HttpProtocolOptions_ExplicitHttpConfig{
				ProtocolConfig: &upstreamhttpv3.HttpProtocolOptions_ExplicitHttpConfig_HttpProtocolOptions{},
			},
		},
		HttpFilters: []*hcmv3.HttpFilter{
			{
				Name:       httpAIUpstreamExtProcFilterName,
				ConfigType: &hcmv3.HttpFilter_TypedConfig{TypedConfig: extProc},
			},
			{
				Name:       httpUpstreamCodecFilterName,
				ConfigType: &hcmv3.HttpFilter_TypedConfig{TypedConfig: codec},
			},
		},
	}
	if err := options.ValidateAll(); err != nil {
		return nil, fmt.Errorf("validate AI upstream HTTP options: %w", err)
	}
	typedOptions, err := anypb.New(options)
	if err != nil {
		return nil, fmt.Errorf("encode AI upstream HTTP options: %w", err)
	}
	return typedOptions, nil
}

func aiExtProcGRPCService() *corev3.GrpcService {
	return &corev3.GrpcService{
		TargetSpecifier: &corev3.GrpcService_EnvoyGrpc_{
			EnvoyGrpc: &corev3.GrpcService_EnvoyGrpc{ClusterName: aiExtProcClusterName},
		},
		// 不设置整条 gRPC 流超时；模型流式响应可能持续数分钟
		// 单次阶段交互仍由 ExternalProcessor.message_timeout 约束
	}
}

func aiExtProcMetadataOptions() *extprocv3.MetadataOptions {
	return &extprocv3.MetadataOptions{
		ReceivingNamespaces: &extprocv3.MetadataOptions_MetadataNamespaces{
			Untyped: []string{aiprotocol.MetadataNamespace},
		},
	}
}

func marshalAIExtProcConfig(stage string, configuration *extprocv3.ExternalProcessor) (*anypb.Any, error) {
	if err := configuration.ValidateAll(); err != nil {
		return nil, fmt.Errorf("validate AI %s ExtProc filter: %w", stage, err)
	}
	typedConfig, err := anypb.New(configuration)
	if err != nil {
		return nil, fmt.Errorf("encode AI %s ExtProc filter: %w", stage, err)
	}
	return typedConfig, nil
}
