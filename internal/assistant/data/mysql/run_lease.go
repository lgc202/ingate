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

// ClaimRun 使用 SKIP LOCKED 领取最早的排队 Run。
// 数据库锁只覆盖领取事务，模型调用期间由有期限的 worker_id 租约保护。
func (s *Store) ClaimRun(
	ctx context.Context,
	workerID string,
	leaseDuration time.Duration,
) (runbiz.Claimed, bool, error) {
	var claimed runbiz.Claimed
	found := false
	err := s.withTransaction(ctx, func(queries *db.Queries) error {
		row, err := queries.ClaimNextRun(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("select queued assistant run: %w", err)
		}
		now := time.Now().UTC()
		rows, err := queries.StartRun(ctx, db.StartRunParams{
			WorkerID:       workerID,
			LeaseExpiresAt: sql.NullTime{Time: now.Add(leaseDuration), Valid: true},
			StartedAt:      sql.NullTime{Time: now, Valid: true},
			ID:             row.ID,
		})
		if err != nil {
			return fmt.Errorf("start assistant run: %w", err)
		}
		if rows != 1 {
			return runbiz.ErrStateConflict
		}
		claimed = runbiz.Claimed{
			Run: runbiz.Run{
				ID:             row.ID,
				ConversationID: row.ConversationID,
				State:          runbiz.StateRunning,
				CreatedAt:      row.CreatedAt,
				StartedAt:      &now,
			},
			ActorID: row.ActorID,
		}
		found = true
		return nil
	})
	if err != nil {
		return runbiz.Claimed{}, false, fmt.Errorf("claim assistant run transaction: %w", err)
	}
	return claimed, found, nil
}

// SetRunModel 记录当前租约实际选中的模型，排队阶段不提前固化在线配置。
func (s *Store) SetRunModel(ctx context.Context, runID, workerID, model string) error {
	rows, err := s.queries.SetRunModel(ctx, db.SetRunModelParams{
		Model: model, ID: runID, WorkerID: workerID,
	})
	if err != nil {
		return fmt.Errorf("set assistant run model: %w", err)
	}
	if rows != 1 {
		return runbiz.ErrLeaseLost
	}
	return nil
}

// RenewRunLease 延长当前实例的租约，并返回用户是否请求取消。
func (s *Store) RenewRunLease(
	ctx context.Context,
	runID string,
	workerID string,
	leaseDuration time.Duration,
) (bool, error) {
	rows, err := s.queries.RenewRunLease(ctx, db.RenewRunLeaseParams{
		LeaseExpiresAt: sql.NullTime{Time: time.Now().UTC().Add(leaseDuration), Valid: true},
		ID:             runID,
		WorkerID:       workerID,
	})
	if err != nil {
		return false, fmt.Errorf("renew assistant run lease: %w", err)
	}
	if rows != 1 {
		return false, runbiz.ErrLeaseLost
	}
	cancelRequested, err := s.queries.RunCancellationRequested(ctx, db.RunCancellationRequestedParams{
		ID: runID, WorkerID: workerID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return false, runbiz.ErrLeaseLost
	}
	if err != nil {
		return false, fmt.Errorf("read assistant run cancellation: %w", err)
	}
	return cancelRequested, nil
}

// FailExpiredRuns 终止已经失去执行实例的 Run。
// 不自动重新排队，避免未来 Run 包含有副作用的工具调用时重复执行。
func (s *Store) FailExpiredRuns(ctx context.Context) (int64, error) {
	var rows int64
	err := s.withTransaction(ctx, func(queries *db.Queries) error {
		now := time.Now().UTC()
		if err := queries.FailExpiredRunItems(ctx, db.FailExpiredRunItemsParams{
			ErrorCode:      string(runbiz.FailureWorkerLost),
			FinishedAt:     sql.NullTime{Time: now, Valid: true},
			LeaseExpiresAt: sql.NullTime{Time: now, Valid: true},
		}); err != nil {
			return fmt.Errorf("fail expired assistant run items: %w", err)
		}
		var err error
		rows, err = queries.FailExpiredRuns(ctx, db.FailExpiredRunsParams{
			ErrorCode:      string(runbiz.FailureWorkerLost),
			FinishedAt:     sql.NullTime{Time: now, Valid: true},
			LeaseExpiresAt: sql.NullTime{Time: now, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("fail expired assistant runs: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("fail expired assistant runs transaction: %w", err)
	}
	return rows, nil
}
