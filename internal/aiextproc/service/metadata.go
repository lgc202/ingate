package service

import (
	"google.golang.org/protobuf/types/known/structpb"

	aiprotocol "github.com/lgc202/ingate/internal/pkg/aiextproc"
)

// dynamicMetadata 把模型映射和响应用量写入统一的 Envoy 元数据命名空间
// 后续 ALS 可以从访问日志读取这些字段，而不需要再次解析模型响应正文
func (s *streamState) dynamicMetadata() *structpb.Struct {
	request, _ := s.request.requestMetadata()
	fields := map[string]*structpb.Value{
		aiprotocol.ClientModelField: structpb.NewStringValue(request.Model),
	}
	clientHost, clientPath := s.request.clientRequest()
	if clientHost != "" {
		fields[aiprotocol.ClientHostField] = structpb.NewStringValue(clientHost)
	}
	if clientPath != "" {
		fields[aiprotocol.ClientPathField] = structpb.NewStringValue(clientPath)
	}
	if selected, ok := s.request.selectedService(); ok {
		fields[aiprotocol.ServiceIDField] = structpb.NewStringValue(selected.id)
		fields[aiprotocol.UpstreamModelField] = structpb.NewStringValue(selected.model)
		fields[aiprotocol.UpstreamProtocolField] = structpb.NewStringValue(string(selected.protocol))
	}
	if s.responseMetadata.ResponseModel != "" {
		fields[aiprotocol.ResponseModelField] = structpb.NewStringValue(s.responseMetadata.ResponseModel)
	}
	if s.responseMetadata.FinishReason != "" {
		fields[aiprotocol.FinishReasonField] = structpb.NewStringValue(s.responseMetadata.FinishReason)
	}
	if s.responseMetadata.Usage.Found {
		// Found 区分“厂商明确返回零用量”和“响应中没有可用的用量字段”
		fields[aiprotocol.InputTokensField] = structpb.NewNumberValue(float64(s.responseMetadata.Usage.InputTokens))
		fields[aiprotocol.OutputTokensField] = structpb.NewNumberValue(float64(s.responseMetadata.Usage.OutputTokens))
		fields[aiprotocol.TotalTokensField] = structpb.NewNumberValue(float64(s.responseMetadata.Usage.TotalTokens))
	}
	return &structpb.Struct{
		Fields: map[string]*structpb.Value{
			aiprotocol.MetadataNamespace: structpb.NewStructValue(&structpb.Struct{Fields: fields}),
		},
	}
}
