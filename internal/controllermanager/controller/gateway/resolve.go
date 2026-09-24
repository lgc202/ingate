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
	allGateways := make(map[string]bool, len(gateways))
	for i := range gateways {
		allGateways[gateways[i].Name] = true
	}

	acceptedGateways := make(map[string]*gatewayv1alpha1.Gateway, len(r.gateways))
	for _, gateway := range r.gateways {
		acceptedGateways[gateway.Name] = gateway
	}

	upstreamsByName := make(map[string]*gatewayv1alpha1.Upstream, len(upstreams))
	for i := range upstreams {
		upstreamsByName[upstreams[i].Name] = &upstreams[i]
	}

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
		gateway, accepted := acceptedGateways[route.Spec.GatewayName]
		if !accepted {
			reason := gatewayMissing
			if allGateways[route.Spec.GatewayName] {
				reason = gatewayRejected
			}
			r.routeRejections = append(r.routeRejections, routeRejection{
				route: route, reason: reason, relatedName: route.Spec.GatewayName,
			})
			continue
		}

		if !hasListener(gateway, route.Spec.ListenerName) {
			r.routeRejections = append(r.routeRejections, routeRejection{
				route: route, reason: listenerMissing, relatedName: route.Spec.ListenerName,
			})
			continue
		}

		upstream := upstreamsByName[route.Spec.UpstreamName]
		if upstream == nil {
			r.routeRejections = append(r.routeRejections, routeRejection{
				route: route, reason: upstreamMissing, relatedName: route.Spec.UpstreamName,
			})
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
