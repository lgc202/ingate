package upstream

import (
	"cmp"
	"context"
	"errors"
	"slices"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/domain"
	adminhandler "github.com/lgc202/ingate/internal/adminapi/handler"
	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	reasonInvalidArgument = "UPSTREAM_INVALID_ARGUMENT"
	reasonAlreadyExists   = "UPSTREAM_ALREADY_EXISTS"
	reasonNotFound        = "UPSTREAM_NOT_FOUND"
	reasonConflict        = "UPSTREAM_CONFLICT"
)

var (
	_ adminv1.UpstreamServiceServer     = (*Handler)(nil)
	_ adminv1.UpstreamServiceHTTPServer = (*Handler)(nil)
)

// Handler 实现 Upstream 的 gRPC 和 HTTP 协议入口。
type Handler struct {
	adminv1.UnimplementedUpstreamServiceServer

	service *upstreamservice.Service
}

// New 创建 Upstream Handler。
func New(service *upstreamservice.Service) *Handler {
	return &Handler{service: service}
}

// CreateUpstream 创建 Upstream。
func (h *Handler) CreateUpstream(
	ctx context.Context,
	req *adminv1.CreateUpstreamRequest,
) (*adminv1.CreateUpstreamReply, error) {
	created, err := h.service.Create(ctx, upstreamservice.CreateParams{
		Name:      req.GetName(),
		Endpoints: endpointsFromProto(req.GetEndpoints()),
	})
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.CreateUpstreamReply{Upstream: upstreamToProto(created)}, nil
}

// GetUpstream 读取指定 Upstream。
func (h *Handler) GetUpstream(
	ctx context.Context,
	req *adminv1.GetUpstreamRequest,
) (*adminv1.GetUpstreamReply, error) {
	upstream, err := h.service.Get(ctx, req.GetName())
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.GetUpstreamReply{Upstream: upstreamToProto(upstream)}, nil
}

// ListUpstreams 分页列出 Upstream。
func (h *Handler) ListUpstreams(
	ctx context.Context,
	req *adminv1.ListUpstreamsRequest,
) (*adminv1.ListUpstreamsReply, error) {
	page, err := h.service.List(ctx, req.GetPageSize(), req.GetPageNum())
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	upstreams := make([]*adminv1.Upstream, len(page.Items))
	for i, upstream := range page.Items {
		upstreams[i] = upstreamToProto(upstream)
	}

	return &adminv1.ListUpstreamsReply{Upstreams: upstreams, TotalCount: page.TotalCount}, nil
}

// UpdateUpstream 完整替换 Upstream 的可编辑配置。
func (h *Handler) UpdateUpstream(
	ctx context.Context,
	req *adminv1.UpdateUpstreamRequest,
) (*adminv1.UpdateUpstreamReply, error) {
	updated, err := h.service.Update(ctx, upstreamservice.UpdateParams{
		Name:      req.GetName(),
		Version:   req.GetVersion(),
		Endpoints: endpointsFromProto(req.GetEndpoints()),
	})
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.UpdateUpstreamReply{Upstream: upstreamToProto(updated)}, nil
}

// DeleteUpstream 删除指定 Upstream。
func (h *Handler) DeleteUpstream(
	ctx context.Context,
	req *adminv1.DeleteUpstreamRequest,
) (*adminv1.DeleteUpstreamReply, error) {
	if err := h.service.Delete(ctx, req.GetName()); err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.DeleteUpstreamReply{}, nil
}

func endpointsFromProto(items []*adminv1.Endpoint) []domain.Endpoint {
	endpoints := make([]domain.Endpoint, len(items))
	for i, endpoint := range items {
		endpoints[i] = domain.Endpoint{
			Address: endpoint.GetAddress(),
			Port:    endpoint.GetPort(),
			Weight:  endpoint.GetWeight(),
		}
	}

	return endpoints
}

func upstreamToProto(upstream domain.Upstream) *adminv1.Upstream {
	endpoints := make([]*adminv1.Endpoint, len(upstream.Endpoints))
	for i, endpoint := range upstream.Endpoints {
		endpoints[i] = &adminv1.Endpoint{
			Address: endpoint.Address,
			Port:    endpoint.Port,
			Weight:  endpoint.Weight,
		}
	}
	// 端点顺序不是业务状态；统一排序，使写入响应与投影查询结果一致。
	slices.SortFunc(endpoints, func(a, b *adminv1.Endpoint) int {
		if order := cmp.Compare(a.Address, b.Address); order != 0 {
			return order
		}

		return cmp.Compare(a.Port, b.Port)
	})

	return &adminv1.Upstream{
		Name:       upstream.Name,
		Endpoints:  endpoints,
		CreateTime: timestamppb.New(upstream.CreateTime),
		Version:    upstream.Version,
	}
}

func toTransportError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, upstreamservice.ErrInvalidArgument):
		return kratoserrors.BadRequest(reasonInvalidArgument, "后端资源参数不合法").WithCause(err)
	case errors.Is(err, upstreamservice.ErrAlreadyExists):
		return kratoserrors.Conflict(reasonAlreadyExists, "同名后端资源已存在").WithCause(err)
	case errors.Is(err, upstreamservice.ErrNotFound):
		return kratoserrors.NotFound(reasonNotFound, "后端资源不存在").WithCause(err)
	case errors.Is(err, upstreamservice.ErrConflict):
		return kratoserrors.Conflict(reasonConflict, "后端资源已被修改，请刷新后重试").WithCause(err)
	default:
		return adminhandler.ToTransportError(ctx, err)
	}
}
