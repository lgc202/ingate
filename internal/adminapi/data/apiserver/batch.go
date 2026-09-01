package apiserver

import (
	"context"
	"errors"

	"golang.org/x/sync/errgroup"

	"github.com/lgc202/ingate/internal/adminapi/biz"
)

// 单个管理请求最多占用八条并发读取，避免大 Route 耗尽 API Server 连接。
const maxConcurrentResourceReads = 8

// listByIDs 并发读取去重后的资源 ID。
// 不存在的资源不进入结果，其余错误会取消未完成的读取。
func listByIDs[T any](
	ctx context.Context,
	resourceIDs []string,
	get func(context.Context, string) (T, error),
) (map[string]T, error) {
	uniqueIDs := make([]string, 0, len(resourceIDs))
	seenIDs := make(map[string]bool, len(resourceIDs))
	for _, resourceID := range resourceIDs {
		if seenIDs[resourceID] {
			continue
		}
		seenIDs[resourceID] = true
		uniqueIDs = append(uniqueIDs, resourceID)
	}

	resources := make([]T, len(uniqueIDs))
	found := make([]bool, len(uniqueIDs))
	group, lookupCtx := errgroup.WithContext(ctx)
	group.SetLimit(maxConcurrentResourceReads)
	for i, resourceID := range uniqueIDs {
		group.Go(func() error {
			resource, err := get(lookupCtx, resourceID)
			if errors.Is(err, biz.ErrResourceNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			resources[i] = resource
			found[i] = true
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	result := make(map[string]T, len(resources))
	for i, resource := range resources {
		if found[i] {
			result[uniqueIDs[i]] = resource
		}
	}
	return result, nil
}
