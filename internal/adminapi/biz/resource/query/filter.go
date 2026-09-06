// Package query 定义声明式资源列表的查询和筛选语义。
package query

import (
	"context"
	"strings"

	"github.com/lgc202/ingate/internal/adminapi/biz/pagination"
	"github.com/lgc202/ingate/internal/adminapi/biz/resource/status"
)

// Filter 表达控制台资源列表共享的关键词、启用状态和生效状态筛选。
type Filter struct {
	query   string
	enabled *bool
	state   status.State
}

// NewFilter 创建已归一化的资源列表筛选条件。
func NewFilter(query string, enabled *bool, state status.State) Filter {
	return Filter{
		query:   strings.ToLower(strings.TrimSpace(query)),
		enabled: enabled,
		state:   state,
	}
}

// Match 判断资源是否满足共享筛选条件。
// searchText 由具体资源选择有用户意义的可搜索字段。
func (f Filter) Match(searchText string, enabled bool, state status.State) bool {
	if f.enabled != nil && enabled != *f.enabled {
		return false
	}
	if f.state != "" && state != f.state {
		return false
	}
	return f.query == "" || strings.Contains(strings.ToLower(searchText), f.query)
}

// FilterPage 在存储分页之上应用控制台筛选，并继续读取后续页直到凑满一页。
// 每次只请求当前页还缺少的数量，因此返回的存储游标始终指向未读取的数据，
// 不需要自定义游标协议。
func FilterPage[T any](
	ctx context.Context,
	page pagination.Request,
	list func(context.Context, pagination.Request) (pagination.Result[T], error),
	match func(T) bool,
) (pagination.Result[T], error) {
	result := pagination.Result[T]{Items: make([]T, 0, page.Limit)}
	next := page.Cursor
	for int64(len(result.Items)) < page.Limit {
		current, err := list(ctx, pagination.Request{
			Limit:  page.Limit - int64(len(result.Items)),
			Cursor: next,
		})
		if err != nil {
			return pagination.Result[T]{}, err
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
