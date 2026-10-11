package v1alpha1

import "k8s.io/apimachinery/pkg/runtime"

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

func addDefaultingFuncs(scheme *runtime.Scheme) error {
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
