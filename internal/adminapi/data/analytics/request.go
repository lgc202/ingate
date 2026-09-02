package analytics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/apperror"
	requestbiz "github.com/lgc202/ingate/internal/adminapi/biz/request"
)

// RequestRepository 通过 Analytics gRPC 查询请求明细。
type RequestRepository struct {
	client analyticsv1.RequestServiceClient
}

// NewRequestRepository 创建请求记录 Repository。
func NewRequestRepository(connection *grpc.ClientConn) *RequestRepository {
	return &RequestRepository{client: analyticsv1.NewRequestServiceClient(connection)}
}

// List 查询请求记录并把内部 Analytics 协议转换为 Admin API 业务类型。
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
			CallerId:    filter.CallerID,
		},
		PageSize:  uint32(options.PageSize),
		PageToken: options.PageToken,
	}
	if filter.StatusCode != nil {
		value := uint32(*filter.StatusCode)
		request.Filter.StatusCode = &value
	}
	reply, err := r.client.ListRequests(ctx, request)
	if err != nil && ctx.Err() != nil {
		return requestbiz.Page{}, ctx.Err()
	}
	if status.Code(err) == codes.InvalidArgument && options.PageToken != "" {
		return requestbiz.Page{}, apperror.InvalidCursor(err)
	}
	if isUnavailable(ctx, err) {
		return requestbiz.Page{}, requestbiz.Unavailable(err)
	}
	if err != nil {
		return requestbiz.Page{}, fmt.Errorf("list analytics request records: %w", err)
	}
	if reply == nil {
		return requestbiz.Page{}, errors.New("analytics returned an empty request list response")
	}
	items := reply.GetRequests()
	if len(items) > options.PageSize {
		return requestbiz.Page{}, errors.New("analytics returned too many request records")
	}
	records := make([]requestbiz.Summary, len(items))
	for i, item := range items {
		summary, err := requestSummary(item)
		if err != nil {
			return requestbiz.Page{}, fmt.Errorf("convert analytics request summary: %w", err)
		}
		records[i] = summary
	}
	return requestbiz.Page{Records: records, NextPageToken: reply.GetNextPageToken()}, nil
}

// Get 查询单次请求记录。
func (r *RequestRepository) Get(
	ctx context.Context,
	recordID string,
	startedAt time.Time,
) (*requestbiz.Record, error) {
	reply, err := r.client.GetRequest(ctx, &analyticsv1.GetRequestRequest{
		Id:        recordID,
		StartedAt: timestamppb.New(startedAt),
	})
	if err != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if status.Code(err) == codes.NotFound {
		return nil, requestbiz.ErrNotFound
	}
	if isUnavailable(ctx, err) {
		return nil, requestbiz.Unavailable(err)
	}
	if err != nil {
		return nil, fmt.Errorf("get analytics request record %q: %w", recordID, err)
	}
	if reply == nil {
		return nil, errors.New("analytics returned an empty request record response")
	}
	record, err := requestRecord(reply)
	if err != nil {
		return nil, fmt.Errorf("convert analytics request record: %w", err)
	}
	if record.ID != recordID || !record.StartedAt.Equal(startedAt) {
		return nil, errors.New("analytics returned a different request record")
	}
	return &record, nil
}
