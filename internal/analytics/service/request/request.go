package request

import (
	"encoding/base64"
	"encoding/json"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	requestbiz "github.com/lgc202/ingate/internal/analytics/biz/request"
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

// pageTokenPayload 是管理面不可解释的分页 Token 内部载荷
//
// StartedAt 和 ID 对应 ClickHouse 的倒序排序键，避免 OFFSET 深分页扫描
type pageTokenPayload struct {
	StartedAt int64  `json:"started_at"`
	ID        string `json:"id"`
}

func buildListOptions(request *analyticsv1.ListRequestsRequest) (requestbiz.ListOptions, error) {
	filter, err := buildFilter(request.GetFilter())
	if err != nil {
		return requestbiz.ListOptions{}, err
	}
	pageSize := int(request.GetPageSize())
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		return requestbiz.ListOptions{}, invalidArgument("page_size exceeds maximum")
	}
	cursor, err := decodePageToken(request.GetPageToken())
	if err != nil {
		return requestbiz.ListOptions{}, invalidArgument("page_token is invalid")
	}
	return requestbiz.ListOptions{Filter: filter, PageSize: pageSize, Cursor: cursor}, nil
}

func buildFilter(filter *analyticsv1.RequestFilter) (requestbiz.Filter, error) {
	start := filter.GetStartTime()
	end := filter.GetEndTime()
	if start == nil || end == nil || start.CheckValid() != nil || end.CheckValid() != nil {
		return requestbiz.Filter{}, invalidArgument("start_time and end_time are required")
	}
	if !start.AsTime().Before(end.AsTime()) {
		return requestbiz.Filter{}, invalidArgument("start_time must be before end_time")
	}
	statusClass, err := buildStatusClass(filter.GetStatusClass())
	if err != nil {
		return requestbiz.Filter{}, err
	}
	var statusCode *uint16
	if filter.StatusCode != nil {
		if filter.GetStatusCode() > 65535 {
			return requestbiz.Filter{}, invalidArgument("status_code is invalid")
		}
		value := uint16(filter.GetStatusCode())
		statusCode = &value
	}
	return requestbiz.Filter{
		StartTime:   start.AsTime(),
		EndTime:     end.AsTime(),
		GatewayID:   filter.GetGatewayId(),
		RouteID:     filter.GetRouteId(),
		UpstreamID:  filter.GetUpstreamId(),
		RequestID:   filter.GetRequestId(),
		Method:      filter.GetMethod(),
		Host:        filter.GetHost(),
		PathPrefix:  filter.GetPathPrefix(),
		StatusClass: statusClass,
		StatusCode:  statusCode,
	}, nil
}

func buildStatusClass(value analyticsv1.StatusClass) (requestbiz.StatusClass, error) {
	switch value {
	case analyticsv1.StatusClass_STATUS_CLASS_UNSPECIFIED:
		return requestbiz.StatusClassUnknown, nil
	case analyticsv1.StatusClass_STATUS_CLASS_SUCCESS:
		return requestbiz.StatusClassSuccess, nil
	case analyticsv1.StatusClass_STATUS_CLASS_CLIENT_ERROR:
		return requestbiz.StatusClassClientError, nil
	case analyticsv1.StatusClass_STATUS_CLASS_SERVER_ERROR:
		return requestbiz.StatusClassServerError, nil
	default:
		return 0, invalidArgument("status_class is invalid")
	}
}

func encodePageToken(cursor *requestbiz.Cursor) (string, error) {
	if cursor == nil {
		return "", nil
	}
	payload, err := json.Marshal(pageTokenPayload{StartedAt: cursor.StartedAt.UnixNano(), ID: cursor.ID})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodePageToken(token string) (*requestbiz.Cursor, error) {
	if token == "" {
		return nil, nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}
	var decoded pageTokenPayload
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, err
	}
	if decoded.ID == "" {
		return nil, invalidArgument("page_token is invalid")
	}
	return &requestbiz.Cursor{StartedAt: time.Unix(0, decoded.StartedAt).UTC(), ID: decoded.ID}, nil
}

func invalidArgument(message string) error {
	return kratoserrors.BadRequest("INVALID_ARGUMENT", message)
}
