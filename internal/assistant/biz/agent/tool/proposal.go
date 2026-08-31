package tool

import (
	"context"
	"errors"
	"fmt"
	"strings"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	changebiz "github.com/lgc202/ingate/internal/assistant/biz/change"
)

const (
	approvalRequestSchema = "ingate_assistant_change_approval_request_v1"
	approvalResultSchema  = "ingate_assistant_change_approval_result_v1"
)

// ApprovalRequest 是 Eino 中断向审批界面公开的强类型信息，也是恢复工具所需的状态。
// 类型名带版本注册，避免 checkpoint 反序列化依赖 Go 包路径或类型重命名。
type ApprovalRequest struct {
	ChangeID string
	CallID   string
	Summary  string
	Proposal changebiz.Proposal
}

// ApprovalResult 是审批界面恢复当前 Eino 中断时提交的唯一数据。
// Feedback 非空表示用户拒绝当前参数，并要求 Agent 根据文字继续处理。
type ApprovalResult struct {
	Approved bool
	Feedback string
}

type proposalToolOutput struct {
	Summary  string              `json:"summary"`
	Status   string              `json:"status"`
	Proposal *changebiz.Proposal `json:"proposal,omitempty"`
}

type changeToolOutput struct {
	Summary    string `json:"summary"`
	Status     string `json:"status"`
	ChangeID   string `json:"change_id,omitempty"`
	ResourceID string `json:"resource_id,omitempty"`
	ErrorCode  string `json:"error_code,omitempty"`
}

type approvalInterruptError struct {
	cause error
}

func init() {
	schema.RegisterName[*ApprovalRequest](approvalRequestSchema)
	schema.RegisterName[*ApprovalResult](approvalResultSchema)
}

func (e *approvalInterruptError) Error() string {
	return e.cause.Error()
}

func (e *approvalInterruptError) Unwrap() error {
	return e.cause
}

// IsApprovalInterrupt 区分预期的 Eino 审批中断与真实工具故障。
// 包装仍保留原始 interrupt signal，Eino 可以通过 errors.As 正常建立 checkpoint。
func IsApprovalInterrupt(err error) bool {
	var interrupt *approvalInterruptError
	return errors.As(err, &interrupt)
}

func proposalInputResult(err error) (proposalToolOutput, error) {
	reason, ok := invalidInputReason(err)
	if !ok {
		return proposalToolOutput{}, err
	}
	return proposalToolOutput{
		Summary: reason,
		Status:  "invalid_input",
	}, nil
}

func executeWithApproval(
	ctx context.Context,
	writer ChangeWriter,
	prepared proposalToolOutput,
) (changeToolOutput, error) {
	wasInterrupted, hasState, request := einotool.GetInterruptState[*ApprovalRequest](ctx)
	if !wasInterrupted {
		if prepared.Proposal == nil || prepared.Status != "approval_required" {
			return changeToolOutput{}, errors.New("prepared change contains no approval request")
		}
		callID := compose.GetToolCallID(ctx)
		if callID == "" {
			return changeToolOutput{}, errors.New("change tool call has no stable call ID")
		}
		request = &ApprovalRequest{
			ChangeID: uuid.NewString(),
			CallID:   callID,
			Summary:  prepared.Summary,
			Proposal: *prepared.Proposal,
		}
		return changeToolOutput{}, approvalInterrupt(
			einotool.StatefulInterrupt(ctx, request, request),
		)
	}
	if !hasState || request == nil {
		return changeToolOutput{}, errors.New("change approval checkpoint contains no request")
	}
	if err := request.validate(); err != nil {
		return changeToolOutput{}, fmt.Errorf("validate change approval checkpoint: %w", err)
	}

	isTarget, hasResult, result := einotool.GetResumeContext[*ApprovalResult](ctx)
	if !isTarget {
		return changeToolOutput{}, approvalInterrupt(
			einotool.StatefulInterrupt(ctx, request, request),
		)
	}
	if !hasResult || result == nil {
		return changeToolOutput{}, errors.New("change approval resume contains no result")
	}
	if !result.Approved {
		summary := "用户拒绝了当前配置变更"
		if feedback := strings.TrimSpace(result.Feedback); feedback != "" {
			summary = "用户拒绝了当前配置，并要求调整：" + feedback
		}
		return changeToolOutput{
			Summary:  summary,
			Status:   "change_rejected",
			ChangeID: request.ChangeID,
		}, nil
	}

	resource, err := writeChange(ctx, writer, request.Proposal)
	if err == nil {
		return changeToolOutput{
			Summary:    "配置变更已执行",
			Status:     "change_succeeded",
			ChangeID:   request.ChangeID,
			ResourceID: resource.ID,
		}, nil
	}
	if errors.Is(err, changebiz.ErrAdminRejected) {
		return changeToolOutput{
			Summary:   "管理服务明确拒绝了配置变更",
			Status:    "change_failed",
			ChangeID:  request.ChangeID,
			ErrorCode: string(changebiz.FailureAdminRejected),
		}, nil
	}
	return changeToolOutput{
		Summary:   "配置变更结果无法确认，系统不会自动重试",
		Status:    "change_outcome_unknown",
		ChangeID:  request.ChangeID,
		ErrorCode: string(changebiz.FailureOutcomeUnknown),
	}, nil
}

func approvalInterrupt(cause error) error {
	return &approvalInterruptError{cause: cause}
}

func writeChange(
	ctx context.Context,
	writer ChangeWriter,
	proposal changebiz.Proposal,
) (changebiz.CreatedResource, error) {
	switch proposal.Kind {
	case changebiz.KindCreateGateway:
		return writer.CreateGateway(ctx, *proposal.Gateway)
	case changebiz.KindCreateService:
		return writer.CreateService(ctx, *proposal.Service)
	default:
		return changebiz.CreatedResource{}, fmt.Errorf(
			"unsupported approved change kind %q",
			proposal.Kind,
		)
	}
}

func (r *ApprovalRequest) validate() error {
	if r == nil || uuid.Validate(r.ChangeID) != nil || strings.TrimSpace(r.CallID) == "" ||
		strings.TrimSpace(r.Summary) == "" || strings.TrimSpace(r.Summary) != r.Summary {
		return errors.New("invalid approval request metadata")
	}
	return r.Proposal.Validate()
}
