// Package conversation 适配运维助手的会话与消息协议。
package conversation

import (
	"context"
	"errors"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	assistantv1 "github.com/lgc202/ingate/api/assistant/v1"
	conversationbiz "github.com/lgc202/ingate/internal/assistant/biz/conversation"
	"github.com/lgc202/ingate/internal/assistant/service/identity"
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

// ListConversations 按更新时间倒序返回当前管理员的会话。
func (s *Service) ListConversations(
	ctx context.Context,
	request *assistantv1.ListConversationsRequest,
) (*assistantv1.ListConversationsResponse, error) {
	actorID, err := identity.ActorID(ctx)
	if err != nil {
		return nil, err
	}
	cursor, err := parseConversationCursor(request.GetCursor())
	if err != nil {
		return nil, invalidArgument("invalid conversation cursor")
	}
	page, err := s.conversations.List(ctx, actorID, pageLimit(request.GetLimit()), cursor)
	if err != nil {
		return nil, mapError(err)
	}
	nextCursor, err := formatConversationCursor(page.NextCursor)
	if err != nil {
		return nil, kerrors.InternalServer("ENCODE_CURSOR_FAILED", "request failed").WithCause(err)
	}
	response := &assistantv1.ListConversationsResponse{
		Conversations: make([]*assistantv1.Conversation, 0, len(page.Items)),
		NextCursor:    nextCursor,
	}
	for _, item := range page.Items {
		response.Conversations = append(response.Conversations, conversationResponse(item))
	}
	return response, nil
}

// GetConversation 返回当前管理员可见的单个会话。
func (s *Service) GetConversation(
	ctx context.Context,
	request *assistantv1.GetConversationRequest,
) (*assistantv1.Conversation, error) {
	actorID, err := identity.ActorID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.conversations.Get(ctx, actorID, request.GetId())
	if err != nil {
		return nil, mapError(err)
	}
	return conversationResponse(item), nil
}

// CreateConversation 为当前管理员创建一个会话。
func (s *Service) CreateConversation(
	ctx context.Context,
	request *assistantv1.CreateConversationRequest,
) (*assistantv1.Conversation, error) {
	actorID, err := identity.ActorID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.conversations.Create(ctx, actorID, request.GetTitle())
	if err != nil {
		return nil, mapError(err)
	}
	return conversationResponse(item), nil
}

// UpdateConversation 更新当前管理员会话的标题。
func (s *Service) UpdateConversation(
	ctx context.Context,
	request *assistantv1.UpdateConversationRequest,
) (*assistantv1.Conversation, error) {
	actorID, err := identity.ActorID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.conversations.UpdateTitle(ctx, actorID, request.GetId(), request.GetTitle())
	if err != nil {
		return nil, mapError(err)
	}
	return conversationResponse(item), nil
}

// DeleteConversation 删除当前管理员的会话及其关联记录。
func (s *Service) DeleteConversation(
	ctx context.Context,
	request *assistantv1.DeleteConversationRequest,
) (*emptypb.Empty, error) {
	actorID, err := identity.ActorID(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.conversations.Delete(ctx, actorID, request.GetId()); err != nil {
		return nil, mapError(err)
	}
	return &emptypb.Empty{}, nil
}

// ListMessages 按创建时间返回当前管理员会话中的持久消息。
func (s *Service) ListMessages(
	ctx context.Context,
	request *assistantv1.ListMessagesRequest,
) (*assistantv1.ListMessagesResponse, error) {
	actorID, err := identity.ActorID(ctx)
	if err != nil {
		return nil, err
	}
	cursor, err := parseMessageCursor(request.GetCursor())
	if err != nil {
		return nil, invalidArgument("invalid message cursor")
	}
	page, err := s.conversations.ListMessages(
		ctx,
		actorID,
		request.GetConversationId(),
		cursor,
		pageLimit(request.GetLimit()),
	)
	if err != nil {
		return nil, mapError(err)
	}
	nextCursor, err := formatMessageCursor(page.NextCursor)
	if err != nil {
		return nil, kerrors.InternalServer("ENCODE_CURSOR_FAILED", "request failed").WithCause(err)
	}
	response := &assistantv1.ListMessagesResponse{
		Messages:   make([]*assistantv1.Message, 0, len(page.Items)),
		NextCursor: nextCursor,
	}
	for _, item := range page.Items {
		response.Messages = append(response.Messages, messageResponse(item))
	}
	return response, nil
}

func mapError(err error) error {
	switch {
	case errors.Is(err, conversationbiz.ErrInvalidTitle):
		return invalidArgument("conversation title is required")
	case errors.Is(err, conversationbiz.ErrNotFound):
		return kerrors.NotFound("RESOURCE_NOT_FOUND", "resource not found")
	default:
		return kerrors.InternalServer("INTERNAL_ERROR", "request failed").WithCause(err)
	}
}

func invalidArgument(reason string) error {
	return kerrors.BadRequest("INVALID_ARGUMENT", reason)
}

func pageLimit(value int32) int {
	if value == 0 {
		return defaultLimit
	}
	return min(int(value), maxLimit)
}

func conversationResponse(item conversationbiz.Conversation) *assistantv1.Conversation {
	return &assistantv1.Conversation{
		Id:        item.ID,
		Title:     item.Title,
		CreatedAt: timestamppb.New(item.CreatedAt),
		UpdatedAt: timestamppb.New(item.UpdatedAt),
	}
}

func messageResponse(item conversationbiz.Message) *assistantv1.Message {
	role := assistantv1.MessageRole_MESSAGE_ROLE_UNSPECIFIED
	switch item.Role {
	case conversationbiz.RoleUser:
		role = assistantv1.MessageRole_MESSAGE_ROLE_USER
	case conversationbiz.RoleAssistant:
		role = assistantv1.MessageRole_MESSAGE_ROLE_ASSISTANT
	}
	return &assistantv1.Message{
		Id:               item.ID,
		ConversationId:   item.ConversationID,
		ExecutionId:      item.ExecutionID,
		Role:             role,
		Content:          item.Content,
		ReasoningContent: item.ReasoningContent,
		CreatedAt:        timestamppb.New(item.CreatedAt),
	}
}
