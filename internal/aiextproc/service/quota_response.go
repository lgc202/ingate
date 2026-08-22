package service

import (
	"encoding/json"
	"strconv"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/grpc/codes"

	"github.com/lgc202/ingate/internal/aiextproc/biz/tokenquota"
)

func (s *streamState) quotaExceededResponse(exceeded *tokenquota.Exceeded) *extprocv3.ProcessingResponse {
	body, _ := json.Marshal(openAIErrorBody{Error: openAIError{
		Message: quotaExceededMessage(exceeded.Period),
		Type:    "rate_limit_error",
		Code:    "token_quota_exceeded",
	}})
	retryAfter := max(int64(time.Until(exceeded.ResetAt).Seconds()), 1)
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ImmediateResponse{
			ImmediateResponse: &extprocv3.ImmediateResponse{
				Status: &typev3.HttpStatus{Code: typev3.StatusCode_TooManyRequests},
				Headers: &extprocv3.HeaderMutation{SetHeaders: []*corev3.HeaderValueOption{
					setHeader("content-type", "application/json"),
					setHeader("content-length", strconv.Itoa(len(body))),
					setHeader("retry-after", strconv.FormatInt(retryAfter, 10)),
				}},
				Body:       body,
				GrpcStatus: &extprocv3.GrpcStatus{Status: uint32(codes.ResourceExhausted)},
				Details:    "ingate_ai_token_quota_exceeded",
			},
		},
		// 保留客户端模型等已有元数据；拒绝原因通过 response_code_details 进入请求记录
		DynamicMetadata: s.dynamicMetadata(),
	}
}

func quotaExceededMessage(period tokenquota.Period) string {
	switch period {
	case tokenquota.PeriodDay:
		return "Daily token quota exceeded. Try again after the quota resets."
	case tokenquota.PeriodWeek:
		return "Weekly token quota exceeded. Try again after the quota resets."
	case tokenquota.PeriodMonth:
		return "Monthly token quota exceeded. Try again after the quota resets."
	default:
		return "Token quota exceeded. Try again after the quota resets."
	}
}
