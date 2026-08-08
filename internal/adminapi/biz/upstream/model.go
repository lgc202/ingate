package upstream

import (
	"net/url"
	"path"
	"strings"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// ValidModel 判断 Upstream 是否满足模型路由的执行约束
func ValidModel(upstream *resource.Upstream) bool {
	if upstream.Spec.Type != resource.UpstreamTypeModel || upstream.Spec.Model == nil {
		return false
	}
	providerProtocol, ok := upstream.Spec.Model.Provider.Protocol()
	if !ok ||
		upstream.Spec.Protocol != providerProtocol ||
		!validAPIBasePath(upstream.Spec.Model.APIBasePath) ||
		len(upstream.Spec.Model.Models) == 0 {
		return false
	}

	enabledModels := 0
	modelNames := make(map[string]struct{}, len(upstream.Spec.Model.Models))
	for _, model := range upstream.Spec.Model.Models {
		if model.Name == "" || strings.TrimSpace(model.Name) != model.Name ||
			model.DisplayName == "" || strings.TrimSpace(model.DisplayName) != model.DisplayName {
			return false
		}
		if _, exists := modelNames[model.Name]; exists {
			return false
		}
		modelNames[model.Name] = struct{}{}
		if model.Enabled {
			enabledModels++
		}
	}
	return enabledModels > 0
}

// ModelEnabled 判断厂商模型是否仍允许被路由引用
func ModelEnabled(modelSpec *resource.ModelSpec, name string) bool {
	for _, model := range modelSpec.Models {
		if model.Name == name {
			return model.Enabled
		}
	}
	return false
}

func validAPIBasePath(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || !strings.HasPrefix(value, "/") {
		return false
	}
	if value != "/" && strings.HasSuffix(value, "/") {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != value {
		return false
	}
	return path.Clean(value) == value
}
