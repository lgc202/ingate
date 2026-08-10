// Package upstream 实现 Upstream 管理 API
package upstream

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	upstreambiz "github.com/lgc202/ingate/internal/adminapi/biz/upstream"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
)

// Service 实现服务管理 API
type Service struct {
	usecase *upstreambiz.Usecase
}

// NewService 创建服务协议层
func NewService(usecase *upstreambiz.Usecase) *Service {
	return &Service{usecase: usecase}
}

func (s *Service) ListUpstreams(ctx context.Context, request *adminv1.ListUpstreamsRequest) (*adminv1.ListUpstreamsResponse, error) {
	result, err := s.usecase.List(ctx, adminservice.PageRequest(request.GetLimit(), request.GetCursor()))
	if err != nil {
		return nil, err
	}
	response := &adminv1.ListUpstreamsResponse{
		Upstreams:  make([]*adminv1.Upstream, 0, len(result.Items)),
		NextCursor: result.NextCursor,
	}
	for i := range result.Items {
		response.Upstreams = append(response.Upstreams, upstreamFromResource(&result.Items[i]))
	}
	return response, nil
}

func (s *Service) GetUpstream(ctx context.Context, request *adminv1.GetUpstreamRequest) (*adminv1.Upstream, error) {
	item, err := s.usecase.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return upstreamFromResource(item), nil
}

func (s *Service) CreateUpstream(ctx context.Context, request *adminv1.CreateUpstreamRequest) (*adminv1.Upstream, error) {
	spec, err := buildUpstreamSpec(upstreamInput{
		name:          request.GetName(),
		upstreamType:  request.GetType(),
		endpoints:     request.GetEndpoints(),
		tls:           request.GetTls(),
		loadBalancing: request.GetLoadBalancing(),
		healthCheck:   request.GetHealthCheck(),
		model:         request.GetModel(),
		apiKey:        request.ApiKey,
	})
	if err != nil {
		return nil, err
	}
	item, err := s.usecase.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return upstreamFromResource(item), nil
}

func (s *Service) UpdateUpstream(ctx context.Context, request *adminv1.UpdateUpstreamRequest) (*adminv1.Upstream, error) {
	spec, err := buildUpstreamSpec(upstreamInput{
		name:          request.GetName(),
		upstreamType:  request.GetType(),
		endpoints:     request.GetEndpoints(),
		tls:           request.GetTls(),
		loadBalancing: request.GetLoadBalancing(),
		healthCheck:   request.GetHealthCheck(),
		model:         request.GetModel(),
		apiKey:        request.ApiKey,
	})
	if err != nil {
		return nil, err
	}
	item, err := s.usecase.Update(ctx, request.GetId(), request.GetVersion(), spec, request.ApiKey)
	if err != nil {
		return nil, err
	}
	return upstreamFromResource(item), nil
}

func (s *Service) DeleteUpstream(ctx context.Context, request *adminv1.DeleteUpstreamRequest) (*emptypb.Empty, error) {
	if err := s.usecase.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
