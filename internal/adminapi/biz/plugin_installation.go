package biz

import (
	"context"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// WasmPluginLister 定义跨策略校验插件安装状态所需的只读能力
type WasmPluginLister interface {
	ListPage(ctx context.Context, page PageRequest) (PageResult[resource.WasmPlugin], error)
}

// PluginInstallationChecker 根据插件包名判断对应制品是否已经安装
// 这里只检查声明式资源是否存在；制品临时下载失败由 Controller 写入状态，不应阻止用户修复已有策略
type PluginInstallationChecker struct {
	plugins WasmPluginLister
}

// NewPluginInstallationChecker 创建插件安装状态检查器
func NewPluginInstallationChecker(plugins WasmPluginLister) *PluginInstallationChecker {
	return &PluginInstallationChecker{plugins: plugins}
}

// Installed 返回指定插件包是否已经安装
func (c *PluginInstallationChecker) Installed(ctx context.Context, packageName string) (bool, error) {
	installed := false
	err := VisitPages(ctx, c.plugins.ListPage, func(plugin resource.WasmPlugin) (bool, error) {
		installed = plugin.Spec.Package == packageName
		return installed, nil
	})
	return installed, err
}
