package gateway

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

type httpRouteBinding struct {
	gatewayKey   client.ObjectKey
	listenerName string
	match        dataplane.Route
	backendName  string
}

type httpRouteParentResolution struct {
	routeKey       client.ObjectKey
	parentRef      gatewayv1alpha1.ParentReference
	accepted       bool
	reason         string
	message        string
	missingBackend string
}

func (r *resolution) selectHTTPRoutes(
	routes []*httpRoute,
	backends []*networkBackend,
) {
	gatewaysByKey := make(map[client.ObjectKey]*gatewayv1alpha1.Gateway, len(r.gateways))
	for _, gateway := range r.gateways {
		gatewaysByKey[client.ObjectKeyFromObject(gateway)] = gateway
	}

	backendsByKey := make(map[client.ObjectKey]*networkBackend, len(backends))
	for _, backend := range backends {
		backendsByKey[backend.key] = backend
	}

	ordered := slices.Clone(routes)
	slices.SortFunc(ordered, func(a, b *httpRoute) int {
		return compareMeta(a.ObjectMeta, b.ObjectMeta)
	})

	used := make(map[client.ObjectKey]bool)
	for _, route := range ordered {
		for _, parent := range route.parents {
			key := client.ObjectKey{Namespace: route.Namespace, Name: parent.Name}
			state := r.attachHTTPRouteParent(route, parent, gatewaysByKey[key], backendsByKey, used)
			for _, rule := range route.rules {
				for _, ref := range rule.refs {
					key := client.ObjectKey{Namespace: route.Namespace, Name: ref.Name}
					if backendsByKey[key] == nil && state.missingBackend == "" {
						state.missingBackend = ref.Name
					}
				}
			}

			r.parents = append(r.parents, state)
		}
	}

	keys := slices.SortedFunc(maps.Keys(used), func(a, b client.ObjectKey) int {
		return cmp.Compare(a.String(), b.String())
	})
	for _, key := range keys {
		r.backends = append(r.backends, backendsByKey[key])
	}
}

func (r *resolution) attachHTTPRouteParent(
	route *httpRoute,
	parent gatewayv1alpha1.ParentReference,
	gateway *gatewayv1alpha1.Gateway,
	backends map[client.ObjectKey]*networkBackend,
	used map[client.ObjectKey]bool,
) httpRouteParentResolution {
	state := httpRouteParentResolution{
		routeKey:  client.ObjectKey{Namespace: route.Namespace, Name: route.Name},
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
		r.bindHTTPRouteRules(route, client.ObjectKeyFromObject(gateway), listener.Name, backends, used)
	}

	return state
}

func (r *resolution) bindHTTPRouteRules(
	route *httpRoute,
	gatewayKey client.ObjectKey,
	listenerName string,
	backends map[client.ObjectKey]*networkBackend,
	used map[client.ObjectKey]bool,
) {
	for _, rule := range route.rules {
		// API 只接受单个目标，异常的多引用不能被静默裁剪为第一个。
		backendName := ""
		if len(rule.refs) == 1 {
			key := client.ObjectKey{Namespace: route.Namespace, Name: rule.refs[0].Name}
			if backends[key] != nil {
				backendName = path.Join(key.Namespace, key.Name)
				used[key] = true
			}
		}

		for _, match := range rule.matches {
			r.bindings = append(r.bindings, httpRouteBinding{
				gatewayKey:   gatewayKey,
				listenerName: listenerName,
				match:        match,
				backendName:  backendName,
			})
		}
	}
}

func (r *Reconciler) updateHTTPRouteStatuses(
	ctx context.Context,
	routes []*httpRoute,
	parents []httpRouteParentResolution,
) error {
	byKey := make(map[client.ObjectKey][]httpRouteParentResolution)
	for _, parent := range parents {
		byKey[parent.routeKey] = append(byKey[parent.routeKey], parent)
	}

	for _, compiled := range routes {
		key := client.ObjectKey{Namespace: compiled.Namespace, Name: compiled.Name}
		var route gatewayv1alpha1.HTTPRoute
		if err := r.client.Get(ctx, key, &route); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}

			return fmt.Errorf("get HTTPRoute %q for status: %w", key.String(), err)
		}

		// 缓存可以先看到新声明；旧编译结果不能确认新对象或新一代已生效。
		if route.UID != compiled.UID || route.Generation != compiled.Generation {
			continue
		}

		desired := make([]gatewayv1alpha1.RouteParentStatus, 0, len(byKey[key]))
		for _, state := range byKey[key] {
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
		if err := r.client.Status().Update(ctx, &route); err != nil {
			return fmt.Errorf("update HTTPRoute %q status: %w", key.String(), err)
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
