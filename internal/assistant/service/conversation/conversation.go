package conversation

import (
	"context"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	assistantv1 "github.com/lgc202/ingate/api/assistant/v1"
	conversationbiz "github.com/lgc202/ingate/internal/assistant/biz/conversation"
)

func (s *Service) ListConversations(ctx context.Context, request *assistantv1.ListConversationsRequest) (*assistantv1.ListConversationsResponse, error) {
	actorID, err := s.actorID(ctx)
	if err != nil {
		return nil, err
	}
	cursor, err := decodeConversationCursor(request.GetCursor())
	if err != nil {
		return nil, invalidArgument("invalid conversation cursor")
	}
	page, err := s.conversations.List(ctx, actorID, pageLimit(request.GetLimit()), cursor)
	if err != nil {
		return nil, s.mapError(err)
	}
	nextCursor, err := encodeConversationCursor(page.NextCursor)
	if err != nil {
		return nil, kratoserrors.InternalServer("ENCODE_CURSOR_FAILED", "request failed").WithCause(err)
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

func (s *Service) GetConversation(ctx context.Context, request *assistantv1.GetConversationRequest) (*assistantv1.Conversation, error) {
	actorID, err := s.actorID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.conversations.Get(ctx, actorID, request.GetId())
	if err != nil {
		return nil, s.mapError(err)
	}
	return conversationResponse(item), nil
}

func (s *Service) CreateConversation(ctx context.Context, request *assistantv1.CreateConversationRequest) (*assistantv1.Conversation, error) {
	actorID, err := s.actorID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.conversations.Create(ctx, actorID, request.GetTitle())
	if err != nil {
		return nil, s.mapError(err)
	}
	return conversationResponse(item), nil
}

func (s *Service) DeleteConversation(ctx context.Context, request *assistantv1.DeleteConversationRequest) (*emptypb.Empty, error) {
	actorID, err := s.actorID(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.conversations.Delete(ctx, actorID, request.GetId()); err != nil {
		return nil, s.mapError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) ListMessages(ctx context.Context, request *assistantv1.ListMessagesRequest) (*assistantv1.ListMessagesResponse, error) {
	actorID, err := s.actorID(ctx)
	if err != nil {
		return nil, err
	}
	cursor, err := decodeMessageCursor(request.GetCursor())
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
		return nil, s.mapError(err)
	}
	nextCursor, err := encodeMessageCursor(page.NextCursor)
	if err != nil {
		return nil, kratoserrors.InternalServer("ENCODE_CURSOR_FAILED", "request failed").WithCause(err)
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
		RunId:            item.RunID,
		Role:             role,
		Content:          item.Content,
		ReasoningContent: item.ReasoningContent,
		CreatedAt:        timestamppb.New(item.CreatedAt),
	}
}
