package configuration

import (
	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	kindPriorityGateway = iota
	kindPriorityRoute
	kindPriorityUpstream
	kindPriorityCertificate
	kindPriorityRateLimitPolicy
	kindPriorityAccessControlPolicy
	kindPriorityTokenQuotaPolicy
	kindPriorityUnknown
)

// Summary 汇总各类声明式资源的当前处理状态
type Summary struct {
	Total    int
	Ready    int
	Pending  int
	Error    int
	Disabled int
}

// Item 表示状态页中的一个声明式资源
type Item struct {
	Kind   resource.Kind
	ID     string
	Name   string
	Status biz.ResourceStatus
}

// Report 保存配置状态汇总和明细
type Report struct {
	Summary Summary
	Items   []Item
}
