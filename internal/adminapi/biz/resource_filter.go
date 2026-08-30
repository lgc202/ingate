package biz

import "strings"

// ResourceFilter 表达控制台资源列表共享的关键词、启用状态和生效状态筛选。
type ResourceFilter struct {
	query   string
	enabled *bool
	state   ResourceState
}

// NewResourceFilter 创建已归一化的资源列表筛选条件。
func NewResourceFilter(query string, enabled *bool, state ResourceState) ResourceFilter {
	return ResourceFilter{
		query:   strings.ToLower(strings.TrimSpace(query)),
		enabled: enabled,
		state:   state,
	}
}

// Match 判断资源是否满足共享筛选条件。
// searchText 由具体资源选择有用户意义的可搜索字段。
func (f ResourceFilter) Match(searchText string, enabled bool, status ResourceStatus) bool {
	if f.enabled != nil && enabled != *f.enabled {
		return false
	}
	if f.state != "" && status.State != f.state {
		return false
	}
	return f.query == "" || strings.Contains(strings.ToLower(searchText), f.query)
}
