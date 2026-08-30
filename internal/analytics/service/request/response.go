package request

import (
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	requestbiz "github.com/lgc202/ingate/internal/analytics/biz/request"
)

func summaryResponse(summary requestbiz.Summary) *analyticsv1.RequestSummary {
	response := &analyticsv1.RequestSummary{
		Id:                  summary.ID,
		StartedAt:           timestamppb.New(summary.StartedAt),
		Method:              summary.Method,
		Host:                summary.Host,
		Path:                summary.Path,
		StatusCode:          uint32(summary.StatusCode),
		GatewayId:           summary.GatewayID,
		RouteId:             summary.RouteID,
		UpstreamId:          summary.UpstreamID,
		CallerId:            summary.CallerID,
		AccessKeyId:         summary.AccessKeyID,
		AiModelCall:         modelCallResponse(summary.ModelCall),
		ResponseCodeDetails: summary.ResponseCodeDetails,
	}
	if summary.Duration != nil {
		response.Duration = durationpb.New(*summary.Duration)
	}
	return response
}

// recordResponse 只在 gRPC 边界还原 ALS 公共协议，biz 和 ClickHouse 不依赖该传输类型。
func recordResponse(record *requestbiz.Record) *alsv1.RequestRecord {
	response := &alsv1.RequestRecord{
		Id:                  record.ID,
		RequestId:           record.RequestID,
		StartedAt:           timestamppb.New(record.StartedAt),
		ClientIp:            record.ClientIP,
		Method:              record.Method,
		Host:                record.Host,
		Path:                record.Path,
		StatusCode:          uint32(record.StatusCode),
		RequestBytes:        record.RequestBytes,
		ResponseBytes:       record.ResponseBytes,
		GatewayId:           record.GatewayID,
		RouteId:             record.RouteID,
		UpstreamId:          record.UpstreamID,
		EnvoyNodeId:         record.EnvoyNodeID,
		Protocol:            record.Protocol,
		ResponseCodeDetails: record.ResponseCodeDetails,
		UpstreamAttempts:    uint32(record.UpstreamAttempts),
		UpstreamAddress:     record.UpstreamAddress,
		AiModelCall:         modelCallResponse(record.ModelCall),
		CallerId:            record.CallerID,
		AccessKeyId:         record.AccessKeyID,
	}
	if record.Duration != nil {
		response.Duration = durationpb.New(*record.Duration)
	}
	if record.TimeToFirstByte != nil {
		response.TimeToFirstByte = durationpb.New(*record.TimeToFirstByte)
	}
	return response
}

func modelCallResponse(call *requestbiz.ModelCall) *alsv1.AIModelCall {
	if call == nil {
		return nil
	}
	return &alsv1.AIModelCall{
		ClientModel:      call.ClientModel,
		UpstreamModel:    call.UpstreamModel,
		UpstreamProtocol: call.UpstreamProtocol,
		ResponseModel:    call.ResponseModel,
		FinishReason:     call.FinishReason,
		InputTokens:      call.InputTokens,
		OutputTokens:     call.OutputTokens,
		TotalTokens:      call.TotalTokens,
	}
}
