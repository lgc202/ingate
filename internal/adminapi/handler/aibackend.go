package handler

import (
	"context"
	"errors"

	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/errno"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	aibackendservice "github.com/lgc202/ingate/internal/adminapi/service/aibackend"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

var (
	_ adminv1.AIBackendServiceServer     = (*AIBackendHandler)(nil)
	_ adminv1.AIBackendServiceHTTPServer = (*AIBackendHandler)(nil)
)

// AIBackendHandler 实现 AIBackend 的 gRPC 和 HTTP 协议入口。
type AIBackendHandler struct {
	adminv1.UnimplementedAIBackendServiceServer

	service *aibackendservice.Service
}

// NewAIBackend 创建模型服务协议入口，复用注入的业务服务。
func NewAIBackend(service *aibackendservice.Service) *AIBackendHandler {
	return &AIBackendHandler{service: service}
}

// CreateAIBackend 创建 AIBackend。
func (h *AIBackendHandler) CreateAIBackend(
	ctx context.Context,
	req *adminv1.CreateAIBackendRequest,
) (*adminv1.CreateAIBackendReply, error) {
	created, err := h.service.Create(ctx, aibackendservice.CreateParams{
		Namespace:   req.GetNamespace(),
		DisplayName: req.GetDisplayName(),
		BackendRef:  gatewayv1alpha1.BackendReference{Name: req.GetBackendRef().GetName()},
		Schema:      aiBackendSchemaFromProto(req.GetSchema()),
	})
	if err != nil {
		return nil, toAIBackendError(err)
	}

	return &adminv1.CreateAIBackendReply{Backend: aiBackendToProto(created)}, nil
}

// GetAIBackend 读取指定 AIBackend。
func (h *AIBackendHandler) GetAIBackend(
	ctx context.Context,
	req *adminv1.GetAIBackendRequest,
) (*adminv1.GetAIBackendReply, error) {
	backend, err := h.service.Get(ctx, req.GetNamespace(), req.GetName())
	if err != nil {
		return nil, toAIBackendError(err)
	}

	return &adminv1.GetAIBackendReply{Backend: aiBackendToProto(backend)}, nil
}

// ListAIBackends 分页列出 AIBackend。
func (h *AIBackendHandler) ListAIBackends(
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
		return nil, toAIBackendError(err)
	}

	backends := make([]*adminv1.AIBackend, len(page.Items))
	for i, backend := range page.Items {
		backends[i] = aiBackendToProto(backend)
	}

	return &adminv1.ListAIBackendsReply{Backends: backends, TotalCount: page.TotalCount}, nil
}

// UpdateAIBackend 完整替换 AIBackend 的可编辑配置。
func (h *AIBackendHandler) UpdateAIBackend(
	ctx context.Context,
	req *adminv1.UpdateAIBackendRequest,
) (*adminv1.UpdateAIBackendReply, error) {
	updated, err := h.service.Update(ctx, aibackendservice.UpdateParams{
		Namespace:   req.GetNamespace(),
		DisplayName: req.GetDisplayName(),
		Name:        req.GetName(),
		Version:     req.GetVersion(),
		BackendRef:  gatewayv1alpha1.BackendReference{Name: req.GetBackendRef().GetName()},
		Schema:      aiBackendSchemaFromProto(req.GetSchema()),
	})
	if err != nil {
		return nil, toAIBackendError(err)
	}

	return &adminv1.UpdateAIBackendReply{Backend: aiBackendToProto(updated)}, nil
}

// DeleteAIBackend 删除指定 AIBackend。
func (h *AIBackendHandler) DeleteAIBackend(
	ctx context.Context,
	req *adminv1.DeleteAIBackendRequest,
) (*adminv1.DeleteAIBackendReply, error) {
	if err := h.service.Delete(ctx, req.GetNamespace(), req.GetName()); err != nil {
		return nil, toAIBackendError(err)
	}

	return &adminv1.DeleteAIBackendReply{}, nil
}

func aiBackendSchemaFromProto(schema *adminv1.APISchema) gatewayv1alpha1.APISchema {
	return gatewayv1alpha1.APISchema{
		Name:   gatewayv1alpha1.APISchemaName(schema.GetName()),
		Prefix: schema.GetPrefix(),
	}
}

func aiBackendToProto(backend repository.AIBackendInfo) *adminv1.AIBackend {
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

func toAIBackendError(err error) error {
	switch {
	case errors.Is(err, aibackendservice.ErrInvalidArgument):
		return errno.ErrAIBackendInvalidArgument.WithCause(err)
	case errors.Is(err, aibackendservice.ErrAlreadyExists):
		return errno.ErrAIBackendAlreadyExists.WithCause(err)
	case errors.Is(err, aibackendservice.ErrNotFound):
		return errno.ErrAIBackendNotFound.WithCause(err)
	case errors.Is(err, aibackendservice.ErrConflict):
		return errno.ErrAIBackendConflict.WithCause(err)
	default:
		return errno.Fallback(err)
	}
}
