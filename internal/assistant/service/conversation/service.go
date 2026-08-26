// Package conversation 适配运维助手的会话、消息与 Run 产品协议。
package conversation

import (
	"context"
	"errors"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport"

	conversationbiz "github.com/lgc202/ingate/internal/assistant/biz/conversation"
	runbiz "github.com/lgc202/ingate/internal/assistant/biz/run"
)

const (
	forwardedUserHeader = "X-Forwarded-User"
	maxActorIDLength    = 128
	defaultLimit        = 20
	maxLimit            = 100
)

// Service 实现助手会话和异步执行的产品协议。
type Service struct {
	conversations *conversationbiz.Service
	runs          *runbiz.Service
}

// NewService 创建会话协议服务。
func NewService(conversations *conversationbiz.Service, runs *runbiz.Service) *Service {
	return &Service{conversations: conversations, runs: runs}
}

func (s *Service) actorID(ctx context.Context) (string, error) {
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return "", kratoserrors.Unauthorized("ACTOR_REQUIRED", "authentication required")
	}
	actorID := strings.TrimSpace(tr.RequestHeader().Get(forwardedUserHeader))
	if actorID == "" {
		return "", kratoserrors.Unauthorized("ACTOR_REQUIRED", "authentication required")
	}
	if len(actorID) > maxActorIDLength {
		return "", invalidArgument("actor identifier is too long")
	}
	return actorID, nil
}

func (s *Service) mapError(err error) error {
	switch {
	case errors.Is(err, conversationbiz.ErrInvalidTitle):
		return invalidArgument("conversation title is required")
	case errors.Is(err, conversationbiz.ErrNotFound), errors.Is(err, runbiz.ErrNotFound):
		return kratoserrors.NotFound("RESOURCE_NOT_FOUND", "resource not found")
	case errors.Is(err, runbiz.ErrStateConflict), errors.Is(err, runbiz.ErrConversationBusy):
		return kratoserrors.Conflict("RESOURCE_CONFLICT", "resource state changed")
	default:
		return kratoserrors.InternalServer("INTERNAL_ERROR", "request failed").WithCause(err)
	}
}

func invalidArgument(reason string) error {
	return kratoserrors.BadRequest("INVALID_ARGUMENT", reason)
}

func pageLimit(value int32) int {
	if value == 0 {
		return defaultLimit
	}
	return min(int(value), maxLimit)
}
