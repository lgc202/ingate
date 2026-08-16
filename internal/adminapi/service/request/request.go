package request

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	requestbiz "github.com/lgc202/ingate/internal/adminapi/biz/request"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

const defaultPageSize = 50

func listOptions(request *adminv1.ListRequestRecordsRequest) (requestbiz.ListOptions, error) {
	startTime, err := requiredTimestamp(request.GetStartTime(), "请选择查询开始时间")
	if err != nil {
		return requestbiz.ListOptions{}, err
	}
	endTime, err := requiredTimestamp(request.GetEndTime(), "请选择查询结束时间")
	if err != nil {
		return requestbiz.ListOptions{}, err
	}
	if !startTime.Before(endTime) {
		return requestbiz.ListOptions{}, adminservice.BadRequest("查询开始时间必须早于结束时间")
	}
	pageSize := int(request.GetPageSize())
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	var statusCode *uint16
	if request.StatusCode != nil {
		value := uint16(request.GetStatusCode())
		statusCode = &value
	}
	return requestbiz.ListOptions{
		Filter: requestbiz.Filter{
			StartTime:  startTime,
			EndTime:    endTime,
			GatewayID:  request.GetGatewayId(),
			RouteID:    request.GetRouteId(),
			ServiceID:  request.GetServiceId(),
			RequestID:  request.GetRequestId(),
			Method:     request.GetMethod(),
			Host:       request.GetHost(),
			PathPrefix: request.GetPathPrefix(),
			Outcome:    requestOutcome(request.GetOutcome()),
			StatusCode: statusCode,
		},
		PageSize:  pageSize,
		PageToken: request.GetPageToken(),
	}, nil
}

func requiredTimestamp(value *timestamppb.Timestamp, userMessage string) (time.Time, error) {
	if value == nil || value.CheckValid() != nil {
		return time.Time{}, adminservice.BadRequest(userMessage)
	}
	return value.AsTime(), nil
}

func requestOutcome(value adminv1.RequestOutcome) requestbiz.Outcome {
	switch value {
	case adminv1.RequestOutcome_REQUEST_OUTCOME_SUCCESS:
		return requestbiz.OutcomeSuccess
	case adminv1.RequestOutcome_REQUEST_OUTCOME_CLIENT_ERROR:
		return requestbiz.OutcomeClientError
	case adminv1.RequestOutcome_REQUEST_OUTCOME_SERVER_ERROR:
		return requestbiz.OutcomeServerError
	default:
		return requestbiz.OutcomeUnknown
	}
}
