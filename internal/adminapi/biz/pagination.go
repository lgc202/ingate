package biz

import "context"

const internalPageSize = 200

// PageRequest 是用例层不依赖底层存储协议的分页游标
type PageRequest struct {
	Size  int64
	Token string
}

// PageResult 保存一页领域对象和下一页游标
type PageResult[T any] struct {
	Items     []T
	NextToken string
}

// VisitPages 分页遍历跨资源校验所需对象，visit 返回 true 时提前结束
func VisitPages[T any](
	ctx context.Context,
	list func(context.Context, PageRequest) (PageResult[T], error),
	visit func(T) (bool, error),
) error {
	page := PageRequest{Size: internalPageSize}
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
		if result.NextToken == "" {
			return nil
		}
		page.Token = result.NextToken
	}
}
