package gateway

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// resolution 是本轮配置及父引用状态，不修改输入，也不代表数据面已 ACK。
type resolution struct {
	gateways         []*gatewayv1alpha1.Gateway
	backends         []*gatewayv1alpha1.Backend
	bindings         []routeBinding
	gatewayConflicts []gatewayConflict
	parents          []parentResolution
}

type gatewayConflict struct {
	gateway           *gatewayv1alpha1.Gateway
	listenerName      string
	owner             *gatewayv1alpha1.Gateway
	ownerListenerName string
}

type listenerBinding struct {
	gateway  *gatewayv1alpha1.Gateway
	listener gatewayv1alpha1.Listener
}

type routeBinding struct {
	route        *gatewayv1alpha1.HTTPRoute
	gatewayName  string
	listenerName string
	ruleIndex    int
	matchIndex   int
	path         gatewayv1alpha1.PathMatch
	backendName  string
}

type parentResolution struct {
	routeName      string
	parentRef      gatewayv1alpha1.ParentReference
	accepted       bool
	reason         string
	message        string
	missingBackend string
}

// resolve 按创建时间和名称稳定排列候选；引用失效不移除路径匹配。
func resolve(
	gateways []gatewayv1alpha1.Gateway,
	routes []gatewayv1alpha1.HTTPRoute,
	backends []gatewayv1alpha1.Backend,
) resolution {
	var result resolution
	result.selectGateways(gateways)
	result.selectRoutes(routes, backends)
	return result
}

func (r *resolution) selectGateways(gateways []gatewayv1alpha1.Gateway) {
	ordered := make([]*gatewayv1alpha1.Gateway, 0, len(gateways))
	for i := range gateways {
		ordered = append(ordered, &gateways[i])
	}
	slices.SortFunc(ordered, func(a, b *gatewayv1alpha1.Gateway) int {
		return compareMeta(a.ObjectMeta, b.ObjectMeta)
	})

	var bindings []listenerBinding
	for _, candidate := range ordered {
		if conflict, found := findListenerConflict(candidate, bindings); found {
			r.gatewayConflicts = append(r.gatewayConflicts, conflict)
			continue
		}

		r.gateways = append(r.gateways, candidate)
		for _, listener := range candidate.Spec.Listeners {
			bindings = append(bindings, listenerBinding{gateway: candidate, listener: listener})
		}
	}
}

func (r *resolution) selectRoutes(routes []gatewayv1alpha1.HTTPRoute, backends []gatewayv1alpha1.Backend) {
	gatewaysByName := make(map[string]*gatewayv1alpha1.Gateway, len(r.gateways))
	for _, gateway := range r.gateways {
		gatewaysByName[gateway.Name] = gateway
	}
	backendsByName := make(map[string]*gatewayv1alpha1.Backend, len(backends))
	for i := range backends {
		backendsByName[backends[i].Name] = &backends[i]
	}
	ordered := make([]*gatewayv1alpha1.HTTPRoute, len(routes))
	for i := range routes {
		ordered[i] = &routes[i]
	}
	slices.SortFunc(ordered, func(a, b *gatewayv1alpha1.HTTPRoute) int {
		return compareMeta(a.ObjectMeta, b.ObjectMeta)
	})

	used := make(map[string]bool)
	for _, route := range ordered {
		for _, parent := range route.Spec.ParentRefs {
			state := r.attachParent(route, parent, gatewaysByName[parent.Name], backendsByName, used)
			for _, rule := range route.Spec.Rules {
				for _, ref := range rule.BackendRefs {
					if backendsByName[ref.Name] == nil && state.missingBackend == "" {
						state.missingBackend = ref.Name
					}
				}
			}
			r.parents = append(r.parents, state)
		}
	}

	for _, name := range slices.Sorted(maps.Keys(used)) {
		r.backends = append(r.backends, backendsByName[name])
	}
}

func (r *resolution) attachParent(
	route *gatewayv1alpha1.HTTPRoute,
	parent gatewayv1alpha1.ParentReference,
	gateway *gatewayv1alpha1.Gateway,
	backends map[string]*gatewayv1alpha1.Backend,
	used map[string]bool,
) parentResolution {
	state := parentResolution{
		routeName: route.Name,
		parentRef: parent,
		reason:    "NoMatchingParent",
		message:   "no accepted HTTP listener matches the parent reference",
	}
	if gateway == nil {
		return state
	}

	for _, listener := range gateway.Spec.Listeners {
		if listener.Protocol != gatewayv1alpha1.ProtocolHTTP {
			continue
		}
		if parent.SectionName != "" && parent.SectionName != listener.Name {
			continue
		}

		state.accepted = true
		state.reason = "Accepted"
		state.message = "attached to at least one HTTP listener"
		r.bindRules(route, gateway.Name, listener.Name, backends, used)
	}

	return state
}

func (r *resolution) bindRules(
	route *gatewayv1alpha1.HTTPRoute,
	gatewayName, listenerName string,
	backends map[string]*gatewayv1alpha1.Backend,
	used map[string]bool,
) {
	for i, rule := range route.Spec.Rules {
		// API 只接受单个目标，异常的多引用不能被静默裁剪为第一个。
		backendName := ""
		if len(rule.BackendRefs) == 1 && backends[rule.BackendRefs[0].Name] != nil {
			backendName = rule.BackendRefs[0].Name
			used[backendName] = true
		}

		matches := rule.Matches
		if len(matches) == 0 {
			matches = []gatewayv1alpha1.HTTPRouteMatch{{}}
		}
		for j, match := range matches {
			r.bindings = append(r.bindings, routeBinding{
				route:        route,
				gatewayName:  gatewayName,
				listenerName: listenerName,
				ruleIndex:    i,
				matchIndex:   j,
				path:         defaultPath(match.Path),
				backendName:  backendName,
			})
		}
	}
}

func compareMeta(a, b metav1.ObjectMeta) int {
	if order := a.CreationTimestamp.Compare(b.CreationTimestamp.Time); order != 0 {
		return order
	}

	return cmp.Compare(a.Name, b.Name)
}

func findListenerConflict(candidate *gatewayv1alpha1.Gateway, bindings []listenerBinding) (gatewayConflict, bool) {
	for _, listener := range candidate.Spec.Listeners {
		for _, binding := range bindings {
			if listener.Port != binding.listener.Port {
				continue
			}
			if listener.Hostname != "" && binding.listener.Hostname != "" && listener.Hostname != binding.listener.Hostname {
				continue
			}

			return gatewayConflict{
				gateway:           candidate,
				listenerName:      listener.Name,
				owner:             binding.gateway,
				ownerListenerName: binding.listener.Name,
			}, true
		}
	}

	return gatewayConflict{}, false
}

func defaultPath(path *gatewayv1alpha1.PathMatch) gatewayv1alpha1.PathMatch {
	result := gatewayv1alpha1.PathMatch{Type: gatewayv1alpha1.PathMatchPrefix, Value: "/"}
	if path != nil {
		if path.Type != "" {
			result.Type = path.Type
		}
		if path.Value != "" {
			result.Value = path.Value
		}
	}
	if result.Type == gatewayv1alpha1.PathMatchPrefix && result.Value != "/" {
		result.Value = strings.TrimSuffix(result.Value, "/")
	}
	return result
}
