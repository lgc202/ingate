package service

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
)

// UpstreamService 实现服务管理 API
type UpstreamService struct {
	usecase *biz.UpstreamUsecase
}

// NewUpstreamService 创建服务协议层
func NewUpstreamService(usecase *biz.UpstreamUsecase) *UpstreamService {
	return &UpstreamService{usecase: usecase}
}

func (s *UpstreamService) ListUpstreams(ctx context.Context, _ *emptypb.Empty) (*adminv1.ListUpstreamsReply, error) {
	items, err := s.usecase.List(ctx)
	if err != nil {
		return nil, operationError(err, "查询服务失败")
	}
	reply := &adminv1.ListUpstreamsReply{Upstreams: make([]*adminv1.Upstream, 0, len(items))}
	for i := range items {
		reply.Upstreams = append(reply.Upstreams, upstreamReply(&items[i]))
	}
	return reply, nil
}

func (s *UpstreamService) GetUpstream(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.Upstream, error) {
	item, err := s.usecase.Get(ctx, request.GetId())
	if err != nil {
		return nil, operationError(err, "查询服务失败")
	}
	return upstreamReply(item), nil
}

func (s *UpstreamService) CreateUpstream(ctx context.Context, request *adminv1.CreateUpstreamRequest) (*adminv1.MutationReply, error) {
	spec, err := upstreamSpec(
		request.GetName(), request.GetType(), request.GetProtocol(), request.GetTls(), request.GetModel(),
		request.GetEndpoints(), request.GetLoadBalancePolicy(), request.GetHealthCheck(), request.GetApiKey(),
	)
	if err != nil {
		return nil, err
	}
	id, err := s.usecase.Create(ctx, spec)
	if err != nil {
		return nil, operationError(err, "创建服务失败")
	}
	return &adminv1.MutationReply{Success: true, Id: id}, nil
}

func (s *UpstreamService) UpdateUpstream(ctx context.Context, request *adminv1.UpdateUpstreamRequest) (*adminv1.MutationReply, error) {
	if request.GetApiKey() != nil && request.GetRemoveApiKey() {
		return nil, badRequest("不能同时设置和移除 API Key")
	}
	spec, err := upstreamSpec(
		request.GetName(), request.GetType(), request.GetProtocol(), request.GetTls(), request.GetModel(),
		request.GetEndpoints(), request.GetLoadBalancePolicy(), request.GetHealthCheck(), request.GetApiKey(),
	)
	if err != nil {
		return nil, err
	}
	if err := s.usecase.Update(ctx, request.GetId(), request.GetVersion(), spec, request.GetRemoveApiKey()); err != nil {
		return nil, operationError(err, "更新服务失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}

func (s *UpstreamService) DeleteUpstream(ctx context.Context, request *adminv1.ResourceRequest) (*adminv1.MutationReply, error) {
	if err := s.usecase.Delete(ctx, request.GetId()); err != nil {
		return nil, operationError(err, "删除服务失败")
	}
	return &adminv1.MutationReply{Success: true, Id: request.GetId()}, nil
}
