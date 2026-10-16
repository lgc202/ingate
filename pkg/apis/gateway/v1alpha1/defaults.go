package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

// SetDefaultsAIBackend 应用 OpenAI 上游路径前缀，不推断 API 格式或网络目标。
func SetDefaultsAIBackend(backend *AIBackend) {
	if backend.Spec.Schema.Name == APISchemaOpenAI && backend.Spec.Schema.Prefix == "" {
		backend.Spec.Schema.Prefix = "/v1"
	}
}

// SetDefaultsHTTPRoute 应用 Gateway API 的缺省路径；不改变父引用、规则顺序或目标。
func SetDefaultsHTTPRoute(route *HTTPRoute) {
	for i := range route.Spec.Rules {
		rule := &route.Spec.Rules[i]
		if len(rule.Matches) == 0 {
			rule.Matches = []HTTPRouteMatch{{}}
		}

		for j := range rule.Matches {
			match := &rule.Matches[j]
			if match.Path == nil {
				match.Path = &PathMatch{}
			}

			if match.Path.Type == "" {
				match.Path.Type = PathMatchPrefix
			}

			if match.Path.Value == "" {
				match.Path.Value = "/"
			}
		}
	}
}

// SetDefaultsModelRoute 应用已声明 Header 条件的缺省匹配方式，保留空匹配及规则顺序。
func SetDefaultsModelRoute(route *ModelRoute) {
	for i := range route.Spec.Rules {
		rule := &route.Spec.Rules[i]
		for j := range rule.Matches {
			match := &rule.Matches[j]
			for k := range match.Headers {
				header := &match.Headers[k]
				if header.Type == "" {
					header.Type = HeaderMatchExact
				}
			}
		}
	}
}

// SetDefaultsMCPRoute 仅为省略的入口和上游路径设置 /mcp，不覆盖显式空值或路径。
func SetDefaultsMCPRoute(route *MCPRoute) {
	if route.Spec.Path == nil {
		route.Spec.Path = ptr.To("/mcp")
	}

	for i := range route.Spec.BackendRefs {
		if route.Spec.BackendRefs[i].Path == nil {
			route.Spec.BackendRefs[i].Path = ptr.To("/mcp")
		}
	}
}

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	scheme.AddTypeDefaultingFunc(&MCPRoute{}, func(obj any) {
		SetDefaultsMCPRoute(obj.(*MCPRoute))
	})
	scheme.AddTypeDefaultingFunc(&MCPRouteList{}, func(obj any) {
		list := obj.(*MCPRouteList)
		for i := range list.Items {
			SetDefaultsMCPRoute(&list.Items[i])
		}
	})
	scheme.AddTypeDefaultingFunc(&ModelRoute{}, func(obj any) {
		SetDefaultsModelRoute(obj.(*ModelRoute))
	})
	scheme.AddTypeDefaultingFunc(&ModelRouteList{}, func(obj any) {
		list := obj.(*ModelRouteList)
		for i := range list.Items {
			SetDefaultsModelRoute(&list.Items[i])
		}
	})
	scheme.AddTypeDefaultingFunc(&AIBackend{}, func(obj any) {
		SetDefaultsAIBackend(obj.(*AIBackend))
	})
	scheme.AddTypeDefaultingFunc(&AIBackendList{}, func(obj any) {
		list := obj.(*AIBackendList)
		for i := range list.Items {
			SetDefaultsAIBackend(&list.Items[i])
		}
	})
	scheme.AddTypeDefaultingFunc(&HTTPRoute{}, func(obj any) {
		SetDefaultsHTTPRoute(obj.(*HTTPRoute))
	})
	scheme.AddTypeDefaultingFunc(&HTTPRouteList{}, func(obj any) {
		list := obj.(*HTTPRouteList)
		for i := range list.Items {
			SetDefaultsHTTPRoute(&list.Items[i])
		}
	})

	return nil
}
