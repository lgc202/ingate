package mysql

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	changebiz "github.com/lgc202/ingate/internal/assistant/biz/change"
	"github.com/lgc202/ingate/internal/assistant/biz/execution"
	"github.com/lgc202/ingate/internal/assistant/data/mysql/db"
)

const (
	proposedChangeKindCreateGateway uint8 = iota + 1
	proposedChangeKindCreateService
)

const (
	proposedChangeStatePendingReview uint8 = iota + 1
	proposedChangeStateExecuting
	proposedChangeStateSucceeded
	proposedChangeStateRejected
	proposedChangeStateFailed
	proposedChangeStateOutcomeUnknown
)

// ListProposedChanges 按创建时间返回会话中已经进入审批流程的配置变更。
func (s *Store) ListProposedChanges(
	ctx context.Context,
	actorID string,
	conversationID string,
) ([]changebiz.ProposedChange, error) {
	if _, err := s.queries.GetConversation(ctx, db.GetConversationParams{
		ID: conversationID, ActorID: actorID,
	}); err != nil {
		return nil, changeNotFound(err)
	}
	rows, err := s.queries.ListProposedChanges(ctx, db.ListProposedChangesParams{
		ConversationID: conversationID,
		ActorID:        actorID,
	})
	if err != nil {
		return nil, fmt.Errorf("list proposed changes: %w", err)
	}
	items := make([]changebiz.ProposedChange, 0, len(rows))
	for _, row := range rows {
		item, err := proposedChangeFromDB(row)
		if err != nil {
			return nil, fmt.Errorf("restore proposed change: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

// ResumeProposedChange 把管理员决定写入等待中的原执行。
// 批准、拒绝和附带反馈的拒绝都只会排队一次，网络重试不会重复恢复 checkpoint。
func (s *Store) ResumeProposedChange(
	ctx context.Context,
	actorID string,
	changeID string,
	approved bool,
	feedback string,
) (changebiz.ProposedChange, error) {
	feedback = strings.TrimSpace(feedback)
	if approved && feedback != "" {
		return changebiz.ProposedChange{}, errors.New("approved change contains rejection feedback")
	}
	if len(feedback) > 65536 {
		return changebiz.ProposedChange{}, errors.New("change feedback exceeds the size limit")
	}
	stored, err := s.queries.GetProposedChange(ctx, db.GetProposedChangeParams{
		ID: changeID, ActorID: actorID,
	})
	if err != nil {
		return changebiz.ProposedChange{}, changeNotFound(err)
	}

	var item changebiz.ProposedChange
	err = s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetConversationForUpdate(ctx, db.GetConversationForUpdateParams{
			ID: stored.ConversationID, ActorID: actorID,
		}); err != nil {
			return changeNotFound(err)
		}
		locked, err := queries.GetProposedChangeForUpdate(
			ctx,
			db.GetProposedChangeForUpdateParams{ID: changeID, ActorID: actorID},
		)
		if err != nil {
			return changeNotFound(err)
		}
		item, err = proposedChangeFromDB(locked)
		if err != nil {
			return fmt.Errorf("restore proposed change: %w", err)
		}
		if item.State != changebiz.StatePendingReview {
			return nil
		}
		executionRow, err := queries.GetExecutionForUpdate(ctx, db.GetExecutionForUpdateParams{
			ID: item.ExecutionID, ActorID: actorID,
		})
		if err != nil {
			return executionNotFound(err)
		}
		if executionRow.State != executionStateWaitingApproval {
			return changebiz.ErrStateConflict
		}

		if approved {
			rows, err := queries.QueueExecutionResume(ctx, db.QueueExecutionResumeParams{
				ResumeInterruptID: locked.InterruptID,
				ResumeDecision:    executionResumeApproved,
				ID:                item.ExecutionID,
			})
			if err != nil {
				return fmt.Errorf("queue approved assistant execution: %w", err)
			}
			if rows != 1 {
				return changebiz.ErrStateConflict
			}
			rows, err = queries.ApproveProposedChange(ctx, changeID)
			if err != nil {
				return fmt.Errorf("approve proposed change: %w", err)
			}
			if rows != 1 {
				return changebiz.ErrStateConflict
			}
		} else {
			if feedback == "" {
				rows, err := queries.QueueExecutionResume(ctx, db.QueueExecutionResumeParams{
					ResumeInterruptID: locked.InterruptID,
					ResumeDecision:    executionResumeRejected,
					ID:                item.ExecutionID,
				})
				if err != nil {
					return fmt.Errorf("queue rejected assistant execution: %w", err)
				}
				if rows != 1 {
					return changebiz.ErrStateConflict
				}
			} else {
				rows, err := queries.QueueExecutionRevision(ctx, db.QueueExecutionRevisionParams{
					ResumeInterruptID: locked.InterruptID,
					ResumeFeedback:    feedback,
					ID:                item.ExecutionID,
				})
				if err != nil {
					return fmt.Errorf("queue assistant change revision: %w", err)
				}
				if rows != 1 {
					return changebiz.ErrStateConflict
				}
				now, err := queries.CurrentTime(ctx)
				if err != nil {
					return fmt.Errorf("read MySQL time: %w", err)
				}
				if err := queries.CreateMessage(ctx, db.CreateMessageParams{
					ID:               uuid.NewString(),
					ConversationID:   item.ConversationID,
					ExecutionID:      item.ExecutionID,
					Role:             messageRoleUser,
					Content:          feedback,
					ReasoningContent: "",
					CreatedAt:        now,
				}); err != nil {
					return fmt.Errorf("create change feedback message: %w", err)
				}
			}
			rows, err := queries.RejectProposedChange(ctx, changeID)
			if err != nil {
				return fmt.Errorf("reject proposed change: %w", err)
			}
			if rows != 1 {
				return changebiz.ErrStateConflict
			}
		}
		if err := queries.TouchConversation(ctx, db.TouchConversationParams{
			ID: item.ConversationID, ActorID: actorID,
		}); err != nil {
			return fmt.Errorf("update conversation activity: %w", err)
		}
		locked, err = queries.GetProposedChangeForUpdate(
			ctx,
			db.GetProposedChangeForUpdateParams{ID: changeID, ActorID: actorID},
		)
		if err != nil {
			return changeNotFound(err)
		}
		item, err = proposedChangeFromDB(locked)
		if err != nil {
			return fmt.Errorf("restore reviewed proposed change: %w", err)
		}
		return nil
	})
	if err != nil {
		return changebiz.ProposedChange{}, fmt.Errorf("resume proposed change transaction: %w", err)
	}
	return item, nil
}

// CompleteProposedChange 在当前 Worker 仍持有原执行租约时收敛写入结果。
func (s *Store) CompleteProposedChange(
	ctx context.Context,
	executionID string,
	workerID string,
	changeID string,
	state changebiz.State,
	resource changebiz.CreatedResource,
	errorCode changebiz.FailureCode,
) error {
	if err := validateCompletion(state, resource, errorCode); err != nil {
		return err
	}
	err := s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetExecutionForWorkerUpdate(ctx, db.GetExecutionForWorkerUpdateParams{
			ID: executionID, WorkerID: workerID,
		}); err != nil {
			return execution.ErrLeaseLost
		}
		item, err := queries.GetProposedChangeForExecutionUpdate(
			ctx,
			db.GetProposedChangeForExecutionUpdateParams{
				ID: changeID, ExecutionID: executionID,
			},
		)
		if err != nil {
			return changeNotFound(err)
		}
		if item.State != proposedChangeStateExecuting {
			return changebiz.ErrStateConflict
		}
		rows, err := queries.CompleteProposedChange(ctx, db.CompleteProposedChangeParams{
			State:      proposedChangeStateToDB(state),
			ResourceID: resource.ID,
			ErrorCode:  string(errorCode),
			ID:         changeID,
		})
		if err != nil {
			return fmt.Errorf("complete proposed change: %w", err)
		}
		if rows != 1 {
			return changebiz.ErrStateConflict
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("complete proposed change transaction: %w", err)
	}
	return nil
}
