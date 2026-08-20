// Package request 实现请求明细查询的 gRPC 协议转换
//
// 该层只负责请求校验、分页 Token 和 Proto 转换，查询语义由 biz/request 承载
package request

import (
	"context"
	"errors"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	requestbiz "github.com/lgc202/ingate/internal/analytics/biz/request"
)

// Service 实现 Analytics RequestService gRPC API
type Service struct {
	queries *requestbiz.Queries
}

// NewService 创建请求明细查询服务
func NewService(queries *requestbiz.Queries) *Service {
	return &Service{queries: queries}
}

// ListRequests 按时间倒序分页查询请求摘要
func (s *Service) ListRequests(
	ctx context.Context,
	request *analyticsv1.ListRequestsRequest,
) (*analyticsv1.ListRequestsResponse, error) {
	options, err := buildListOptions(request)
	if err != nil {
		return nil, err
	}
	page, err := s.queries.List(ctx, options)
	if err != nil {
		return nil, err
	}
	nextPageToken, err := encodePageToken(page.NextCursor)
	if err != nil {
		return nil, kratoserrors.InternalServer("ENCODE_PAGE_TOKEN", "encode page token failed")
	}
	requests := make([]*analyticsv1.RequestSummary, 0, len(page.Records))
	for i := range page.Records {
		requests = append(requests, summaryResponse(page.Records[i]))
	}
	return &analyticsv1.ListRequestsResponse{
		Requests:      requests,
		NextPageToken: nextPageToken,
	}, nil
}

func summaryResponse(summary requestbiz.Summary) *analyticsv1.RequestSummary {
	response := &analyticsv1.RequestSummary{
		Id:          summary.ID,
		StartedAt:   timestamppb.New(summary.StartedAt),
		Method:      summary.Method,
		Host:        summary.Host,
		Path:        summary.Path,
		StatusCode:  summary.StatusCode,
		GatewayId:   summary.GatewayID,
		RouteId:     summary.RouteID,
		UpstreamId:  summary.UpstreamID,
		CallerId:    summary.CallerID,
		AccessKeyId: summary.AccessKeyID,
		AiModelCall: modelCallResponse(summary.ModelCall),
	}
	if summary.Duration != nil {
		response.Duration = durationpb.New(*summary.Duration)
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

// GetRequest 使用记录 ID 和开始时间查询单次请求明细
func (s *Service) GetRequest(
	ctx context.Context,
	request *analyticsv1.GetRequestRequest,
) (*alsv1.RequestRecord, error) {
	startedAt := request.GetStartedAt()
	if request.GetId() == "" || startedAt == nil || startedAt.CheckValid() != nil {
		return nil, kratoserrors.BadRequest("INVALID_ARGUMENT", "id and started_at are required")
	}
	record, err := s.queries.Get(ctx, request.GetId(), startedAt.AsTime())
	if errors.Is(err, requestbiz.ErrNotFound) {
		return nil, kratoserrors.NotFound("REQUEST_NOT_FOUND", "request record not found")
	}
	return record, err
}
