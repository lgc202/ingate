// Package upstream 提供 Upstream 管理 API
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
	upstreams *upstreambiz.Service
}

// NewService 创建服务协议层
func NewService(upstreams *upstreambiz.Service) *Service {
	return &Service{upstreams: upstreams}
}

func (s *Service) ListUpstreams(ctx context.Context, request *adminv1.ListUpstreamsRequest) (*adminv1.ListUpstreamsResponse, error) {
	page, err := s.upstreams.List(ctx, adminservice.PageRequest(request.GetLimit(), request.GetCursor()))
	if err != nil {
		return nil, err
	}
	response := &adminv1.ListUpstreamsResponse{
		Upstreams:  make([]*adminv1.Upstream, 0, len(page.Items)),
		NextCursor: page.NextCursor,
	}
	for i := range page.Items {
		response.Upstreams = append(response.Upstreams, upstreamResponse(&page.Items[i]))
	}
	return response, nil
}

func (s *Service) GetUpstream(ctx context.Context, request *adminv1.GetUpstreamRequest) (*adminv1.Upstream, error) {
	upstream, err := s.upstreams.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return upstreamResponse(upstream), nil
}

func (s *Service) CreateUpstream(ctx context.Context, request *adminv1.CreateUpstreamRequest) (*adminv1.Upstream, error) {
	spec, err := createSpec(request)
	if err != nil {
		return nil, err
	}
	upstream, err := s.upstreams.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return upstreamResponse(upstream), nil
}

func (s *Service) UpdateUpstream(ctx context.Context, request *adminv1.UpdateUpstreamRequest) (*adminv1.Upstream, error) {
	spec, preserveAPIKey, err := updateSpec(request)
	if err != nil {
		return nil, err
	}
	upstream, err := s.upstreams.Update(ctx, request.GetId(), upstreambiz.UpdateInput{
		Version:             request.GetVersion(),
		Spec:                spec,
		PreserveModelAPIKey: preserveAPIKey,
	})
	if err != nil {
		return nil, err
	}
	return upstreamResponse(upstream), nil
}

func (s *Service) DeleteUpstream(ctx context.Context, request *adminv1.DeleteUpstreamRequest) (*emptypb.Empty, error) {
	if err := s.upstreams.Delete(ctx, request.GetId(), request.GetVersion()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
