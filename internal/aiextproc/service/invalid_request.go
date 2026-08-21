package service

import (
	"encoding/json"
	"strconv"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/grpc/codes"
)

type openAIErrorBody struct {
	Error openAIError `json:"error"`
}

type openAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
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
