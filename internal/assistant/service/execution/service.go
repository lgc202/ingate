// Package execution 适配 Agent 异步执行的产品协议。
package execution

import (
	"context"
	"errors"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	assistantv1 "github.com/lgc202/ingate/api/assistant/v1"
	executionbiz "github.com/lgc202/ingate/internal/assistant/biz/execution"
	"github.com/lgc202/ingate/internal/assistant/service/identity"
)

// Service 实现执行创建、查询和取消协议。
type Service struct {
	assistantv1.UnimplementedAgentExecutionServiceServer

	executions *executionbiz.Service
}

// NewService 创建 Agent 执行协议服务。
func NewService(executions *executionbiz.Service) *Service {
	return &Service{executions: executions}
}

func (s *Service) CreateAgentExecution(
	ctx context.Context,
	request *assistantv1.CreateAgentExecutionRequest,
) (*assistantv1.AgentExecution, error) {
	actorID, err := identity.ActorID(ctx)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(request.GetContent())
	if content == "" {
		return nil, kratoserrors.BadRequest("INVALID_ARGUMENT", "message content is required")
	}
	item, err := s.executions.Create(ctx, actorID, request.GetConversationId(), content)
	if err != nil {
		return nil, mapError(err)
	}
	return executionResponse(item), nil
}

func (s *Service) GetAgentExecution(
	ctx context.Context,
	request *assistantv1.GetAgentExecutionRequest,
) (*assistantv1.AgentExecution, error) {
	actorID, err := identity.ActorID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.executions.Get(ctx, actorID, request.GetId())
	if err != nil {
		return nil, mapError(err)
	}
	return executionResponse(item), nil
}

func (s *Service) ListAgentExecutionSteps(
	ctx context.Context,
	request *assistantv1.ListAgentExecutionStepsRequest,
) (*assistantv1.ListAgentExecutionStepsResponse, error) {
	actorID, err := identity.ActorID(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.executions.ListSteps(ctx, actorID, request.GetExecutionId())
	if err != nil {
		return nil, mapError(err)
	}
	response := &assistantv1.ListAgentExecutionStepsResponse{
		Steps: make([]*assistantv1.AgentExecutionStep, 0, len(items)),
	}
	for _, item := range items {
		response.Steps = append(response.Steps, stepResponse(item))
	}
	return response, nil
}

func (s *Service) CancelAgentExecution(
	ctx context.Context,
	request *assistantv1.CancelAgentExecutionRequest,
) (*assistantv1.AgentExecution, error) {
	actorID, err := identity.ActorID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.executions.Cancel(ctx, actorID, request.GetId())
	if err != nil {
		return nil, mapError(err)
	}
	return executionResponse(item), nil
}

func mapError(err error) error {
	switch {
	case errors.Is(err, executionbiz.ErrNotFound):
		return kratoserrors.NotFound("RESOURCE_NOT_FOUND", "resource not found")
	case errors.Is(err, executionbiz.ErrStateConflict), errors.Is(err, executionbiz.ErrConversationBusy):
		return kratoserrors.Conflict("RESOURCE_CONFLICT", "resource state changed")
	default:
		return kratoserrors.InternalServer("INTERNAL_ERROR", "request failed").WithCause(err)
	}
}

func executionResponse(item executionbiz.AgentExecution) *assistantv1.AgentExecution {
	state := assistantv1.AgentExecutionState_AGENT_EXECUTION_STATE_UNSPECIFIED
	switch item.State {
	case executionbiz.StateQueued:
		state = assistantv1.AgentExecutionState_AGENT_EXECUTION_STATE_QUEUED
	case executionbiz.StateRunning:
		state = assistantv1.AgentExecutionState_AGENT_EXECUTION_STATE_RUNNING
	case executionbiz.StateSucceeded:
		state = assistantv1.AgentExecutionState_AGENT_EXECUTION_STATE_SUCCEEDED
	case executionbiz.StateFailed:
		state = assistantv1.AgentExecutionState_AGENT_EXECUTION_STATE_FAILED
	case executionbiz.StateCancelled:
		state = assistantv1.AgentExecutionState_AGENT_EXECUTION_STATE_CANCELLED
	}
	response := &assistantv1.AgentExecution{
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

func stepResponse(item executionbiz.Step) *assistantv1.AgentExecutionStep {
	kind := assistantv1.AgentExecutionStepKind_AGENT_EXECUTION_STEP_KIND_UNSPECIFIED
	switch item.Kind {
	case executionbiz.StepKindModelCall:
		kind = assistantv1.AgentExecutionStepKind_AGENT_EXECUTION_STEP_KIND_MODEL_CALL
	case executionbiz.StepKindToolCall:
		kind = assistantv1.AgentExecutionStepKind_AGENT_EXECUTION_STEP_KIND_TOOL_CALL
	}
	state := assistantv1.AgentExecutionStepState_AGENT_EXECUTION_STEP_STATE_UNSPECIFIED
	switch item.State {
	case executionbiz.StepStateRunning:
		state = assistantv1.AgentExecutionStepState_AGENT_EXECUTION_STEP_STATE_RUNNING
	case executionbiz.StepStateCompleted:
		state = assistantv1.AgentExecutionStepState_AGENT_EXECUTION_STEP_STATE_COMPLETED
	case executionbiz.StepStateFailed:
		state = assistantv1.AgentExecutionStepState_AGENT_EXECUTION_STEP_STATE_FAILED
	case executionbiz.StepStateCancelled:
		state = assistantv1.AgentExecutionStepState_AGENT_EXECUTION_STEP_STATE_CANCELLED
	}
	response := &assistantv1.AgentExecutionStep{
		Id:          item.ID,
		ExecutionId: item.ExecutionID,
		Sequence:    item.Sequence,
		Kind:        kind,
		State:       state,
		Name:        item.Name,
		Summary:     item.Summary,
		ErrorCode:   string(item.ErrorCode),
		CreatedAt:   timestamppb.New(item.CreatedAt),
	}
	if item.StartedAt != nil {
		response.StartedAt = timestamppb.New(*item.StartedAt)
	}
	if item.FinishedAt != nil {
		response.FinishedAt = timestamppb.New(*item.FinishedAt)
	}
	return response
}
