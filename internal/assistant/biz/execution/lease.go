package execution

import (
	"context"
	"time"
)

// executionLease 管理一条已领取任务的执行上下文和续租协程。
// 它不是持久化租约本身；MySQL 中的 worker_id 与 lease_expires_at 才是归属事实。
type executionLease struct {
	ctx     context.Context
	cancel  context.CancelCauseFunc
	stopped chan error
}

func (e *Executor) startExecutionLease(
	ctx context.Context,
	claim Claim,
	workerID string,
	leaseDuration time.Duration,
) *executionLease {
	executionCtx, cancel := context.WithCancelCause(ctx)
	lease := &executionLease{
		ctx:     executionCtx,
		cancel:  cancel,
		stopped: make(chan error, 1),
	}
	go func() {
		lease.stopped <- e.watchLease(
			executionCtx,
			cancel,
			claim.ID,
			workerID,
			leaseDuration,
		)
	}()
	return lease
}

// stop 通知续租协程退出，并返回最终取消原因与续租错误。
// cancel cause 只接受第一次写入，因此租约丢失和用户取消不会被正常结束覆盖。
func (l *executionLease) stop() (error, error) {
	l.cancel(errExecutionFinished)
	leaseErr := <-l.stopped
	return context.Cause(l.ctx), leaseErr
}

// watchLease 在模型执行期间续租，并把用户取消请求转换为同一条执行上下文的取消原因。
func (e *Executor) watchLease(
	ctx context.Context,
	cancel context.CancelCauseFunc,
	executionID string,
	workerID string,
	leaseDuration time.Duration,
) error {
	// 一秒上限让同一轮询同时承担续租和取消检测，避免再引入一条取消消息通道。
	interval := min(leaseDuration/3, time.Second)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			cancelRequested, err := e.store.RenewExecutionLease(
				ctx,
				executionID,
				workerID,
				leaseDuration,
			)
			if err != nil {
				cancel(err)
				return err
			}
			if cancelRequested {
				cancel(ErrCancellation)
				return ErrCancellation
			}
		}
	}
}
