package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lgc202/ingate/internal/assistant/biz/execution"
	"github.com/lgc202/ingate/internal/assistant/data/mysql/db"
)

// ClaimExecution 使用 SKIP LOCKED 领取最早的排队执行。
// 数据库锁只覆盖领取事务，模型调用期间由有期限的 worker_id 租约保护。
func (s *Store) ClaimExecution(
	ctx context.Context,
	workerID string,
	leaseDuration time.Duration,
) (execution.Claim, bool, error) {
	var claim execution.Claim
	found := false
	err := s.withTransaction(ctx, func(queries *db.Queries) error {
		row, err := queries.ClaimNextExecution(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("select queued assistant execution: %w", err)
		}
		rows, err := queries.StartExecution(ctx, db.StartExecutionParams{
			WorkerID:                  workerID,
			LeaseDurationMicroseconds: leaseDuration.Microseconds(),
			ID:                        row.ID,
		})
		if err != nil {
			return fmt.Errorf("start assistant execution: %w", err)
		}
		if rows != 1 {
			return execution.ErrStateConflict
		}
		claim = execution.Claim{
			ID:             row.ID,
			ConversationID: row.ConversationID,
			ActorID:        row.ActorID,
		}
		found = true
		return nil
	})
	if err != nil {
		return execution.Claim{}, false, fmt.Errorf("claim assistant execution transaction: %w", err)
	}
	return claim, found, nil
}

// SetExecutionModel 记录当前租约实际选中的模型，排队阶段不提前固化在线配置。
func (s *Store) SetExecutionModel(ctx context.Context, executionID, workerID, model string) error {
	rows, err := s.queries.SetExecutionModel(ctx, db.SetExecutionModelParams{
		Model: model, ID: executionID, WorkerID: workerID,
	})
	if err != nil {
		return fmt.Errorf("set assistant execution model: %w", err)
	}
	if rows != 1 {
		return execution.ErrLeaseLost
	}
	return nil
}

// RenewExecutionLease 延长当前实例的租约，并返回用户是否请求取消。
func (s *Store) RenewExecutionLease(
	ctx context.Context,
	executionID string,
	workerID string,
	leaseDuration time.Duration,
) (bool, error) {
	rows, err := s.queries.RenewExecutionLease(ctx, db.RenewExecutionLeaseParams{
		LeaseDurationMicroseconds: leaseDuration.Microseconds(),
		ID:                        executionID,
		WorkerID:                  workerID,
	})
	if err != nil {
		return false, fmt.Errorf("renew assistant execution lease: %w", err)
	}
	if rows != 1 {
		return false, execution.ErrLeaseLost
	}
	cancelRequested, err := s.queries.ExecutionCancellationRequested(
		ctx,
		db.ExecutionCancellationRequestedParams{
			ID:       executionID,
			WorkerID: workerID,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return false, execution.ErrLeaseLost
	}
	if err != nil {
		return false, fmt.Errorf("read assistant execution cancellation: %w", err)
	}
	return cancelRequested, nil
}

// FailExpiredExecutions 终止已经失去执行实例的任务。
// 不自动重新排队，避免未来执行包含有副作用的工具调用时重复执行。
func (s *Store) FailExpiredExecutions(ctx context.Context) (int64, error) {
	var rows int64
	err := s.withTransaction(ctx, func(queries *db.Queries) error {
		// 与正常终态写入保持 conversation -> execution -> step 的锁顺序。
		if err := queries.TouchExpiredExecutionConversations(ctx); err != nil {
			return fmt.Errorf("update expired execution conversations: %w", err)
		}
		if err := queries.FailExpiredExecutionSteps(ctx, string(execution.FailureWorkerLost)); err != nil {
			return fmt.Errorf("fail expired assistant execution steps: %w", err)
		}
		var err error
		rows, err = queries.FailExpiredExecutions(ctx, string(execution.FailureWorkerLost))
		if err != nil {
			return fmt.Errorf("fail expired assistant executions: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("fail expired assistant executions transaction: %w", err)
	}
	return rows, nil
}
