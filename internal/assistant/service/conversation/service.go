// Package conversation 适配运维助手的 HTTP API 与会话业务对象。
package conversation

import (
	"context"
	"errors"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	assistantv1 "github.com/lgc202/ingate/api/assistant/v1"
	conversationbiz "github.com/lgc202/ingate/internal/assistant/biz/conversation"
)

const (
	forwardedUserHeader = "X-Forwarded-User"
	maxActorIDLength    = 128
	defaultLimit        = 20
	maxLimit            = 100
)

// Service 实现会话、消息和 Run 状态的产品协议。
type Service struct {
	conversations *conversationbiz.Service
}

// NewService 创建会话协议服务。
func NewService(conversations *conversationbiz.Service) *Service {
	return &Service{conversations: conversations}
}

func (s *Service) ListConversations(
	ctx context.Context,
	request *assistantv1.ListConversationsRequest,
) (*assistantv1.ListConversationsResponse, error) {
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
		response.Conversations = append(response.Conversations, s.conversationResponse(item))
	}
	return response, nil
}

func (s *Service) GetConversation(
	ctx context.Context,
	request *assistantv1.GetConversationRequest,
) (*assistantv1.Conversation, error) {
	actorID, err := s.actorID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.conversations.Get(ctx, actorID, request.GetId())
	if err != nil {
		return nil, s.mapError(err)
	}
	return s.conversationResponse(item), nil
}

func (s *Service) CreateConversation(
	ctx context.Context,
	request *assistantv1.CreateConversationRequest,
) (*assistantv1.Conversation, error) {
	actorID, err := s.actorID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.conversations.Create(ctx, actorID, request.GetTitle())
	if err != nil {
		return nil, s.mapError(err)
	}
	return s.conversationResponse(item), nil
}

