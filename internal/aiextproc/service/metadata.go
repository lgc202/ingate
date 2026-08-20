package service

import (
	"encoding/json"
	"strconv"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/lgc202/ingate/internal/aiextproc/filterconfig"
)

type openAIErrorBody struct {
	Error openAIError `json:"error"`
}

type openAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// dynamicMetadata 把模型映射和响应用量写入统一的 Envoy 元数据命名空间
// 后续 ALS 可以从访问日志读取这些字段，而不需要再次解析模型响应正文
func (s *streamState) dynamicMetadata() *structpb.Struct {
	request, _ := s.request.requestMetadata()
	fields := map[string]*structpb.Value{
		"client_model": structpb.NewStringValue(request.Model),
	}
	clientHost, clientPath := s.request.clientRequest()
	if clientHost != "" {
		fields[filterconfig.ClientHostField] = structpb.NewStringValue(clientHost)
	}
	if clientPath != "" {
		fields[filterconfig.ClientPathField] = structpb.NewStringValue(clientPath)
	}
	if selected, ok := s.request.selectedService(); ok {
		fields["service_id"] = structpb.NewStringValue(selected.ID)
		fields["upstream_model"] = structpb.NewStringValue(selected.Model)
		fields["upstream_protocol"] = structpb.NewStringValue(string(selected.Protocol))
	}
	if s.responseMetadata.ResponseModel != "" {
		fields["response_model"] = structpb.NewStringValue(s.responseMetadata.ResponseModel)
	}
	if s.responseMetadata.FinishReason != "" {
		fields["finish_reason"] = structpb.NewStringValue(s.responseMetadata.FinishReason)
	}
	if s.responseMetadata.Usage.Found {
		// Found 区分“厂商明确返回零用量”和“响应中没有可用的用量字段”
		fields["input_tokens"] = structpb.NewNumberValue(float64(s.responseMetadata.Usage.InputTokens))
		fields["output_tokens"] = structpb.NewNumberValue(float64(s.responseMetadata.Usage.OutputTokens))
		fields["total_tokens"] = structpb.NewNumberValue(float64(s.responseMetadata.Usage.TotalTokens))
	}
	return &structpb.Struct{
		Fields: map[string]*structpb.Value{
			filterconfig.MetadataNamespace: structpb.NewStructValue(&structpb.Struct{Fields: fields}),
		},
	}
}

// invalidRequestResponse 返回 OpenAI 兼容错误，使现有 AI 客户端无需理解 ExtProc 或 gRPC
func invalidRequestResponse(message string) *extprocv3.ProcessingResponse {
	// 结构由固定字段组成，json.Marshal 在这里不会遇到不支持的值
	body, _ := json.Marshal(openAIErrorBody{
		Error: openAIError{
			Message: message,
			Type:    "invalid_request_error",
			Code:    "invalid_request",
		},
	})
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ImmediateResponse{
			ImmediateResponse: &extprocv3.ImmediateResponse{
				Status: &typev3.HttpStatus{Code: typev3.StatusCode_BadRequest},
				Headers: &extprocv3.HeaderMutation{
					SetHeaders: []*corev3.HeaderValueOption{
						setHeader("content-type", "application/json"),
						setHeader("content-length", strconv.Itoa(len(body))),
					},
				},
				Body:       body,
				GrpcStatus: &extprocv3.GrpcStatus{Status: uint32(codes.InvalidArgument)},
				Details:    "ingate_ai_invalid_request",
			},
		},
	}
}
