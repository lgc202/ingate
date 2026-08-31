package biz

import "context"

const internalPageLimit = 200

// PageRequest 是 biz 层不依赖底层存储协议的分页参数。
type PageRequest struct {
	Limit  int64
	Cursor string
}

// PageResult 保存一页领域对象和下一页游标。
type PageResult[T any] struct {
	Items      []T
	NextCursor string
}

// VisitPages 分页遍历跨资源校验所需对象，visit 返回 true 时提前结束。
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

// FilterPage 在存储分页之上应用控制台筛选，并继续读取后续页直到凑满一页。
// 每次只请求当前页还缺少的数量，因此返回的存储游标始终指向未读取的数据，
// 不需要自定义游标协议。
func FilterPage[T any](
	ctx context.Context,
	page PageRequest,
	list func(context.Context, PageRequest) (PageResult[T], error),
	match func(T) bool,
) (PageResult[T], error) {
	result := PageResult[T]{Items: make([]T, 0, page.Limit)}
	next := page.Cursor
	for int64(len(result.Items)) < page.Limit {
		current, err := list(ctx, PageRequest{
			Limit:  page.Limit - int64(len(result.Items)),
			Cursor: next,
		})
		if err != nil {
			return PageResult[T]{}, err
		}
		for _, item := range current.Items {
			if match(item) {
				result.Items = append(result.Items, item)
			}
		}
		next = current.NextCursor
		if next == "" {
			break
		}
	}
	result.NextCursor = next
	return result, nil
}
