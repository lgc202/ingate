package upstream

import (
	"net/url"
	"path"
	"slices"
	"strings"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// ValidModel 判断 Upstream 是否满足模型路由的执行约束
func ValidModel(upstream *resource.Upstream) bool {
	if upstream.Spec.Type != resource.UpstreamTypeModel || upstream.Spec.Model == nil {
		return false
	}
	modelSpec := upstream.Spec.Model
	if _, ok := modelSpec.Provider.Protocol(); !ok || !validBasePath(modelSpec.BasePath) || len(modelSpec.Models) == 0 {
		return false
	}
	if modelSpec.APIKey != "" && upstream.Spec.TLS == nil {
		return false
	}

	seen := make(map[string]struct{}, len(modelSpec.Models))
	for _, model := range modelSpec.Models {
		if model == "" || strings.TrimSpace(model) != model {
			return false
		}
		if _, exists := seen[model]; exists {
			return false
		}
		seen[model] = struct{}{}
	}
	return true
}

// HasModel 判断模型目录是否包含指定厂商模型
func HasModel(modelSpec *resource.ModelSpec, name string) bool {
	return modelSpec != nil && slices.Contains(modelSpec.Models, name)
}

func validBasePath(value string) bool {
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
