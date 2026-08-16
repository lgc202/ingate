package analytics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	requestbiz "github.com/lgc202/ingate/internal/adminapi/biz/request"
)

// RequestRepository 通过 Analytics gRPC 查询请求明细
type RequestRepository struct {
	client analyticsv1.RequestServiceClient
}

// NewRequestRepository 创建请求记录 Repository
func NewRequestRepository(connection *grpc.ClientConn) *RequestRepository {
	return &RequestRepository{client: analyticsv1.NewRequestServiceClient(connection)}
}

// List 查询请求记录并把内部 Analytics 协议转换为 Admin API 业务类型
func (r *RequestRepository) List(ctx context.Context, options requestbiz.ListOptions) (requestbiz.Page, error) {
	filter := options.Filter
	request := &analyticsv1.ListRequestsRequest{
		Filter: &analyticsv1.RequestFilter{
			StartTime:   timestamppb.New(filter.StartTime),
			EndTime:     timestamppb.New(filter.EndTime),
			GatewayId:   filter.GatewayID,
			RouteId:     filter.RouteID,
			UpstreamId:  filter.ServiceID,
			RequestId:   filter.RequestID,
			Method:      filter.Method,
			Host:        filter.Host,
			PathPrefix:  filter.PathPrefix,
			StatusClass: analyticsStatusClass(filter.Outcome),
		},
		PageSize:  uint32(options.PageSize),
		PageToken: options.PageToken,
	}
	if filter.StatusCode != nil {
		value := uint32(*filter.StatusCode)
		request.Filter.StatusCode = &value
	}
	reply, err := r.client.ListRequests(ctx, request)
	if status.Code(err) == codes.Unavailable || status.Code(err) == codes.DeadlineExceeded {
		return requestbiz.Page{}, fmt.Errorf("%w: %w", requestbiz.ErrUnavailable, err)
	}
	if err != nil {
		return requestbiz.Page{}, fmt.Errorf("list analytics request records: %w", err)
	}
	records := make([]requestbiz.Summary, 0, len(reply.GetRequests()))
	for _, item := range reply.GetRequests() {
		record, err := requestSummary(item)
		if err != nil {
			return requestbiz.Page{}, err
		}
		records = append(records, record)
	}
	return requestbiz.Page{Records: records, NextPageToken: reply.GetNextPageToken()}, nil
}

func requestSummary(summary *analyticsv1.RequestSummary) (requestbiz.Summary, error) {
	if summary == nil || summary.GetId() == "" || summary.GetStartedAt() == nil || summary.GetStartedAt().CheckValid() != nil {
		return requestbiz.Summary{}, errors.New("analytics returned an invalid request summary")
	}
	duration, err := optionalDuration(summary.GetDuration())
	if err != nil {
		return requestbiz.Summary{}, fmt.Errorf("invalid request duration: %w", err)
	}
	return requestbiz.Summary{
		ID:         summary.GetId(),
		StartedAt:  summary.GetStartedAt().AsTime(),
		Duration:   duration,
		Method:     summary.GetMethod(),
		Host:       summary.GetHost(),
		Path:       summary.GetPath(),
		StatusCode: summary.GetStatusCode(),
		Outcome:    requestOutcome(summary.GetStatusCode()),
		GatewayID:  summary.GetGatewayId(),
		RouteID:    summary.GetRouteId(),
		ServiceID:  summary.GetUpstreamId(),
	}, nil
}

// Get 查询单次请求记录
func (r *RequestRepository) Get(
	ctx context.Context,
	id string,
	startedAt time.Time,
) (*requestbiz.Record, error) {
	reply, err := r.client.GetRequest(ctx, &analyticsv1.GetRequestRequest{
		Id:        id,
		StartedAt: timestamppb.New(startedAt),
	})
	if status.Code(err) == codes.NotFound {
		return nil, requestbiz.ErrNotFound
	}
	if status.Code(err) == codes.Unavailable || status.Code(err) == codes.DeadlineExceeded {
		return nil, fmt.Errorf("%w: %w", requestbiz.ErrUnavailable, err)
	}
	if err != nil {
		return nil, fmt.Errorf("get analytics request record %q: %w", id, err)
	}
	record, err := requestRecord(reply)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func requestRecord(record *alsv1.RequestRecord) (requestbiz.Record, error) {
	if record == nil || record.GetId() == "" || record.GetStartedAt() == nil || record.GetStartedAt().CheckValid() != nil {
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
	}, nil
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
