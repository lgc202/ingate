package pluginsource

import (
	"net/url"
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func createSpec(request *adminv1.CreatePluginSourceRequest) (resource.PluginSourceSpec, error) {
	return sourceSpec(request.GetName(), request.GetUrl(), request.GetEnabled())
}

func updateSpec(request *adminv1.UpdatePluginSourceRequest) (resource.PluginSourceSpec, error) {
	return sourceSpec(request.GetName(), request.GetUrl(), request.GetEnabled())
}

func sourceSpec(name, sourceURL string, enabled bool) (resource.PluginSourceSpec, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return resource.PluginSourceSpec{}, adminservice.BadRequest("插件源名称不能为空")
	}
	sourceURL = strings.TrimSpace(sourceURL)
	parsed, err := url.ParseRequestURI(sourceURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return resource.PluginSourceSpec{}, adminservice.BadRequest("目录地址必须是有效的 HTTP 或 HTTPS 地址")
	}
	return resource.PluginSourceSpec{
		DisplayName: name,
		URL:         sourceURL,
		Enabled:     enabled,
	}, nil
}
