// Package upstream 实现 Upstream 管理 API
package upstream

import (
	"context"

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

func (s *Service) ListUpstreams(ctx context.Context, request *adminv1.ListRequest) (*adminv1.ListUpstreamsReply, error) {
	result, err := s.usecase.List(ctx, adminservice.PageRequest(request.GetPageSize(), request.GetPageToken()))
	if err != nil {
		return nil, err
	}
	reply := &adminv1.ListUpstreamsReply{Upstreams: make([]*adminv1.Upstream, 0, len(result.Items)), Page: adminservice.PageInfo(result.NextCursor)}
	for i := range result.Items {
		reply.Upstreams = append(reply.Upstreams, newUpstreamReply(&result.Items[i]))
	}
	return reply, nil
}

func (s *Service) GetUpstream(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.Upstream, error) {
	item, err := s.usecase.Get(ctx, request.GetId())
	if err != nil {
		return nil, err
	}
	return newUpstreamReply(item), nil
}

func (s *Service) CreateUpstream(ctx context.Context, request *adminv1.CreateUpstreamRequest) (*adminv1.MutationReply, error) {
	spec, err := buildUpstreamSpec(
		request.GetName(), request.GetType(), request.GetProtocol(), request.GetTls(), request.GetModel(),
		request.GetEndpoints(), request.GetLoadBalancePolicy(), request.GetHealthCheck(), request.GetApiKey(),
	)
	if err != nil {
		return nil, err
	}
	id, err := s.usecase.Create(ctx, spec)
	if err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: id}, nil
}

func (s *Service) UpdateUpstream(ctx context.Context, request *adminv1.UpdateUpstreamRequest) (*adminv1.MutationReply, error) {
	if request.GetApiKey() != nil && request.GetRemoveApiKey() {
		return nil, adminservice.BadRequest("不能同时设置和移除 API Key")
	}
	spec, err := buildUpstreamSpec(
		request.GetName(), request.GetType(), request.GetProtocol(), request.GetTls(), request.GetModel(),
		request.GetEndpoints(), request.GetLoadBalancePolicy(), request.GetHealthCheck(), request.GetApiKey(),
	)
	if err != nil {
		return nil, err
	}
	if err := s.usecase.Update(ctx, request.GetId(), request.GetVersion(), spec, request.GetRemoveApiKey()); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *Service) DeleteUpstream(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.Delete(ctx, request.GetId()); err != nil {
		return nil, err
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}