func (s *Service) DeleteConversation(
	ctx context.Context,
	request *assistantv1.DeleteConversationRequest,
) (*emptypb.Empty, error) {
	actorID, err := s.actorID(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.conversations.Delete(ctx, actorID, request.GetId()); err != nil {
		return nil, s.mapError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) ListMessages(
	ctx context.Context,
	request *assistantv1.ListMessagesRequest,
) (*assistantv1.ListMessagesResponse, error) {
	actorID, err := s.actorID(ctx)
	if err != nil {
		return nil, err
	}
	cursor, err := decodeMessageCursor(request.GetCursor())
	if err != nil {
		return nil, invalidArgument("invalid message cursor")
	}
	page, err := s.conversations.ListMessages(
		ctx, actorID, request.GetConversationId(), cursor, pageLimit(request.GetLimit()),
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
		response.Messages = append(response.Messages, s.messageResponse(item))
	}
	return response, nil
}

func (s *Service) CreateRun(
	ctx context.Context,
	request *assistantv1.CreateRunRequest,
) (*assistantv1.Run, error) {
	actorID, err := s.actorID(ctx)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(request.GetContent())
	if content == "" {
		return nil, invalidArgument("message content is required")
	}
	run, err := s.conversations.CreateRun(ctx, actorID, request.GetConversationId(), content)
	if err != nil {
		return nil, s.mapError(err)
	}
	return s.runResponse(run), nil
}

func (s *Service) GetRun(
	ctx context.Context,
	request *assistantv1.GetRunRequest,
) (*assistantv1.Run, error) {
	actorID, err := s.actorID(ctx)
	if err != nil {
		return nil, err
	}
	run, err := s.conversations.GetRun(ctx, actorID, request.GetId())
	if err != nil {
		return nil, s.mapError(err)
	}
	return s.runResponse(run), nil
}

func (s *Service) ListRunItems(
	ctx context.Context,
	request *assistantv1.ListRunItemsRequest,
) (*assistantv1.ListRunItemsResponse, error) {
	actorID, err := s.actorID(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.conversations.ListRunItems(ctx, actorID, request.GetRunId())
	if err != nil {
		return nil, s.mapError(err)
	}
	response := &assistantv1.ListRunItemsResponse{Items: make([]*assistantv1.RunItem, 0, len(items))}
	for _, item := range items {
		response.Items = append(response.Items, s.runItemResponse(item))
	}
	return response, nil
}

func (s *Service) CancelRun(
	ctx context.Context,
	request *assistantv1.CancelRunRequest,
) (*assistantv1.Run, error) {
	actorID, err := s.actorID(ctx)
	if err != nil {
		return nil, err
	}
	run, err := s.conversations.CancelRun(ctx, actorID, request.GetId())
	if err != nil {
		return nil, s.mapError(err)
	}
	return s.runResponse(run), nil
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
	case errors.Is(err, conversationbiz.ErrNotFound):
		return kratoserrors.NotFound("RESOURCE_NOT_FOUND", "resource not found")
	case errors.Is(err, conversationbiz.ErrRunStateConflict), errors.Is(err, conversationbiz.ErrRunRunning):
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

func (s *Service) conversationResponse(item conversationbiz.Conversation) *assistantv1.Conversation {
	return &assistantv1.Conversation{
		Id:        item.ID,
		Title:     item.Title,
		CreatedAt: timestamppb.New(item.CreatedAt),
		UpdatedAt: timestamppb.New(item.UpdatedAt),
	}
}

func (s *Service) messageResponse(item conversationbiz.Message) *assistantv1.Message {
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

func (s *Service) runResponse(item conversationbiz.Run) *assistantv1.Run {
	state := assistantv1.RunState_RUN_STATE_UNSPECIFIED
	switch item.State {
	case conversationbiz.StateQueued:
		state = assistantv1.RunState_RUN_STATE_QUEUED
	case conversationbiz.StateRunning:
		state = assistantv1.RunState_RUN_STATE_RUNNING
	case conversationbiz.StateSucceeded:
		state = assistantv1.RunState_RUN_STATE_SUCCEEDED
	case conversationbiz.StateFailed:
		state = assistantv1.RunState_RUN_STATE_FAILED
	case conversationbiz.StateCancelled:
		state = assistantv1.RunState_RUN_STATE_CANCELLED
	}
	response := &assistantv1.Run{
		Id:                    item.ID,
		ConversationId:        item.ConversationID,
		State:                 state,
		Model:                 item.Model,
		ErrorCode:             string(item.ErrorCode),
		CreatedAt:             timestamppb.New(item.CreatedAt),
		CancellationRequested: item.CancellationRequested,
	}
	if item.StartedAt != nil {
		response.StartedAt = timestamppb.New(*item.StartedAt)
	}
	if item.FinishedAt != nil {
		response.FinishedAt = timestamppb.New(*item.FinishedAt)
	}
	return response
}

func (s *Service) runItemResponse(item conversationbiz.RunItem) *assistantv1.RunItem {
	kind := assistantv1.RunItemKind_RUN_ITEM_KIND_UNSPECIFIED
	switch item.Kind {
	case conversationbiz.ItemKindModelCall:
		kind = assistantv1.RunItemKind_RUN_ITEM_KIND_MODEL_CALL
	case conversationbiz.ItemKindToolCall:
		kind = assistantv1.RunItemKind_RUN_ITEM_KIND_TOOL_CALL
	case conversationbiz.ItemKindToolResult:
		kind = assistantv1.RunItemKind_RUN_ITEM_KIND_TOOL_RESULT
	case conversationbiz.ItemKindDelegation:
		kind = assistantv1.RunItemKind_RUN_ITEM_KIND_DELEGATION
	case conversationbiz.ItemKindApproval:
		kind = assistantv1.RunItemKind_RUN_ITEM_KIND_APPROVAL
	}
	state := assistantv1.RunItemState_RUN_ITEM_STATE_UNSPECIFIED
	switch item.State {
	case conversationbiz.ItemStatePending:
		state = assistantv1.RunItemState_RUN_ITEM_STATE_PENDING
	case conversationbiz.ItemStateRunning:
		state = assistantv1.RunItemState_RUN_ITEM_STATE_RUNNING
	case conversationbiz.ItemStateCompleted:
		state = assistantv1.RunItemState_RUN_ITEM_STATE_COMPLETED
	case conversationbiz.ItemStateFailed:
		state = assistantv1.RunItemState_RUN_ITEM_STATE_FAILED
	case conversationbiz.ItemStateCancelled:
		state = assistantv1.RunItemState_RUN_ITEM_STATE_CANCELLED
	}
	response := &assistantv1.RunItem{
		Id:        item.ID,
		RunId:     item.RunID,
		Sequence:  item.Sequence,
		Kind:      kind,
		State:     state,
		Name:      item.Name,
		CallId:    item.CallID,
		Summary:   item.Summary,
		ErrorCode: string(item.ErrorCode),
		CreatedAt: timestamppb.New(item.CreatedAt),
	}
	if item.StartedAt != nil {
		response.StartedAt = timestamppb.New(*item.StartedAt)
	}
	if item.FinishedAt != nil {
		response.FinishedAt = timestamppb.New(*item.FinishedAt)
	}
	return response
}
