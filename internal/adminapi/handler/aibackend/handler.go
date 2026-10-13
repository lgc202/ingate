// Package aibackend 适配 AIBackendService 的 HTTP 和 gRPC 协议。
package aibackend

import (
	"context"
	"errors"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminhandler "github.com/lgc202/ingate/internal/adminapi/handler"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	aibackendservice "github.com/lgc202/ingate/internal/adminapi/service/aibackend"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

const (
	reasonInvalidArgument = "AI_BACKEND_INVALID_ARGUMENT"
	reasonAlreadyExists   = "AI_BACKEND_ALREADY_EXISTS"
	reasonNotFound        = "AI_BACKEND_NOT_FOUND"
	reasonConflict        = "AI_BACKEND_CONFLICT"
)

var (
	_ adminv1.AIBackendServiceServer     = (*Handler)(nil)
	_ adminv1.AIBackendServiceHTTPServer = (*Handler)(nil)
)

// Handler 实现 AIBackend 的 gRPC 和 HTTP 协议入口。
type Handler struct {
	adminv1.UnimplementedAIBackendServiceServer

	service *aibackendservice.Service
}

// New 创建 AIBackend Handler。
func New(service *aibackendservice.Service) *Handler {
	return &Handler{service: service}
}

// CreateAIBackend 创建 AIBackend。
func (h *Handler) CreateAIBackend(
	ctx context.Context,
	req *adminv1.CreateAIBackendRequest,
) (*adminv1.CreateAIBackendReply, error) {
	created, err := h.service.Create(ctx, aibackendservice.CreateParams{
		Namespace:   req.GetNamespace(),
		DisplayName: req.GetDisplayName(),
		BackendRef:  gatewayv1alpha1.BackendReference{Name: req.GetBackendRef().GetName()},
		Schema:      schemaFromProto(req.GetSchema()),
	})
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.CreateAIBackendReply{Backend: backendToProto(created)}, nil
}

// GetAIBackend 读取指定 AIBackend。
func (h *Handler) GetAIBackend(
	ctx context.Context,
	req *adminv1.GetAIBackendRequest,
) (*adminv1.GetAIBackendReply, error) {
	backend, err := h.service.Get(ctx, req.GetNamespace(), req.GetName())
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.GetAIBackendReply{Backend: backendToProto(backend)}, nil
}

// ListAIBackends 分页列出 AIBackend。
func (h *Handler) ListAIBackends(
	ctx context.Context,
	req *adminv1.ListAIBackendsRequest,
) (*adminv1.ListAIBackendsReply, error) {
	page, err := h.service.List(ctx, aibackendservice.ListParams{
		Namespace:   req.GetNamespace(),
		PageSize:    req.GetPageSize(),
		PageNum:     req.GetPageNum(),
		DisplayName: req.GetDisplayName(),
	})
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	backends := make([]*adminv1.AIBackend, len(page.Items))
	for i, backend := range page.Items {
		backends[i] = backendToProto(backend)
	}

	return &adminv1.ListAIBackendsReply{Backends: backends, TotalCount: page.TotalCount}, nil
}

// UpdateAIBackend 完整替换 AIBackend 的可编辑配置。
func (h *Handler) UpdateAIBackend(
	ctx context.Context,
	req *adminv1.UpdateAIBackendRequest,
) (*adminv1.UpdateAIBackendReply, error) {
	updated, err := h.service.Update(ctx, aibackendservice.UpdateParams{
		Namespace:   req.GetNamespace(),
		DisplayName: req.GetDisplayName(),
		Name:        req.GetName(),
		Version:     req.GetVersion(),
		BackendRef:  gatewayv1alpha1.BackendReference{Name: req.GetBackendRef().GetName()},
		Schema:      schemaFromProto(req.GetSchema()),
	})
	if err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.UpdateAIBackendReply{Backend: backendToProto(updated)}, nil
}

// DeleteAIBackend 删除指定 AIBackend。
func (h *Handler) DeleteAIBackend(
	ctx context.Context,
	req *adminv1.DeleteAIBackendRequest,
) (*adminv1.DeleteAIBackendReply, error) {
	if err := h.service.Delete(ctx, req.GetNamespace(), req.GetName()); err != nil {
		return nil, toTransportError(ctx, err)
	}

	return &adminv1.DeleteAIBackendReply{}, nil
}

func schemaFromProto(schema *adminv1.APISchema) gatewayv1alpha1.APISchema {
	return gatewayv1alpha1.APISchema{
		Name:   gatewayv1alpha1.APISchemaName(schema.GetName()),
		Prefix: schema.GetPrefix(),
	}
}

func backendToProto(backend repository.AIBackendInfo) *adminv1.AIBackend {
	return &adminv1.AIBackend{
		Namespace:   backend.Namespace,
		Name:        backend.Name,
		DisplayName: backend.DisplayName,
		BackendRef:  &adminv1.BackendReference{Name: backend.BackendRef.Name},
		Schema:      &adminv1.APISchema{Name: string(backend.Schema.Name), Prefix: backend.Schema.Prefix},
		CreateTime:  timestamppb.New(backend.CreateTime),
		Version:     backend.Version,
	}
}

func toTransportError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, aibackendservice.ErrInvalidArgument):
		return kratoserrors.BadRequest(reasonInvalidArgument, "模型服务参数不合法").WithCause(err)
	case errors.Is(err, aibackendservice.ErrAlreadyExists):
		return kratoserrors.Conflict(reasonAlreadyExists, "资源标识冲突，请重新创建").WithCause(err)
	case errors.Is(err, aibackendservice.ErrNotFound):
		return kratoserrors.NotFound(reasonNotFound, "模型服务不存在").WithCause(err)
	case errors.Is(err, aibackendservice.ErrConflict):
		return kratoserrors.Conflict(reasonConflict, "模型服务已被修改，请刷新后重试").WithCause(err)
	default:
		return adminhandler.ToTransportError(ctx, err)
	}
}
