package gateway

import (
	"cmp"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// resolution 是一轮协调的接纳结果，不修改输入资源，也不代表配置已经发布。
type resolution struct {
	gateways         []*gatewayv1alpha1.Gateway
	routes           []*gatewayv1alpha1.Route
	upstreams        []*gatewayv1alpha1.Upstream
	gatewayConflicts []gatewayConflict
	routeRejections  []routeRejection
}

type gatewayConflict struct {
	gateway           *gatewayv1alpha1.Gateway
	listenerName      string
	owner             *gatewayv1alpha1.Gateway
	ownerListenerName string
}

type routeRejection struct {
	route       *gatewayv1alpha1.Route
	reason      routeRejectionReason
	relatedName string
}

type routeRejectionReason string

const (
	gatewayMissing  routeRejectionReason = "GatewayMissing"
	gatewayRejected routeRejectionReason = "GatewayRejected"
	listenerMissing routeRejectionReason = "ListenerMissing"
	upstreamMissing routeRejectionReason = "UpstreamMissing"
	matchConflict   routeRejectionReason = "MatchConflict"
)

type listenerBinding struct {
	gateway  *gatewayv1alpha1.Gateway
	listener gatewayv1alpha1.Listener
}

type routeMatch struct {
	gatewayName  string
	listenerName string
	pathType     gatewayv1alpha1.PathMatchType
	pathValue    string
}

// routeReferences 只索引本轮资源；引用检查不占用匹配规则，也不修改输入。
type routeReferences struct {
	allGateways      map[string]bool
	acceptedGateways map[string]*gatewayv1alpha1.Gateway
	upstreams        map[string]*gatewayv1alpha1.Upstream
}

// resolve 根据同一轮列出的资源计算接纳结果；调用方负责 List 和发布失败的重试。
func resolve(
	gateways []gatewayv1alpha1.Gateway,
	routes []gatewayv1alpha1.Route,
	upstreams []gatewayv1alpha1.Upstream,
) resolution {
	var result resolution
	result.selectGateways(gateways)
	result.selectRoutes(gateways, routes, upstreams)
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

func (r *resolution) selectRoutes(
	gateways []gatewayv1alpha1.Gateway,
	routes []gatewayv1alpha1.Route,
	upstreams []gatewayv1alpha1.Upstream,
) {
	references := newRouteReferences(gateways, r.gateways, upstreams)
	ordered := make([]*gatewayv1alpha1.Route, 0, len(routes))
	for i := range routes {
		ordered = append(ordered, &routes[i])
	}
	slices.SortFunc(ordered, func(a, b *gatewayv1alpha1.Route) int {
		return compareMeta(a.ObjectMeta, b.ObjectMeta)
	})

	usedUpstreams := make(map[string]bool)
	matches := make(map[routeMatch]*gatewayv1alpha1.Route)
	for _, route := range ordered {
		upstream, rejection := references.resolve(route)
		if rejection != nil {
			r.routeRejections = append(r.routeRejections, *rejection)
			continue
		}

		match := routeMatch{
			gatewayName:  route.Spec.GatewayName,
			listenerName: route.Spec.ListenerName,
			pathType:     route.Spec.Path.Type,
			pathValue:    canonicalPath(route.Spec.Path),
		}
		if owner := matches[match]; owner != nil {
			r.routeRejections = append(r.routeRejections, routeRejection{
				route: route, reason: matchConflict, relatedName: owner.Name,
			})
			continue
		}

		r.routes = append(r.routes, route)
		matches[match] = route
		if !usedUpstreams[upstream.Name] {
			r.upstreams = append(r.upstreams, upstream)
			usedUpstreams[upstream.Name] = true
		}
	}
}

func newRouteReferences(
	gateways []gatewayv1alpha1.Gateway,
	acceptedGateways []*gatewayv1alpha1.Gateway,
	upstreams []gatewayv1alpha1.Upstream,
) routeReferences {
	references := routeReferences{
		allGateways:      make(map[string]bool, len(gateways)),
		acceptedGateways: make(map[string]*gatewayv1alpha1.Gateway, len(acceptedGateways)),
		upstreams:        make(map[string]*gatewayv1alpha1.Upstream, len(upstreams)),
	}
	for i := range gateways {
		references.allGateways[gateways[i].Name] = true
	}
	for _, gateway := range acceptedGateways {
		references.acceptedGateways[gateway.Name] = gateway
	}
	for i := range upstreams {
		references.upstreams[upstreams[i].Name] = &upstreams[i]
	}

	return references
}

// resolve 按 Gateway、Listener、Upstream 的顺序检查引用；成功时 rejection 为 nil。
func (r routeReferences) resolve(route *gatewayv1alpha1.Route) (*gatewayv1alpha1.Upstream, *routeRejection) {
	gateway, accepted := r.acceptedGateways[route.Spec.GatewayName]
	if !accepted {
		reason := gatewayMissing
		if r.allGateways[route.Spec.GatewayName] {
			reason = gatewayRejected
		}
		return nil, &routeRejection{route: route, reason: reason, relatedName: route.Spec.GatewayName}
	}
	if !hasListener(gateway, route.Spec.ListenerName) {
		return nil, &routeRejection{route: route, reason: listenerMissing, relatedName: route.Spec.ListenerName}
	}

	upstream := r.upstreams[route.Spec.UpstreamName]
	if upstream == nil {
		return nil, &routeRejection{route: route, reason: upstreamMissing, relatedName: route.Spec.UpstreamName}
	}

	return upstream, nil
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

func hasListener(gateway *gatewayv1alpha1.Gateway, name string) bool {
	for _, listener := range gateway.Spec.Listeners {
		if listener.Name == name {
			return true
		}
	}

	return false
}

// canonicalPath 使尾斜杠不同的 PathPrefix 表示同一条路径段匹配规则。
func canonicalPath(path gatewayv1alpha1.PathMatch) string {
	if path.Type != gatewayv1alpha1.PathMatchPrefix || path.Value == "/" {
		return path.Value
	}

	return strings.TrimSuffix(path.Value, "/")
}

func compareMeta(a, b metav1.ObjectMeta) int {
	if order := a.CreationTimestamp.Compare(b.CreationTimestamp.Time); order != 0 {
		return order
	}

	return cmp.Compare(a.Name, b.Name)
}
