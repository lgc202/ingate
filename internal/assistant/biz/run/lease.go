package run

import (
	"context"
	"time"
)

// keepLease 在模型执行期间续租，并把用户取消请求转换为同一条执行上下文的取消原因。
func (s *Service) keepLease(
	ctx context.Context,
	cancel context.CancelCauseFunc,
	runID string,
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
			cancelRequested, err := s.store.RenewRunLease(ctx, runID, workerID, leaseDuration)
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
