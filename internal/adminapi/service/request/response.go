package request

import (
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	requestbiz "github.com/lgc202/ingate/internal/adminapi/biz/request"
)

func summaryResponse(summary *requestbiz.Summary) *adminv1.RequestRecordSummary {
	return &adminv1.RequestRecordSummary{
		Id:         summary.ID,
		StartedAt:  timestamppb.New(summary.StartedAt),
		Duration:   durationResponse(summary.Duration),
		Method:     summary.Method,
		Host:       summary.Host,
		Path:       summary.Path,
		StatusCode: summary.StatusCode,
		Outcome:    outcomeResponse(summary.Outcome),
		GatewayId:  summary.GatewayID,
		RouteId:    summary.RouteID,
		ServiceId:  summary.ServiceID,
	}
}

func recordResponse(record *requestbiz.Record) *adminv1.RequestRecord {
	return &adminv1.RequestRecord{
		Id:                  record.ID,
		RequestId:           record.RequestID,
		StartedAt:           timestamppb.New(record.StartedAt),
		Duration:            durationResponse(record.Duration),
		TimeToFirstByte:     durationResponse(record.TimeToFirstByte),
		ClientIp:            record.ClientIP,
		Method:              record.Method,
		Host:                record.Host,
		Path:                record.Path,
		StatusCode:          record.StatusCode,
		Outcome:             outcomeResponse(record.Outcome),
		RequestBytes:        record.RequestBytes,
		ResponseBytes:       record.ResponseBytes,
		GatewayId:           record.GatewayID,
		RouteId:             record.RouteID,
		ServiceId:           record.ServiceID,
		Protocol:            record.Protocol,
		ResponseCodeDetails: record.ResponseCodeDetails,
		UpstreamAttempts:    record.UpstreamAttempts,
		UpstreamAddress:     record.UpstreamAddress,
		ProxyInstanceId:     record.ProxyInstanceID,
	}
}

func durationResponse(value *time.Duration) *durationpb.Duration {
	if value == nil {
		return nil
	}
	return durationpb.New(*value)
}

func outcomeResponse(value requestbiz.Outcome) adminv1.RequestOutcome {
	switch value {
	case requestbiz.OutcomeSuccess:
		return adminv1.RequestOutcome_REQUEST_OUTCOME_SUCCESS
	case requestbiz.OutcomeClientError:
		return adminv1.RequestOutcome_REQUEST_OUTCOME_CLIENT_ERROR
	case requestbiz.OutcomeServerError:
		return adminv1.RequestOutcome_REQUEST_OUTCOME_SERVER_ERROR
	case requestbiz.OutcomeNoResponse:
		return adminv1.RequestOutcome_REQUEST_OUTCOME_NO_RESPONSE
	default:
		return adminv1.RequestOutcome_REQUEST_OUTCOME_UNSPECIFIED
	}
}
