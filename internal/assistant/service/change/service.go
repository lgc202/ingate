// Package change 适配运维助手配置变更的审批协议。
package change

import (
	"context"
	"errors"
	"strings"

	kerrors "github.com/go-kratos/kratos/v3/errors"

	assistantv1 "github.com/lgc202/ingate/api/assistant/v1"
	changebiz "github.com/lgc202/ingate/internal/assistant/biz/change"
	"github.com/lgc202/ingate/internal/assistant/service/identity"
)

var _ assistantv1.ProposedChangeServiceHTTPServer = (*Service)(nil)

// Service 实现配置变更的查询、批准和拒绝协议。
type Service struct {
	changes *changebiz.Usecase
}

// NewService 创建配置变更协议服务。
func NewService(changes *changebiz.Usecase) *Service {
	return &Service{changes: changes}
}

// ListProposedChanges 返回当前管理员在指定会话中的配置变更。
func (s *Service) ListProposedChanges(
	ctx context.Context,
	request *assistantv1.ListProposedChangesRequest,
) (*assistantv1.ListProposedChangesResponse, error) {
	actorID, err := identity.ActorID(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.changes.List(ctx, actorID, request.GetConversationId())
	if err != nil {
		return nil, mapError(err)
	}
	response := &assistantv1.ListProposedChangesResponse{
		ProposedChanges: make([]*assistantv1.ProposedChange, 0, len(items)),
	}
	for _, item := range items {
		response.ProposedChanges = append(response.ProposedChanges, changeResponse(item))
	}
	return response, nil
}

// ApproveProposedChange 批准配置变更，并排队恢复生成它的 Agent 执行。
func (s *Service) ApproveProposedChange(
	ctx context.Context,
	request *assistantv1.ApproveProposedChangeRequest,
) (*assistantv1.ProposedChange, error) {
	actorID, err := identity.ActorID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.changes.Approve(ctx, actorID, request.GetId())
	if err != nil {
		return nil, mapError(err)
	}
	return changeResponse(item), nil
}

// RejectProposedChange 拒绝配置变更，并排队恢复生成它的 Agent 执行。
func (s *Service) RejectProposedChange(
	ctx context.Context,
	request *assistantv1.RejectProposedChangeRequest,
) (*assistantv1.ProposedChange, error) {
	actorID, err := identity.ActorID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.changes.Reject(ctx, actorID, request.GetId())
	if err != nil {
		return nil, mapError(err)
	}
	return changeResponse(item), nil
}

// ReviseProposedChange 拒绝当前配置，并把文字反馈交回原 Agent 上下文。
func (s *Service) ReviseProposedChange(
	ctx context.Context,
	request *assistantv1.ReviseProposedChangeRequest,
) (*assistantv1.ProposedChange, error) {
	actorID, err := identity.ActorID(ctx)
	if err != nil {
		return nil, err
	}
	feedback := strings.TrimSpace(request.GetFeedback())
	if feedback == "" {
		return nil, kerrors.BadRequest("INVALID_ARGUMENT", "feedback is required")
	}
	item, err := s.changes.Revise(ctx, actorID, request.GetId(), feedback)
	if err != nil {
		return nil, mapError(err)
	}
	return changeResponse(item), nil
}

func mapError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return kerrors.ClientClosed("REQUEST_CANCELLED", "request cancelled").WithCause(err)
	case errors.Is(err, context.DeadlineExceeded):
		return kerrors.GatewayTimeout("REQUEST_TIMEOUT", "request timed out").WithCause(err)
	case errors.Is(err, changebiz.ErrNotFound):
		return kerrors.NotFound("RESOURCE_NOT_FOUND", "resource not found")
	case errors.Is(err, changebiz.ErrStateConflict):
		return kerrors.Conflict("RESOURCE_CONFLICT", "resource state changed")
	default:
		return kerrors.InternalServer("INTERNAL_ERROR", "request failed").WithCause(err)
	}
}
