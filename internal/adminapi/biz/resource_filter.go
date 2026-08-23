package biz

import "strings"

// ResourceFilter 表达控制台资源列表共享的关键词、启用状态和生效状态筛选
type ResourceFilter struct {
	Query   string
	Enabled *bool
	State   ResourceState
}

// Match 判断资源是否满足共享筛选条件，searchText 由具体资源选择有用户意义的可搜索字段
func (f ResourceFilter) Match(searchText string, enabled bool, status ResourceStatus) bool {
	if f.Enabled != nil && enabled != *f.Enabled {
		return false
	}
	if f.State != "" && status.State != f.State {
		return false
	}
	query := strings.ToLower(strings.TrimSpace(f.Query))
	return query == "" || strings.Contains(strings.ToLower(searchText), query)
}
