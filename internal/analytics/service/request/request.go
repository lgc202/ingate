package request

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	kerrors "github.com/go-kratos/kratos/v3/errors"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	requestbiz "github.com/lgc202/ingate/internal/analytics/biz/request"
	"github.com/lgc202/ingate/internal/pkg/analyticsconfig"
	"github.com/lgc202/ingate/internal/pkg/requestrecord"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
)

const (
	defaultPageSize    uint32 = 50
	maxPageSize        uint32 = 200
	maxPageTokenLength        = 512
)

type pageTokenValue struct {
	StartedAtNanoseconds *int64 `json:"started_at_ns"`
	ID                   string `json:"id"`
	FilterFingerprint    string `json:"filter"`
}

type pageTokenFilter struct {
	StartTimeNanoseconds int64   `json:"start_time_ns"`
	EndTimeNanoseconds   int64   `json:"end_time_ns"`
	GatewayID            string  `json:"gateway_id"`
	RouteID              string  `json:"route_id"`
	UpstreamID           string  `json:"upstream_id"`
	RequestID            string  `json:"request_id"`
	Method               string  `json:"method"`
	Host                 string  `json:"host"`
	PathPrefix           string  `json:"path_prefix"`
	StatusClass          uint8   `json:"status_class"`
	StatusCode           *uint16 `json:"status_code"`
	CallerID             string  `json:"caller_id"`
}

func buildListOptions(request *analyticsv1.ListRequestsRequest) (requestbiz.ListOptions, error) {
	filter, err := buildFilter(request.GetFilter())
	if err != nil {
		return requestbiz.ListOptions{}, err
	}
	pageSize := cmp.Or(request.GetPageSize(), defaultPageSize)
	if pageSize > maxPageSize {
		return requestbiz.ListOptions{}, invalidArgument("page_size exceeds maximum")
	}
	cursor, err := parsePageToken(request.GetPageToken(), filter)
	if err != nil {
		return requestbiz.ListOptions{}, invalidArgument("page_token is invalid")
	}
	return requestbiz.ListOptions{Filter: filter, PageSize: int(pageSize), Cursor: cursor}, nil
}

func buildFilter(filter *analyticsv1.RequestFilter) (requestbiz.Filter, error) {
	start := filter.GetStartTime()
	end := filter.GetEndTime()
	if start == nil || end == nil || start.CheckValid() != nil || end.CheckValid() != nil {
		return requestbiz.Filter{}, invalidArgument("start_time and end_time are required")
	}
	if !analyticsconfig.IsValidQueryRange(start.AsTime(), end.AsTime()) {
		return requestbiz.Filter{}, invalidArgument("time range is invalid or exceeds the maximum")
	}
	statusClass, err := buildStatusClass(filter.GetStatusClass())
	if err != nil {
		return requestbiz.Filter{}, err
	}
	var statusCode *uint16
	if filter.StatusCode != nil {
		requestedStatusCode := filter.GetStatusCode()
		if requestedStatusCode > 65535 ||
			(requestedStatusCode > 0 && requestedStatusCode < 100) {
			return requestbiz.Filter{}, invalidArgument("status_code is invalid")
		}
		value := uint16(requestedStatusCode)
		if statusClass != requestbiz.StatusClassUnknown &&
			requestbiz.ClassifyStatusCode(value) != statusClass {
			return requestbiz.Filter{}, invalidArgument("status_code does not match status_class")
		}
		statusCode = &value
	}
	for _, resourceID := range []string{
		filter.GetGatewayId(),
		filter.GetRouteId(),
		filter.GetUpstreamId(),
		filter.GetCallerId(),
	} {
		if resourceID != "" && !resourceconfig.IsCanonicalID(resourceID) {
			return requestbiz.Filter{}, invalidArgument("filter contains an invalid resource ID")
		}
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
		CallerID:    filter.GetCallerId(),
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
	case analyticsv1.StatusClass_STATUS_CLASS_NO_RESPONSE:
		return requestbiz.StatusClassNoResponse, nil
	default:
		return 0, invalidArgument("status_class is invalid")
	}
}

func formatPageToken(cursor *requestbiz.Cursor, filter requestbiz.Filter) (string, error) {
	if cursor == nil {
		return "", nil
	}
	if !requestrecord.IsValidID(cursor.ID) || !analyticsconfig.IsSupportedTime(cursor.StartedAt) {
		return "", errors.New("query returned an invalid page cursor")
	}
	fingerprint, err := filterFingerprint(filter)
	if err != nil {
		return "", err
	}
	startedAtNanoseconds := cursor.StartedAt.UnixNano()
	payload, err := json.Marshal(pageTokenValue{
		StartedAtNanoseconds: &startedAtNanoseconds,
		ID:                   cursor.ID,
		FilterFingerprint:    fingerprint,
	})
	if err != nil {
		return "", fmt.Errorf("marshal page token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func parsePageToken(value string, filter requestbiz.Filter) (*requestbiz.Cursor, error) {
	if value == "" {
		return nil, nil
	}
	if len(value) > maxPageTokenLength {
		return nil, fmt.Errorf("page token exceeds %d bytes", maxPageTokenLength)
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("parse page token encoding: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var token pageTokenValue
	if err := decoder.Decode(&token); err != nil {
		return nil, fmt.Errorf("parse page token value: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("page token contains trailing data")
	}
	if token.StartedAtNanoseconds == nil || !requestrecord.IsValidID(token.ID) {
		return nil, errors.New("page token contains invalid values")
	}
	fingerprint, err := filterFingerprint(filter)
	if err != nil {
		return nil, err
	}
	if token.FilterFingerprint != fingerprint {
		return nil, errors.New("page token does not match the current filter")
	}
	startedAt := time.Unix(0, *token.StartedAtNanoseconds).UTC()
	if startedAt.Before(filter.StartTime) || !startedAt.Before(filter.EndTime) {
		return nil, errors.New("page token is outside the current time range")
	}
	return &requestbiz.Cursor{StartedAt: startedAt, ID: token.ID}, nil
}

func filterFingerprint(filter requestbiz.Filter) (string, error) {
	encoded, err := json.Marshal(pageTokenFilter{
		StartTimeNanoseconds: filter.StartTime.UnixNano(),
		EndTimeNanoseconds:   filter.EndTime.UnixNano(),
		GatewayID:            filter.GatewayID,
		RouteID:              filter.RouteID,
		UpstreamID:           filter.UpstreamID,
		RequestID:            filter.RequestID,
		Method:               filter.Method,
		Host:                 filter.Host,
		PathPrefix:           filter.PathPrefix,
		StatusClass:          uint8(filter.StatusClass),
		StatusCode:           filter.StatusCode,
		CallerID:             filter.CallerID,
	})
	if err != nil {
		return "", fmt.Errorf("marshal page token filter: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func invalidArgument(message string) error {
	return kerrors.BadRequest("INVALID_ARGUMENT", message)
}
