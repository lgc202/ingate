package biz

import (
	"context"
	"errors"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/wasmconfig"
)

// WasmPluginGetter 定义跨策略校验插件安装状态所需的读取能力。
type WasmPluginGetter interface {
	Get(ctx context.Context, pluginID string) (*resource.WasmPlugin, error)
}

// PluginInstallationChecker 根据插件包名判断对应制品是否已经安装。
// 这里只检查声明式资源是否存在；制品临时下载失败由 Controller 写入状态，
// 不应阻止用户修复已有策略。
type PluginInstallationChecker struct {
	plugins WasmPluginGetter
}

// NewPluginInstallationChecker 创建插件安装状态检查器。
func NewPluginInstallationChecker(plugins WasmPluginGetter) *PluginInstallationChecker {
	return &PluginInstallationChecker{plugins: plugins}
}

// Installed 返回指定插件包是否已经安装。
func (c *PluginInstallationChecker) Installed(ctx context.Context, packageName string) (bool, error) {
	_, err := c.plugins.Get(ctx, wasmconfig.PluginID(packageName))
	if errors.Is(err, ErrResourceNotFound) {
		return false, nil
	}
	return err == nil, err
}
