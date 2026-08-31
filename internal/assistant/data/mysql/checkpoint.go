package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lgc202/ingate/internal/assistant/data/mysql/db"
)

// CheckpointStore 只暴露 Eino Runner 要求的 Get 和 Set，避免这些通用方法名
// 污染同时承担会话存储职责的 Store API。
type CheckpointStore struct {
	store *Store
}

// NewCheckpointStore 创建基于同一 MySQL 连接池的 Eino checkpoint 存储。
func NewCheckpointStore(store *Store) *CheckpointStore {
	return &CheckpointStore{store: store}
}

// Get 读取 Eino 保存的执行 checkpoint。
func (s *CheckpointStore) Get(ctx context.Context, executionID string) ([]byte, bool, error) {
	checkpoint, err := s.store.queries.GetCheckpoint(ctx, executionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get assistant checkpoint: %w", err)
	}
	if len(checkpoint) == 0 {
		return nil, false, errors.New("stored assistant checkpoint is empty")
	}
	return checkpoint, true, nil
}

// Set 原子写入或替换一次 Eino checkpoint。
func (s *CheckpointStore) Set(ctx context.Context, executionID string, checkpoint []byte) error {
	if len(checkpoint) == 0 {
		return errors.New("assistant checkpoint is empty")
	}
	if err := s.store.queries.UpsertCheckpoint(ctx, db.UpsertCheckpointParams{
		ExecutionID: executionID,
		Checkpoint:  checkpoint,
	}); err != nil {
		return fmt.Errorf("store assistant checkpoint: %w", err)
	}
	return nil
}
