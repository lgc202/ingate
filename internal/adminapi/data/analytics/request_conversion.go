package analytics

import (
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	requestbiz "github.com/lgc202/ingate/internal/adminapi/biz/request"
)

func requestSummary(summary *analyticsv1.RequestSummary) (requestbiz.Summary, error) {
	if summary == nil || !validRequestIdentity(summary.GetId(), summary.GetStartedAt()) {
		return requestbiz.Summary{}, errors.New("analytics returned an invalid request summary")
	}
	duration, err := optionalDuration(summary.GetDuration())
	if err != nil {
		return requestbiz.Summary{}, fmt.Errorf("invalid request duration: %w", err)
	}
	return requestbiz.Summary{
		ID:          summary.GetId(),
		StartedAt:   summary.GetStartedAt().AsTime(),
		Duration:    duration,
		Method:      summary.GetMethod(),
		Host:        summary.GetHost(),
		Path:        summary.GetPath(),
		StatusCode:  summary.GetStatusCode(),
		Outcome:     requestOutcome(summary.GetStatusCode()),
		GatewayID:   summary.GetGatewayId(),
		RouteID:     summary.GetRouteId(),
		ServiceID:   summary.GetUpstreamId(),
		CallerID:    summary.GetCallerId(),
		AccessKeyID: summary.GetAccessKeyId(),
		ModelCall:   modelCall(summary.GetAiModelCall()),
	}, nil
}

func requestRecord(record *alsv1.RequestRecord) (requestbiz.Record, error) {
	if record == nil || !validRequestIdentity(record.GetId(), record.GetStartedAt()) {
		return requestbiz.Record{}, errors.New("analytics returned an invalid request record")
	}
	duration, err := optionalDuration(record.GetDuration())
	if err != nil {
		return requestbiz.Record{}, fmt.Errorf("invalid request duration: %w", err)
	}
	timeToFirstByte, err := optionalDuration(record.GetTimeToFirstByte())
	if err != nil {
		return requestbiz.Record{}, fmt.Errorf("invalid request time to first byte: %w", err)
	}
	return requestbiz.Record{
		ID:                  record.GetId(),
		RequestID:           record.GetRequestId(),
		StartedAt:           record.GetStartedAt().AsTime(),
		Duration:            duration,
		TimeToFirstByte:     timeToFirstByte,
		ClientIP:            record.GetClientIp(),
		Method:              record.GetMethod(),
		Host:                record.GetHost(),
		Path:                record.GetPath(),
		StatusCode:          record.GetStatusCode(),
		Outcome:             requestOutcome(record.GetStatusCode()),
		RequestBytes:        record.GetRequestBytes(),
		ResponseBytes:       record.GetResponseBytes(),
		GatewayID:           record.GetGatewayId(),
		RouteID:             record.GetRouteId(),
		ServiceID:           record.GetUpstreamId(),
		Protocol:            record.GetProtocol(),
		ResponseCodeDetails: record.GetResponseCodeDetails(),
		UpstreamAttempts:    record.GetUpstreamAttempts(),
		UpstreamAddress:     record.GetUpstreamAddress(),
		ProxyInstanceID:     record.GetEnvoyNodeId(),
		CallerID:            record.GetCallerId(),
		AccessKeyID:         record.GetAccessKeyId(),
		ModelCall:           modelCall(record.GetAiModelCall()),
	}, nil
}

func validRequestIdentity(id string, startedAt *timestamppb.Timestamp) bool {
	return id != "" && startedAt != nil && startedAt.CheckValid() == nil
}

func modelCall(call *alsv1.AIModelCall) *requestbiz.ModelCall {
	if call == nil {
		return nil
	}
	return &requestbiz.ModelCall{
		ClientModel:      call.GetClientModel(),
		UpstreamModel:    call.GetUpstreamModel(),
		UpstreamProtocol: call.GetUpstreamProtocol(),
		ResponseModel:    call.GetResponseModel(),
		FinishReason:     call.GetFinishReason(),
		InputTokens:      optionalUint64(call.InputTokens),
		OutputTokens:     optionalUint64(call.OutputTokens),
		TotalTokens:      optionalUint64(call.TotalTokens),
	}
}

// optionalDuration 校验跨进程返回的 Proto Duration，并保留字段未采集与零耗时的区别
func optionalDuration(value *durationpb.Duration) (*time.Duration, error) {
	if value == nil {
		return nil, nil
	}
	if err := value.CheckValid(); err != nil {
		return nil, err
	}
	duration := value.AsDuration()
	return &duration, nil
}

// optionalUint64 复制 Proto 可选标量，避免业务对象继续共享生成消息的可变字段
func optionalUint64(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func requestOutcome(statusCode uint32) requestbiz.Outcome {
	switch {
	case statusCode >= 500:
		return requestbiz.OutcomeServerError
	case statusCode >= 400:
		return requestbiz.OutcomeClientError
	case statusCode >= 100:
		return requestbiz.OutcomeSuccess
	default:
		return requestbiz.OutcomeNoResponse
	}
}

func analyticsStatusClass(outcome requestbiz.Outcome) analyticsv1.StatusClass {
	switch outcome {
	case requestbiz.OutcomeSuccess:
		return analyticsv1.StatusClass_STATUS_CLASS_SUCCESS
	case requestbiz.OutcomeClientError:
		return analyticsv1.StatusClass_STATUS_CLASS_CLIENT_ERROR
	case requestbiz.OutcomeServerError:
		return analyticsv1.StatusClass_STATUS_CLASS_SERVER_ERROR
	case requestbiz.OutcomeNoResponse:
		return analyticsv1.StatusClass_STATUS_CLASS_NO_RESPONSE
	default:
		return analyticsv1.StatusClass_STATUS_CLASS_UNSPECIFIED
	}
}
