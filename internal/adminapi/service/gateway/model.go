package gateway

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

// Overview 表示 Gateway 详情页需要的聚合视图
type Overview struct {
	Gateway          *resource.Gateway          `json:"gateway"`
	Routes           []resource.Route           `json:"routes"`
	Upstreams        []resource.Upstream        `json:"upstreams"`
	RuntimeSnapshots []resource.RuntimeSnapshot `json:"runtimeSnapshots"`
}
