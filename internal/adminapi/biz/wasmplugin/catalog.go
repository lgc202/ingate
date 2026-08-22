package wasmplugin

import (
	"context"
	"time"

	"golang.org/x/mod/semver"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// CatalogItem 描述插件市场向用户展示的一项官方插件
// 制品地址和校验信息只供安装流程使用，不通过管理 API 暴露
type CatalogItem struct {
	Package     string
	Name        string
	Version     string
	Category    string
	Description string
	Provider    string
	License     string
	SourceURL   string
}

// CatalogSnapshot 是一次原子读取到的插件目录
// LastCheckedAt 只记录最近一次成功访问远程目录的时间，远程失败时继续使用上次成功结果
type CatalogSnapshot struct {
	Items         []CatalogItem
	LastCheckedAt time.Time
}

// Catalog 定义插件市场读取官方目录和解析安装制品所需的能力
// 目录实现位于 data 层，避免 biz 感知 HTTP、ETag 和本地兜底文件
type Catalog interface {
	Snapshot() CatalogSnapshot
	PluginSpec(packageName string) (resource.WasmPluginSpec, bool)
	Refresh(ctx context.Context) error
}

func newerVersion(current, candidate string) bool {
	current = canonicalVersion(current)
	candidate = canonicalVersion(candidate)
	return semver.IsValid(current) && semver.IsValid(candidate) && semver.Compare(candidate, current) > 0
}

func canonicalVersion(value string) string {
	if value == "" || value[0] == 'v' {
		return value
	}
	return "v" + value
}
