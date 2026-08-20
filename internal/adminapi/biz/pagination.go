package biz

import "context"

const internalPageLimit = 200

// PageRequest 是 biz 层不依赖底层存储协议的分页参数
type PageRequest struct {
	Limit  int64
	Cursor string
}

// PageResult 保存一页领域对象和下一页游标
type PageResult[T any] struct {
	Items      []T
	NextCursor string
}

// VisitPages 分页遍历跨资源校验所需对象，visit 返回 true 时提前结束
func VisitPages[T any](
	ctx context.Context,
	list func(context.Context, PageRequest) (PageResult[T], error),
	visit func(T) (bool, error),
) error {
	page := PageRequest{Limit: internalPageLimit}
	for {
		result, err := list(ctx, page)
		if err != nil {
			return err
		}
		for _, item := range result.Items {
			stop, err := visit(item)
			if err != nil || stop {
				return err
			}
		}
		if result.NextCursor == "" {
			return nil
		}
		page.Cursor = result.NextCursor
	}
}
