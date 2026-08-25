package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lgc202/ingate/internal/assistant/biz/conversation"
	"github.com/lgc202/ingate/internal/assistant/data/mysql/db"
)

// Run Item 的类型和状态使用小整数持久化，避免把可变展示文本写入索引字段。
const (
	runItemKindModelCall uint8 = iota + 1
	runItemKindToolCall
	runItemKindToolResult
	runItemKindDelegation
	runItemKindApproval
)

const (
	runItemStatePending uint8 = iota + 1
	runItemStateRunning
	runItemStateCompleted
	runItemStateFailed
	runItemStateCancelled
)

// StartRunItem 在当前 Worker 持有 Run 租约时追加一个运行中的执行步骤。
// Run 行锁同时保护 sequence 分配，使同一 Run 将来并发追加工具步骤时仍保持稳定顺序。
func (s *Store) StartRunItem(
	ctx context.Context,
	runID string,
	workerID string,
	item conversation.RunItem,
) (conversation.RunItem, error) {
	err := s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetRunForWorkerUpdate(ctx, db.GetRunForWorkerUpdateParams{
			ID: runID, WorkerID: workerID,
		}); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return conversation.ErrRunLeaseLost
			}
			return fmt.Errorf("lock assistant run for item: %w", err)
		}
		sequence, err := queries.NextRunItemSequence(ctx, runID)
		if err != nil {
			return fmt.Errorf("allocate assistant run item sequence: %w", err)
		}
		now := time.Now().UTC()
		item.RunID = runID
		item.Sequence = uint32(sequence)
		item.State = conversation.ItemStateRunning
		item.CreatedAt = now
		item.StartedAt = &now
		if err := queries.CreateRunItem(ctx, db.CreateRunItemParams{
			ID:         item.ID,
			RunID:      item.RunID,
			Sequence:   item.Sequence,
			Kind:       runItemKindToDB(item.Kind),
			State:      runItemStateRunning,
			Name:       item.Name,
			CallID:     item.CallID,
			Summary:    item.Summary,
			ErrorCode:  string(item.ErrorCode),
			CreatedAt:  item.CreatedAt,
			StartedAt:  sql.NullTime{Time: now, Valid: true},
			FinishedAt: sql.NullTime{},
		}); err != nil {
			return fmt.Errorf("create assistant run item: %w", err)
		}
		return nil
	})
	if err != nil {
		return conversation.RunItem{}, fmt.Errorf("start assistant run item transaction: %w", err)
	}
	return item, nil
}

// ListRunItems 按执行顺序返回指定用户可见的 Run Item。
func (s *Store) ListRunItems(ctx context.Context, actorID, runID string) ([]conversation.RunItem, error) {
	if _, err := s.GetRun(ctx, actorID, runID); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListRunItems(ctx, db.ListRunItemsParams{RunID: runID, ActorID: actorID})
	if err != nil {
		return nil, fmt.Errorf("list assistant run items: %w", err)
	}
	items := make([]conversation.RunItem, 0, len(rows))
	for _, row := range rows {
		item, err := runItemFromDB(row)
		if err != nil {
			return nil, fmt.Errorf("decode assistant run item: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

func runItemFromDB(item db.AssistantRunItem) (conversation.RunItem, error) {
	kind, err := runItemKindFromDB(item.Kind)
	if err != nil {
		return conversation.RunItem{}, err
	}
	state, err := runItemStateFromDB(item.State)
	if err != nil {
		return conversation.RunItem{}, err
	}
	result := conversation.RunItem{
		ID:        item.ID,
		RunID:     item.RunID,
		Sequence:  item.Sequence,
		Kind:      kind,
		State:     state,
		Name:      item.Name,
		CallID:    item.CallID,
		Summary:   item.Summary,
		ErrorCode: conversation.FailureCode(item.ErrorCode),
		CreatedAt: item.CreatedAt,
	}
	if item.StartedAt.Valid {
		result.StartedAt = &item.StartedAt.Time
	}
	if item.FinishedAt.Valid {
		result.FinishedAt = &item.FinishedAt.Time
	}
	return result, nil
}

func runItemKindToDB(kind conversation.RunItemKind) uint8 {
	switch kind {
	case conversation.ItemKindModelCall:
		return runItemKindModelCall
	case conversation.ItemKindToolCall:
		return runItemKindToolCall
	case conversation.ItemKindToolResult:
		return runItemKindToolResult
	case conversation.ItemKindDelegation:
		return runItemKindDelegation
	case conversation.ItemKindApproval:
		return runItemKindApproval
	default:
		return 0
	}
}

func runItemKindFromDB(kind uint8) (conversation.RunItemKind, error) {
	switch kind {
	case runItemKindModelCall:
		return conversation.ItemKindModelCall, nil
	case runItemKindToolCall:
		return conversation.ItemKindToolCall, nil
	case runItemKindToolResult:
		return conversation.ItemKindToolResult, nil
	case runItemKindDelegation:
		return conversation.ItemKindDelegation, nil
	case runItemKindApproval:
		return conversation.ItemKindApproval, nil
	default:
		return "", fmt.Errorf("invalid assistant run item kind %d", kind)
	}
}

func runItemStateFromDB(state uint8) (conversation.RunItemState, error) {
	switch state {
	case runItemStatePending:
		return conversation.ItemStatePending, nil
	case runItemStateRunning:
		return conversation.ItemStateRunning, nil
	case runItemStateCompleted:
		return conversation.ItemStateCompleted, nil
	case runItemStateFailed:
		return conversation.ItemStateFailed, nil
	case runItemStateCancelled:
		return conversation.ItemStateCancelled, nil
	default:
		return "", fmt.Errorf("invalid assistant run item state %d", state)
	}
}
