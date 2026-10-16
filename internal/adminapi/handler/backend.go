package handler

import (
	"cmp"
	"context"
	"errors"
	"slices"

	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/errno"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	backendservice "github.com/lgc202/ingate/internal/adminapi/service/backend"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

var (
	_ adminv1.BackendServiceServer     = (*BackendHandler)(nil)
	_ adminv1.BackendServiceHTTPServer = (*BackendHandler)(nil)
)

// BackendHandler 实现 Backend 的 gRPC 和 HTTP 协议入口。
type BackendHandler struct {
	adminv1.UnimplementedBackendServiceServer

	service *backendservice.Service
}

// NewBackend 创建后端资源协议入口，复用注入的业务服务。
func NewBackend(service *backendservice.Service) *BackendHandler {
	return &BackendHandler{service: service}
}

// CreateBackend 创建 Backend。
func (h *BackendHandler) CreateBackend(
	ctx context.Context,
	req *adminv1.CreateBackendRequest,
) (*adminv1.CreateBackendReply, error) {
	created, err := h.service.Create(ctx, backendservice.CreateParams{
		Namespace:   req.GetNamespace(),
		DisplayName: req.GetDisplayName(),
		Endpoints:   backendEndpointsFromProto(req.GetEndpoints()),
	})
	if err != nil {
		return nil, toBackendError(err)
	}

	return &adminv1.CreateBackendReply{Backend: backendToProto(created)}, nil
}

// GetBackend 读取指定 Backend。
func (h *BackendHandler) GetBackend(
	ctx context.Context,
	req *adminv1.GetBackendRequest,
) (*adminv1.GetBackendReply, error) {
	backend, err := h.service.Get(ctx, req.GetNamespace(), req.GetName())
	if err != nil {
		return nil, toBackendError(err)
	}

	return &adminv1.GetBackendReply{Backend: backendToProto(backend)}, nil
}

// ListBackends 分页列出 Backend。
func (h *BackendHandler) ListBackends(
	ctx context.Context,
	req *adminv1.ListBackendsRequest,
) (*adminv1.ListBackendsReply, error) {
	page, err := h.service.List(ctx, backendservice.ListParams{
		Namespace:   req.GetNamespace(),
		PageSize:    req.GetPageSize(),
		PageNum:     req.GetPageNum(),
		DisplayName: req.GetDisplayName(),
	})
	if err != nil {
		return nil, toBackendError(err)
	}

	backends := make([]*adminv1.Backend, len(page.Items))
	for i, backend := range page.Items {
		backends[i] = backendToProto(backend)
	}

	return &adminv1.ListBackendsReply{Backends: backends, TotalCount: page.TotalCount}, nil
}

// UpdateBackend 完整替换 Backend 的可编辑配置。
func (h *BackendHandler) UpdateBackend(
	ctx context.Context,
	req *adminv1.UpdateBackendRequest,
) (*adminv1.UpdateBackendReply, error) {
	updated, err := h.service.Update(ctx, backendservice.UpdateParams{
		Namespace:   req.GetNamespace(),
		DisplayName: req.GetDisplayName(),
		Name:        req.GetName(),
		Version:     req.GetVersion(),
		Endpoints:   backendEndpointsFromProto(req.GetEndpoints()),
	})
	if err != nil {
		return nil, toBackendError(err)
	}

	return &adminv1.UpdateBackendReply{Backend: backendToProto(updated)}, nil
}

// DeleteBackend 删除指定 Backend。
func (h *BackendHandler) DeleteBackend(
	ctx context.Context,
	req *adminv1.DeleteBackendRequest,
) (*adminv1.DeleteBackendReply, error) {
	if err := h.service.Delete(ctx, req.GetNamespace(), req.GetName()); err != nil {
		return nil, toBackendError(err)
	}

	return &adminv1.DeleteBackendReply{}, nil
}

func backendEndpointsFromProto(items []*adminv1.Endpoint) []gatewayv1alpha1.Endpoint {
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
		Namespace:   backend.Namespace,
		Name:        backend.Name,
		DisplayName: backend.DisplayName,
		Endpoints:   endpoints,
		CreateTime:  timestamppb.New(backend.CreateTime),
		Version:     backend.Version,
	}
}

func toBackendError(err error) error {
	switch {
	case errors.Is(err, backendservice.ErrInvalidArgument):
		return errno.ErrBackendInvalidArgument.WithCause(err)
	case errors.Is(err, backendservice.ErrAlreadyExists):
		return errno.ErrBackendAlreadyExists.WithCause(err)
	case errors.Is(err, backendservice.ErrNotFound):
		return errno.ErrBackendNotFound.WithCause(err)
	case errors.Is(err, backendservice.ErrConflict):
		return errno.ErrBackendConflict.WithCause(err)
	default:
		return errno.Fallback(err)
	}
}
