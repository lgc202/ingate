// Package request 实现请求明细查询的 gRPC 协议转换。
//
// 该层只负责请求校验、分页 Token 和 Proto 转换，查询语义由 biz/request 承载。
package request

import (
	"context"
	"errors"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/samber/lo"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	analyticsv1 "github.com/lgc202/ingate/api/analytics/v1"
	requestbiz "github.com/lgc202/ingate/internal/analytics/biz/request"
	"github.com/lgc202/ingate/internal/pkg/analyticsconfig"
	"github.com/lgc202/ingate/internal/pkg/requestrecord"
)

// Service 实现 Analytics RequestService gRPC API。
type Service struct {
	query *requestbiz.Query
}

// NewService 创建请求明细查询服务。
func NewService(query *requestbiz.Query) *Service {
	return &Service{query: query}
}

// ListRequests 按时间倒序分页查询请求摘要。
func (s *Service) ListRequests(
	ctx context.Context,
	request *analyticsv1.ListRequestsRequest,
) (*analyticsv1.ListRequestsResponse, error) {
	options, err := buildListOptions(request)
	if err != nil {
		return nil, err
	}
	page, err := s.query.List(ctx, options)
	if err != nil {
		return nil, err
	}
	nextPageToken, err := formatPageToken(page.NextCursor, options.Filter)
	if err != nil {
		return nil, kerrors.InternalServer("FORMAT_PAGE_TOKEN_FAILED", "format page token failed").WithCause(err)
	}
	return &analyticsv1.ListRequestsResponse{
		Requests: lo.Map(page.Records, func(record requestbiz.Summary, _ int) *analyticsv1.RequestSummary {
			return summaryResponse(record)
		}),
		NextPageToken: nextPageToken,
	}, nil
}

// GetRequest 使用记录 ID 和开始时间查询单次请求明细。
func (s *Service) GetRequest(
	ctx context.Context,
	request *analyticsv1.GetRequestRequest,
) (*alsv1.RequestRecord, error) {
	startedAt := request.GetStartedAt()
	if !requestrecord.IsValidID(request.GetId()) || startedAt == nil ||
		startedAt.CheckValid() != nil || !analyticsconfig.IsSupportedTime(startedAt.AsTime()) {
		return nil, kerrors.BadRequest("INVALID_ARGUMENT", "id and started_at are required")
	}
	record, err := s.query.Get(ctx, request.GetId(), startedAt.AsTime())
	if errors.Is(err, requestbiz.ErrNotFound) {
		return nil, kerrors.NotFound("REQUEST_NOT_FOUND", "request record not found")
	}
	if err != nil {
		return nil, err
	}
	return recordResponse(record), nil
}
