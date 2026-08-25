package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	runbiz "github.com/lgc202/ingate/internal/assistant/biz/run"
	"github.com/lgc202/ingate/internal/assistant/data/mysql/db"
)

// Run Item 的类型和状态使用小整数持久化，避免把可变展示文本写入索引字段。
const (
	runItemKindModelCall uint8 = iota + 1
	runItemKindToolCall
)

const (
	runItemStateRunning uint8 = iota + 1
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
	item runbiz.Item,
) (runbiz.Item, error) {
	err := s.withTransaction(ctx, func(queries *db.Queries) error {
		if _, err := queries.GetRunForWorkerUpdate(ctx, db.GetRunForWorkerUpdateParams{
			ID: runID, WorkerID: workerID,
		}); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return runbiz.ErrLeaseLost
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
		item.State = runbiz.ItemStateRunning
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
			StartedAt:  now,
			FinishedAt: sql.NullTime{},
		}); err != nil {
			return fmt.Errorf("create assistant run item: %w", err)
		}
		return nil
	})
	if err != nil {
		return runbiz.Item{}, fmt.Errorf("start assistant run item transaction: %w", err)
	}
	return item, nil
}

// CompleteRunItem 只结束当前 Worker 实际执行成功的步骤。
// Run 终态不再批量补齐运行中步骤，避免把未完成的模型或工具调用伪装成成功。
func (s *Store) CompleteRunItem(
	ctx context.Context,
	runID string,
	workerID string,
	callID string,
	kind runbiz.ItemKind,
	summary string,
) error {
	rows, err := s.queries.CompleteRunItem(ctx, db.CompleteRunItemParams{
		Summary:    summary,
		FinishedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
		RunID:      runID,
		CallID:     callID,
		Kind:       runItemKindToDB(kind),
		WorkerID:   workerID,
	})
	if err != nil {
		return fmt.Errorf("complete assistant run item: %w", err)
	}
	if rows != 1 {
		return runbiz.ErrLeaseLost
	}
	return nil
}

// FailRunItem 记录工具调用等单个步骤的稳定失败码，原始错误只沿调用链返回日志边界。
func (s *Store) FailRunItem(
	ctx context.Context,
	runID string,
	workerID string,
	callID string,
	kind runbiz.ItemKind,
	code runbiz.FailureCode,
) error {
	rows, err := s.queries.FailRunItem(ctx, db.FailRunItemParams{
		ErrorCode:  string(code),
		FinishedAt: sql.NullTime{Time: time.Now().UTC(), Valid: true},
		RunID:      runID,
		CallID:     callID,
		Kind:       runItemKindToDB(kind),
		WorkerID:   workerID,
	})
	if err != nil {
		return fmt.Errorf("fail assistant run item: %w", err)
	}
	if rows != 1 {
		return runbiz.ErrLeaseLost
	}
	return nil
}

// ListRunItems 按执行顺序返回指定用户可见的 Run Item。
func (s *Store) ListRunItems(ctx context.Context, actorID, runID string) ([]runbiz.Item, error) {
	if _, err := s.GetRun(ctx, actorID, runID); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListRunItems(ctx, db.ListRunItemsParams{RunID: runID, ActorID: actorID})
	if err != nil {
		return nil, fmt.Errorf("list assistant run items: %w", err)
	}
	items := make([]runbiz.Item, 0, len(rows))
	for _, row := range rows {
		item, err := runItemFromDB(row)
		if err != nil {
			return nil, fmt.Errorf("decode assistant run item: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

func runItemFromDB(item db.AssistantRunItem) (runbiz.Item, error) {
	kind, err := runItemKindFromDB(item.Kind)
	if err != nil {
		return runbiz.Item{}, err
	}
	state, err := runItemStateFromDB(item.State)
	if err != nil {
		return runbiz.Item{}, err
	}
	result := runbiz.Item{
		ID:        item.ID,
		RunID:     item.RunID,
		Sequence:  item.Sequence,
		Kind:      kind,
		State:     state,
		Name:      item.Name,
		CallID:    item.CallID,
		Summary:   item.Summary,
		ErrorCode: runbiz.FailureCode(item.ErrorCode),
		CreatedAt: item.CreatedAt,
	}
	result.StartedAt = &item.StartedAt
	if item.FinishedAt.Valid {
		result.FinishedAt = &item.FinishedAt.Time
	}
	return result, nil
}

func runItemKindToDB(kind runbiz.ItemKind) uint8 {
	switch kind {
	case runbiz.ItemKindModelCall:
		return runItemKindModelCall
	case runbiz.ItemKindToolCall:
		return runItemKindToolCall
	default:
		return 0
	}
}

func runItemKindFromDB(kind uint8) (runbiz.ItemKind, error) {
	switch kind {
	case runItemKindModelCall:
		return runbiz.ItemKindModelCall, nil
	case runItemKindToolCall:
		return runbiz.ItemKindToolCall, nil
	default:
		return "", fmt.Errorf("invalid assistant run item kind %d", kind)
	}
}

func runItemStateFromDB(state uint8) (runbiz.ItemState, error) {
	switch state {
	case runItemStateRunning:
		return runbiz.ItemStateRunning, nil
	case runItemStateCompleted:
		return runbiz.ItemStateCompleted, nil
	case runItemStateFailed:
		return runbiz.ItemStateFailed, nil
	case runItemStateCancelled:
		return runbiz.ItemStateCancelled, nil
	default:
		return "", fmt.Errorf("invalid assistant run item state %d", state)
	}
}
