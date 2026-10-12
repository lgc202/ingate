package gateway

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

type httpRouteBinding struct {
	route        *gatewayv1alpha1.HTTPRoute
	gatewayName  string
	listenerName string
	ruleIndex    int
	matchIndex   int
	path         gatewayv1alpha1.PathMatch
	backendName  string
}

type httpRouteParentResolution struct {
	routeName      string
	parentRef      gatewayv1alpha1.ParentReference
	accepted       bool
	reason         string
	message        string
	missingBackend string
}

func (r *resolution) selectHTTPRoutes(
	routes []gatewayv1alpha1.HTTPRoute,
	backends []gatewayv1alpha1.Backend,
) {
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
			state := r.attachHTTPRouteParent(route, parent, gatewaysByName[parent.Name], backendsByName, used)
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

func (r *resolution) attachHTTPRouteParent(
	route *gatewayv1alpha1.HTTPRoute,
	parent gatewayv1alpha1.ParentReference,
	gateway *gatewayv1alpha1.Gateway,
	backends map[string]*gatewayv1alpha1.Backend,
	used map[string]bool,
) httpRouteParentResolution {
	state := httpRouteParentResolution{
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
		r.bindHTTPRouteRules(route, gateway.Name, listener.Name, backends, used)
	}

	return state
}

func (r *resolution) bindHTTPRouteRules(
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
			r.bindings = append(r.bindings, httpRouteBinding{
				route:        route,
				gatewayName:  gatewayName,
				listenerName: listenerName,
				ruleIndex:    i,
				matchIndex:   j,
				path:         defaultHTTPRoutePath(match.Path),
				backendName:  backendName,
			})
		}
	}
}

func (r *Reconciler) updateHTTPRouteStatuses(
	ctx context.Context,
	routes []gatewayv1alpha1.HTTPRoute,
	parents []httpRouteParentResolution,
) error {
	byName := make(map[string][]httpRouteParentResolution)
	for _, parent := range parents {
		byName[parent.routeName] = append(byName[parent.routeName], parent)
	}

	for i := range routes {
		route := routes[i].DeepCopy()
		desired := make([]gatewayv1alpha1.RouteParentStatus, 0, len(byName[route.Name]))
		for _, state := range byName[route.Name] {
			parent := gatewayv1alpha1.RouteParentStatus{
				ParentRef:      state.parentRef,
				ControllerName: controllerName,
			}
			for _, previous := range route.Status.Parents {
				if previous.ParentRef == state.parentRef && previous.ControllerName == controllerName {
					parent.Conditions = slices.Clone(previous.Conditions)
					break
				}
			}

			accepted := acceptedCondition(route.Generation)
			accepted.Reason = state.reason
			accepted.Message = state.message
			if !state.accepted {
				accepted.Status = metav1.ConditionFalse
			}
			meta.SetStatusCondition(&parent.Conditions, accepted)

			resolved := metav1.Condition{
				Type:               "ResolvedRefs",
				Status:             metav1.ConditionTrue,
				ObservedGeneration: route.Generation,
				Reason:             "ResolvedRefs",
				Message:            "all Backend references resolve",
			}
			if state.missingBackend != "" {
				resolved.Status = metav1.ConditionFalse
				resolved.Reason = "BackendNotFound"
				resolved.Message = fmt.Sprintf("Backend %q does not exist", state.missingBackend)
			}
			meta.SetStatusCondition(&parent.Conditions, resolved)
			desired = append(desired, parent)
		}

		// 仅替换本控制器拥有的父状态，保留其他控制器的记录。
		for _, parent := range route.Status.Parents {
			if parent.ControllerName != controllerName {
				desired = append(desired, parent)
			}
		}
		if apiequality.Semantic.DeepEqual(route.Status.Parents, desired) {
			continue
		}

		route.Status.Parents = desired
		if err := r.client.Status().Update(ctx, route); err != nil {
			return fmt.Errorf("update HTTPRoute %q status: %w", route.Name, err)
		}
	}

	return nil
}

func defaultHTTPRoutePath(path *gatewayv1alpha1.PathMatch) gatewayv1alpha1.PathMatch {
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
