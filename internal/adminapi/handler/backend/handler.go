package backend

import (
	"cmp"
	"context"
	"errors"
	"slices"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminhandler "github.com/lgc202/ingate/internal/adminapi/handler"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	backendservice "github.com/lgc202/ingate/internal/adminapi/service/backend"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

const (
	reasonInvalidArgument = "BACKEND_INVALID_ARGUMENT"
	reasonAlreadyExists   = "BACKEND_ALREADY_EXISTS"
	reasonNotFound        = "BACKEND_NOT_FOUND"
	reasonConflict        = "BACKEND_CONFLICT"
)

var (
	_ adminv1.BackendServiceServer     = (*Handler)(nil)
	_ adminv1.BackendServiceHTTPServer = (*Handler)(nil)
)

// Handler 实现 Backend 的 gRPC 和 HTTP 协议入口。
type Handler struct {
	adminv1.UnimplementedBackendServiceServer

	service *backendservice.Service
}

// New 创建 Backend Handler。
func New(service *backendservice.Service) *Handler {
	return &Handler{service: service}
}

// CreateBackend 创建 Backend。
func (h *Handler) CreateBackend(
	ctx context.Context,
	req *adminv1.CreateBackendRequest,
) (*adminv1.CreateBackendReply, error) {
	created, err := h.service.Create(ctx, backendservice.CreateParams{
		DisplayName: req.GetDisplayName(),
		Endpoints:   endpointsFromProto(req.GetEndpoints()),
	})
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.CreateBackendReply{Backend: backendToProto(created)}, nil
}

// GetBackend 读取指定 Backend。
func (h *Handler) GetBackend(
	ctx context.Context,
	req *adminv1.GetBackendRequest,
) (*adminv1.GetBackendReply, error) {
	backend, err := h.service.Get(ctx, req.GetName())
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.GetBackendReply{Backend: backendToProto(backend)}, nil
}

// ListBackends 分页列出 Backend。
func (h *Handler) ListBackends(
	ctx context.Context,
	req *adminv1.ListBackendsRequest,
) (*adminv1.ListBackendsReply, error) {
	page, err := h.service.List(ctx, backendservice.ListParams{
		PageSize:    req.GetPageSize(),
		PageNum:     req.GetPageNum(),
		DisplayName: req.GetDisplayName(),
	})
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	backends := make([]*adminv1.Backend, len(page.Items))
	for i, backend := range page.Items {
		backends[i] = backendToProto(backend)
	}

	return &adminv1.ListBackendsReply{Backends: backends, TotalCount: page.TotalCount}, nil
}

// UpdateBackend 完整替换 Backend 的可编辑配置。
func (h *Handler) UpdateBackend(
	ctx context.Context,
	req *adminv1.UpdateBackendRequest,
) (*adminv1.UpdateBackendReply, error) {
	updated, err := h.service.Update(ctx, backendservice.UpdateParams{
		DisplayName: req.GetDisplayName(),
		Name:        req.GetName(),
		Version:     req.GetVersion(),
		Endpoints:   endpointsFromProto(req.GetEndpoints()),
	})
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.UpdateBackendReply{Backend: backendToProto(updated)}, nil
}

// DeleteBackend 删除指定 Backend。
func (h *Handler) DeleteBackend(
	ctx context.Context,
	req *adminv1.DeleteBackendRequest,
) (*adminv1.DeleteBackendReply, error) {
	if err := h.service.Delete(ctx, req.GetName()); err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.DeleteBackendReply{}, nil
}

func endpointsFromProto(items []*adminv1.Endpoint) []gatewayv1alpha1.Endpoint {
	endpoints := make([]gatewayv1alpha1.Endpoint, len(items))
	for i, endpoint := range items {
		endpoints[i] = gatewayv1alpha1.Endpoint{
			Address: endpoint.GetAddress(),
			Port:    endpoint.GetPort(),
			Weight:  endpoint.GetWeight(),
		}
	}

	return endpoints
}

func backendToProto(backend repository.BackendInfo) *adminv1.Backend {
	endpoints := make([]*adminv1.Endpoint, len(backend.Endpoints))
	for i, endpoint := range backend.Endpoints {
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

	return &adminv1.Backend{
		Name:        backend.Name,
		DisplayName: backend.DisplayName,
		Endpoints:   endpoints,
		CreateTime:  timestamppb.New(backend.CreateTime),
		Version:     backend.Version,
	}
}

func toTransportError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, backendservice.ErrInvalidArgument):
		return kratoserrors.BadRequest(reasonInvalidArgument, "后端资源参数不合法").WithCause(err)
	case errors.Is(err, backendservice.ErrAlreadyExists):
		return kratoserrors.Conflict(reasonAlreadyExists, "资源标识冲突，请重新创建").WithCause(err)
	case errors.Is(err, backendservice.ErrNotFound):
		return kratoserrors.NotFound(reasonNotFound, "后端资源不存在").WithCause(err)
	case errors.Is(err, backendservice.ErrConflict):
		return kratoserrors.Conflict(reasonConflict, "后端资源已被修改，请刷新后重试").WithCause(err)
	default:
		return adminhandler.ToTransportError(ctx, err)
	}
}
