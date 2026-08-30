// Package upstream 提供 Upstream 管理 API。
package upstream

import (
	"context"

	"github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	upstreambiz "github.com/lgc202/ingate/internal/adminapi/biz/upstream"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// Service 实现服务管理 API。
type Service struct {
	upstreams *upstreambiz.Usecase
}

// NewService 创建服务协议层。
func NewService(upstreams *upstreambiz.Usecase) *Service {
	return &Service{upstreams: upstreams}
}

// ListUpstreams 返回满足筛选条件的服务列表。
func (s *Service) ListUpstreams(
	ctx context.Context,
	request *adminv1.ListUpstreamsRequest,
) (*adminv1.ListUpstreamsResponse, error) {
	typeFilter := upstreambiz.TypeAny
	switch request.GetType() {
	case adminv1.UpstreamType_UPSTREAM_TYPE_UNSPECIFIED:
	case adminv1.UpstreamType_UPSTREAM_TYPE_HTTP:
		typeFilter = upstreambiz.TypeHTTP
	case adminv1.UpstreamType_UPSTREAM_TYPE_MODEL:
		typeFilter = upstreambiz.TypeModel
	default:
		return nil, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"服务类型不正确",
		)
	}

	filter := upstreambiz.ListFilter{
		ResourceFilter: adminservice.ResourceFilter(
			request.GetQuery(),
			nil,
			request.GetState(),
		),
		Type: typeFilter,
	}
	page, err := s.upstreams.List(
		ctx,
		adminservice.PageRequest(request.GetLimit(), request.GetCursor()),
		filter,
	)
	if err != nil {
		return nil, err
	}
	upstreams := make([]*adminv1.Upstream, len(page.Items))
	for i := range page.Items {
		upstreams[i] = upstreamResponse(&page.Items[i])
	}
	return &adminv1.ListUpstreamsResponse{
		Upstreams:  upstreams,
		NextCursor: page.NextCursor,
	}, nil
}

// GetUpstream 返回指定服务。
func (s *Service) GetUpstream(
	ctx context.Context,
	request *adminv1.GetUpstreamRequest,
) (*adminv1.Upstream, error) {
	upstream, err := s.upstreams.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return upstreamResponse(upstream), nil
}

// CreateUpstream 创建服务。
func (s *Service) CreateUpstream(
	ctx context.Context,
	request *adminv1.CreateUpstreamRequest,
) (*adminv1.Upstream, error) {
	spec, err := parseUpstreamSpec(
		request.GetName(),
		request.GetEndpoints(),
		request.GetTls(),
		request.GetLoadBalancing(),
		request.GetHealthCheck(),
	)
	if err != nil {
		return nil, err
	}
	model, err := parseModelForCreate(request.GetModel())
	if err != nil {
		return nil, err
	}
	spec.Model = model

	upstream, err := s.upstreams.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return upstreamResponse(upstream), nil
}

// UpdateUpstream 完整替换服务配置。
func (s *Service) UpdateUpstream(
	ctx context.Context,
	request *adminv1.UpdateUpstreamRequest,
) (*adminv1.Upstream, error) {
	spec, err := parseUpstreamSpec(
		request.GetName(),
		request.GetEndpoints(),
		request.GetTls(),
		request.GetLoadBalancing(),
		request.GetHealthCheck(),
	)
	if err != nil {
		return nil, err
	}
	model, preserveAPIKey, err := parseModelForUpdate(request.GetModel())
	if err != nil {
		return nil, err
	}
	spec.Model = model

	input := upstreambiz.ReplaceInput{
		ExpectedGeneration: request.GetVersion(),
		Spec:               spec,
		PreserveAPIKey:     preserveAPIKey,
	}
	upstream, err := s.upstreams.Replace(ctx, request.GetId(), input)
	if err != nil {
		return nil, err
	}
	return upstreamResponse(upstream), nil
}

// DeleteUpstream 删除服务。
func (s *Service) DeleteUpstream(
	ctx context.Context,
	request *adminv1.DeleteUpstreamRequest,
) (*emptypb.Empty, error) {
	if err := s.upstreams.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
