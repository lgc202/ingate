package pluginsource

import (
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/httpurl"
)

func parsePluginSourceSpec(
	displayName string,
	sourceURL string,
	enabled bool,
) (resource.PluginSourceSpec, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return resource.PluginSourceSpec{}, adminv1.ErrorInvalidArgument("插件源名称不能为空")
	}
	sourceURL = strings.TrimSpace(sourceURL)
	if !httpurl.IsValid(sourceURL) {
		return resource.PluginSourceSpec{}, adminv1.ErrorInvalidArgument("目录地址必须是有效的 HTTP 或 HTTPS 地址")
	}
	return resource.PluginSourceSpec{
		DisplayName: displayName,
		URL:         sourceURL,
		Enabled:     enabled,
	}, nil
}
