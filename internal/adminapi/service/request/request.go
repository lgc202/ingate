package request

import (
	"cmp"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	requestbiz "github.com/lgc202/ingate/internal/adminapi/biz/request"
	"github.com/lgc202/ingate/internal/pkg/analyticsconfig"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
)

const defaultPageSize = 50

func listOptions(request *adminv1.ListRequestRecordsRequest) (requestbiz.ListOptions, error) {
	filter, err := listFilter(request)
	if err != nil {
		return requestbiz.ListOptions{}, err
	}
	pageSize := cmp.Or(int(request.GetPageSize()), defaultPageSize)
	return requestbiz.ListOptions{
		Filter:    filter,
		PageSize:  pageSize,
		PageToken: request.GetPageToken(),
	}, nil
}

func listFilter(request *adminv1.ListRequestRecordsRequest) (requestbiz.Filter, error) {
	startTime, err := requiredTimestamp(request.GetStartTime(), "请选择查询开始时间")
	if err != nil {
		return requestbiz.Filter{}, err
	}
	endTime, err := requiredTimestamp(request.GetEndTime(), "请选择查询结束时间")
	if err != nil {
		return requestbiz.Filter{}, err
	}
	if !startTime.Before(endTime) {
		return requestbiz.Filter{}, adminv1.ErrorInvalidArgument("查询开始时间必须早于结束时间")
	}
	if !analyticsconfig.IsValidQueryRange(startTime, endTime) {
		return requestbiz.Filter{}, adminv1.ErrorInvalidArgument("单次最多查询 90 天请求记录")
	}
	if err := validateResourceFilters(request); err != nil {
		return requestbiz.Filter{}, err
	}
	outcome := requestOutcome(request.GetOutcome())
	statusCode, err := requestStatusCode(request, outcome)
	if err != nil {
		return requestbiz.Filter{}, err
	}
	return requestbiz.Filter{
		StartTime:  startTime,
		EndTime:    endTime,
		GatewayID:  request.GetGatewayId(),
		RouteID:    request.GetRouteId(),
		ServiceID:  request.GetServiceId(),
		RequestID:  request.GetRequestId(),
		Method:     request.GetMethod(),
		Host:       request.GetHost(),
		PathPrefix: request.GetPathPrefix(),
		Outcome:    outcome,
		StatusCode: statusCode,
		CallerID:   request.GetCallerId(),
	}, nil
}

func validateResourceFilters(request *adminv1.ListRequestRecordsRequest) error {
	for _, resourceID := range []string{
		request.GetGatewayId(),
		request.GetRouteId(),
		request.GetServiceId(),
		request.GetCallerId(),
	} {
		if resourceID != "" && !resourceconfig.IsCanonicalID(resourceID) {
			return adminv1.ErrorInvalidArgument("资源筛选条件无效")
		}
	}
	return nil
}

func requestStatusCode(
	request *adminv1.ListRequestRecordsRequest,
	outcome requestbiz.Outcome,
) (*uint16, error) {
	if request.StatusCode == nil {
		return nil, nil
	}
	requestedStatusCode := request.GetStatusCode()
	if requestedStatusCode > 65535 ||
		(requestedStatusCode > 0 && requestedStatusCode < 100) {
		return nil, adminv1.ErrorInvalidArgument("HTTP 状态码无效")
	}
	if outcome != requestbiz.OutcomeUnknown &&
		requestbiz.ClassifyStatusCode(requestedStatusCode) != outcome {
		return nil, adminv1.ErrorInvalidArgument("HTTP 状态码与请求结果不一致")
	}
	return new(uint16(requestedStatusCode)), nil
}

func requiredTimestamp(value *timestamppb.Timestamp, userMessage string) (time.Time, error) {
	if value == nil || value.CheckValid() != nil {
		return time.Time{}, adminv1.ErrorInvalidArgument("%s", userMessage)
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
	case adminv1.RequestOutcome_REQUEST_OUTCOME_NO_RESPONSE:
		return requestbiz.OutcomeNoResponse
	default:
		return requestbiz.OutcomeUnknown
	}
}
