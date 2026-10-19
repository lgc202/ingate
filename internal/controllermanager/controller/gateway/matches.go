package gateway

import (
	"cmp"
	"fmt"
	"path"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// httpRoute 保存单个对象的匹配结果和未解析引用；Backend 变化不重做路径转换。
// ObjectMeta 只保留排序、身份和状态协调字段，不得写回 API Server。
type httpRoute struct {
	metav1.ObjectMeta
	parents []gatewayv1alpha1.ParentReference
	rules   []httpRule
}

type httpRule struct {
	refs    []gatewayv1alpha1.BackendReference
	matches []dataplane.Route
}

type modelRoute struct {
	metav1.ObjectMeta
	parents []gatewayv1alpha1.ParentReference
	rules   []modelRule
}

type modelRule struct {
	refs    []gatewayv1alpha1.AIBackendReference
	matches []aidataplane.ModelRoute
}

type mcpRoute struct {
	metav1.ObjectMeta
	parents []gatewayv1alpha1.ParentReference
	refs    []gatewayv1alpha1.MCPBackendReference
	match   aidataplane.MCPRoute
}

func compileHTTPRoute(route *gatewayv1alpha1.HTTPRoute) *httpRoute {
	compiled := &httpRoute{ObjectMeta: projectResourceMeta(route), parents: slices.Clone(route.Spec.ParentRefs)}
	for i, rule := range route.Spec.Rules {
		matches := rule.Matches
		if len(matches) == 0 {
			matches = []gatewayv1alpha1.HTTPRouteMatch{{}}
		}

		result := httpRule{refs: slices.Clone(rule.BackendRefs)}
		for j, match := range matches {
			matched := defaultHTTPRoutePath(match.Path)
			result.matches = append(result.matches, dataplane.Route{
				Name:     fmt.Sprintf("%s/rule/%d/match/%d", path.Join(route.Namespace, route.Name), i, j),
				PathType: dataplane.PathMatchType(matched.Type),
				Path:     matched.Value,
			})
		}

		compiled.rules = append(compiled.rules, result)
	}

	return compiled
}

func compileModelRoute(route *gatewayv1alpha1.ModelRoute) *modelRoute {
	compiled := &modelRoute{ObjectMeta: projectForwardingMeta(route), parents: slices.Clone(route.Spec.ParentRefs)}
	for i, rule := range route.Spec.Rules {
		matches := rule.Matches
		if len(matches) == 0 {
			matches = []gatewayv1alpha1.ModelRouteMatch{{}}
		}

		result := modelRule{refs: slices.Clone(rule.BackendRefs)}
		for j, match := range matches {
			headers := make([]aidataplane.HeaderMatch, len(match.Headers))
			for k, header := range match.Headers {
				headers[k] = aidataplane.HeaderMatch{
					Name:  strings.ToLower(header.Name),
					Type:  aidataplane.HeaderMatchType(cmp.Or(header.Type, gatewayv1alpha1.HeaderMatchExact)),
					Value: header.Value,
				}
			}

			result.matches = append(result.matches, aidataplane.ModelRoute{
				Name:    fmt.Sprintf("%s/rule/%d/match/%d", path.Join(route.Namespace, route.Name), i, j),
				Headers: headers,
			})
		}

		compiled.rules = append(compiled.rules, result)
	}

	return compiled
}

func compileMCPRoute(route *gatewayv1alpha1.MCPRoute) *mcpRoute {
	spec := route.Spec.DeepCopy()
	compiled := &mcpRoute{
		ObjectMeta: projectForwardingMeta(route),
		parents:    spec.ParentRefs,
		refs:       spec.BackendRefs,
		match: aidataplane.MCPRoute{
			Name: path.Join(route.Namespace, route.Name),
			Path: ptr.Deref(spec.Path, "/mcp"),
		},
	}
	if len(spec.BackendRefs) == 1 {
		compiled.match.BackendPath = ptr.Deref(spec.BackendRefs[0].Path, "/mcp")
	}

	return compiled
}

// ResourceName 标识单个 HTTPRoute 的编译结果。
func (r *httpRoute) ResourceName() string {
	return path.Join(r.Namespace, r.Name)
}

// ResourceName 标识单个 ModelRoute 的编译结果。
func (r *modelRoute) ResourceName() string {
	return path.Join(r.Namespace, r.Name)
}

// ResourceName 标识单个 MCPRoute 的编译结果。
func (r *mcpRoute) ResourceName() string {
	return path.Join(r.Namespace, r.Name)
}
