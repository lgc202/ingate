package configurationstatus

import (
	admindto "github.com/lgc202/ingate/internal/admin/dto"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// GetConfigurationStatusResp 是配置状态聚合接口响应
type GetConfigurationStatusResp struct {
	Summary Summary `json:"summary"`
	Items   []Item  `json:"items"`
}

// Summary 是各配置状态的资源数量
type Summary struct {
	Total    int `json:"total"`
	Ready    int `json:"ready"`
	Pending  int `json:"pending"`
	Error    int `json:"error"`
	Disabled int `json:"disabled"`
}

// Item 是单个声明式资源的配置状态
type Item struct {
	Kind   resource.Kind           `json:"kind"`
	ID     string                  `json:"id"`
	Name   string                  `json:"name"`
	Status admindto.ResourceStatus `json:"status"`
}
