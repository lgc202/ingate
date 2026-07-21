package configurationstatus

import (
	"github.com/lgc202/ingate/internal/adminapi/service/resourcestatus"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Summary 汇总各配置状态的资源数量
type Summary struct {
	Total    int
	Ready    int
	Pending  int
	Error    int
	Disabled int
}

// Item 表示一个声明式资源的控制台配置状态
type Item struct {
	Kind   resource.Kind
	ID     string
	Name   string
	Status resourcestatus.Status
}

// Report 表示配置状态聚合用例结果
type Report struct {
	Summary Summary
	Items   []Item
}
