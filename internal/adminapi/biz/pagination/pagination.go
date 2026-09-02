// Package pagination 定义 Admin API 业务层的分页遍历语义。
package pagination

import "context"

const internalPageLimit = 200

// Request 是业务层不依赖底层存储协议的分页参数。
type Request struct {
	Limit  int64
	Cursor string
}

// Result 保存一页领域对象和下一页游标。
type Result[T any] struct {
	Items      []T
	NextCursor string
}

// VisitPages 分页遍历跨资源校验所需对象，visit 返回 true 时提前结束。
func VisitPages[T any](
	ctx context.Context,
	list func(context.Context, Request) (Result[T], error),
	visit func(T) (bool, error),
) error {
	page := Request{Limit: internalPageLimit}
	for {
		result, err := list(ctx, page)
		if err != nil {
			return err
		}
		for _, item := range result.Items {
			stop, err := visit(item)
			if err != nil {
				return err
			}
			if stop {
				return nil
			}
		}
		if result.NextCursor == "" {
			return nil
		}
		page.Cursor = result.NextCursor
	}
}
