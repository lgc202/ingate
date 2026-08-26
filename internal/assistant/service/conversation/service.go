// Package conversation 适配运维助手的会话与消息协议。
package conversation

import (
	"errors"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"

	assistantv1 "github.com/lgc202/ingate/api/assistant/v1"
	conversationbiz "github.com/lgc202/ingate/internal/assistant/biz/conversation"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

// Service 实现助手会话和消息查询协议。
type Service struct {
	assistantv1.UnimplementedConversationServiceServer

	conversations *conversationbiz.Service
}

// NewService 创建会话协议服务。
func NewService(conversations *conversationbiz.Service) *Service {
	return &Service{conversations: conversations}
}

func (s *Service) mapError(err error) error {
	switch {
	case errors.Is(err, conversationbiz.ErrInvalidTitle):
		return invalidArgument("conversation title is required")
	case errors.Is(err, conversationbiz.ErrNotFound):
		return kratoserrors.NotFound("RESOURCE_NOT_FOUND", "resource not found")
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
