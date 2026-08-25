package conversation

import (
	"context"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	assistantv1 "github.com/lgc202/ingate/api/assistant/v1"
	runbiz "github.com/lgc202/ingate/internal/assistant/biz/run"
)

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
	item, err := s.runs.Create(ctx, actorID, request.GetConversationId(), content)
	if err != nil {
		return nil, s.mapError(err)
	}
	return runResponse(item), nil
}

func (s *Service) GetRun(
	ctx context.Context,
	request *assistantv1.GetRunRequest,
) (*assistantv1.Run, error) {
	actorID, err := s.actorID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.runs.Get(ctx, actorID, request.GetId())
	if err != nil {
		return nil, s.mapError(err)
	}
	return runResponse(item), nil
}

func (s *Service) ListRunItems(
	ctx context.Context,
	request *assistantv1.ListRunItemsRequest,
) (*assistantv1.ListRunItemsResponse, error) {
	actorID, err := s.actorID(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.runs.ListItems(ctx, actorID, request.GetRunId())
	if err != nil {
		return nil, s.mapError(err)
	}
	response := &assistantv1.ListRunItemsResponse{Items: make([]*assistantv1.RunItem, 0, len(items))}
	for _, item := range items {
		response.Items = append(response.Items, runItemResponse(item))
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
	item, err := s.runs.Cancel(ctx, actorID, request.GetId())
	if err != nil {
		return nil, s.mapError(err)
	}
	return runResponse(item), nil
}

func runResponse(item runbiz.Run) *assistantv1.Run {
	state := assistantv1.RunState_RUN_STATE_UNSPECIFIED
	switch item.State {
	case runbiz.StateQueued:
		state = assistantv1.RunState_RUN_STATE_QUEUED
	case runbiz.StateRunning:
		state = assistantv1.RunState_RUN_STATE_RUNNING
	case runbiz.StateSucceeded:
		state = assistantv1.RunState_RUN_STATE_SUCCEEDED
	case runbiz.StateFailed:
		state = assistantv1.RunState_RUN_STATE_FAILED
	case runbiz.StateCancelled:
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

func runItemResponse(item runbiz.Item) *assistantv1.RunItem {
	kind := assistantv1.RunItemKind_RUN_ITEM_KIND_UNSPECIFIED
	switch item.Kind {
	case runbiz.ItemKindModelCall:
		kind = assistantv1.RunItemKind_RUN_ITEM_KIND_MODEL_CALL
	case runbiz.ItemKindToolCall:
		kind = assistantv1.RunItemKind_RUN_ITEM_KIND_TOOL_CALL
	}
	state := assistantv1.RunItemState_RUN_ITEM_STATE_UNSPECIFIED
	switch item.State {
	case runbiz.ItemStateRunning:
		state = assistantv1.RunItemState_RUN_ITEM_STATE_RUNNING
	case runbiz.ItemStateCompleted:
		state = assistantv1.RunItemState_RUN_ITEM_STATE_COMPLETED
	case runbiz.ItemStateFailed:
		state = assistantv1.RunItemState_RUN_ITEM_STATE_FAILED
	case runbiz.ItemStateCancelled:
		state = assistantv1.RunItemState_RUN_ITEM_STATE_CANCELLED
	}
	response := &assistantv1.RunItem{
		Id:        item.ID,
		RunId:     item.RunID,
		Sequence:  item.Sequence,
		Kind:      kind,
		State:     state,
		Name:      item.Name,
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
