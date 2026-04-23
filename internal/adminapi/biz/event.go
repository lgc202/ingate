package biz

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	"github.com/lgc202/ingate/internal/adminapi/store"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
)

const eventTimeLayout = "2006-01-02 15:04"

type EventService struct {
	store *store.APIServerStore
}

type synthesizedEvent struct {
	id       string
	level    string
	message  string
	resource string
	at       time.Time
}

func NewEventService(store *store.APIServerStore) *EventService {
	return &EventService{store: store}
}

func (s *EventService) ListEvents(ctx context.Context) (dto.EventListResponse, error) {
	gateways, err := s.store.ListGateways(ctx)
	if err != nil {
		return dto.EventListResponse{}, err
	}
	routes, err := s.store.ListRoutes(ctx)
	if err != nil {
		return dto.EventListResponse{}, err
	}
	backends, err := s.store.ListBackends(ctx)
	if err != nil {
		return dto.EventListResponse{}, err
	}
	authPolicies, err := s.store.ListAuthPolicies(ctx)
	if err != nil {
		return dto.EventListResponse{}, err
	}
	trafficPolicies, err := s.store.ListTrafficPolicies(ctx)
	if err != nil {
		return dto.EventListResponse{}, err
	}

	items := buildSynthesizedEvents(gateways.Items, routes.Items, backends.Items, authPolicies.Items, trafficPolicies.Items)
	resp := dto.EventListResponse{Items: make([]dto.Event, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, dto.Event{
			ID:       item.id,
			Level:    item.level,
			Message:  item.message,
			Resource: item.resource,
			Time:     formatEventTime(item.at),
		})
	}
	return resp, nil
}

func buildSynthesizedEvents(
	gateways []gatewayv1alpha1.Gateway,
	routes []gatewayv1alpha1.Route,
	backends []gatewayv1alpha1.Backend,
	authPolicies []policyv1alpha1.AuthPolicy,
	trafficPolicies []policyv1alpha1.TrafficPolicy,
) []synthesizedEvent {
	gatewayNames := gatewayNameSet(gateways)
	backendNames := backendNameSet(backends)
	routeNames := routeNameSet(routes)
	events := make([]synthesizedEvent, 0, len(gateways)+len(routes)+len(backends)+len(authPolicies)+len(trafficPolicies))

	for i := range gateways {
		if item, ok := conditionEvent(resourceKindGateway, gateways[i].Name, gateways[i].CreationTimestamp.Time, gateways[i].Status.Conditions); ok {
			events = append(events, item)
		}
	}
	for i := range routes {
		route := routes[i]
		if item, ok := routeConditionEvent(route); ok {
			events = append(events, item)
		}
		events = append(events, unresolvedRouteEvents(route, gatewayNames, backendNames)...)
	}
	for i := range backends {
		if item, ok := conditionEvent(resourceKindBackend, backends[i].Name, backends[i].CreationTimestamp.Time, backends[i].Status.Conditions); ok {
			events = append(events, item)
		}
	}
	for i := range authPolicies {
		policy := authPolicies[i]
		if item, ok := conditionEvent(resourceKindAuthPolicy, policy.Name, policy.CreationTimestamp.Time, policy.Status.Conditions); ok {
			events = append(events, item)
		}
		events = append(events, unresolvedPolicyTargetEvents(resourceKindAuthPolicy, policy.Name, policy.CreationTimestamp.Time, policy.Spec.TargetRefs, gatewayNames, routeNames, backendNames)...)
	}
	for i := range trafficPolicies {
		policy := trafficPolicies[i]
		if item, ok := conditionEvent(resourceKindTrafficPolicy, policy.Name, policy.CreationTimestamp.Time, policy.Status.Conditions); ok {
			events = append(events, item)
		}
		events = append(events, unresolvedPolicyTargetEvents(resourceKindTrafficPolicy, policy.Name, policy.CreationTimestamp.Time, policy.Spec.TargetRefs, gatewayNames, routeNames, backendNames)...)
	}

	sort.SliceStable(events, func(i, j int) bool {
		return events[i].at.After(events[j].at)
	})
	if len(events) > 200 {
		return events[:200]
	}
	return events
}

func routeConditionEvent(route gatewayv1alpha1.Route) (synthesizedEvent, bool) {
	conditions := make([]metav1.Condition, 0, len(route.Status.Conditions))
	conditions = append(conditions, route.Status.Conditions...)
	for _, parent := range route.Status.Parents {
		conditions = append(conditions, parent.Conditions...)
	}
	return conditionEvent(resourceKindRoute, route.Name, route.CreationTimestamp.Time, conditions)
}

func conditionEvent(kind, name string, fallback time.Time, conditions []metav1.Condition) (synthesizedEvent, bool) {
	condition, status, ok := primaryCondition(conditions)
	if !ok {
		return synthesizedEvent{}, false
	}

	message := conditionEventMessage(kind, name, status, condition.Reason, condition.Message)

	eventTime := condition.LastTransitionTime.Time
	if eventTime.IsZero() {
		eventTime = fallback
	}
	if eventTime.IsZero() {
		eventTime = time.Now()
	}

	return synthesizedEvent{
		id:       fmt.Sprintf("%s-%s-%s", kind, name, status),
		level:    eventLevel(status),
		message:  message,
		resource: fmt.Sprintf("%s/%s", kind, name),
		at:       eventTime,
	}, true
}

func unresolvedRouteEvents(route gatewayv1alpha1.Route, gatewayNames, backendNames map[string]struct{}) []synthesizedEvent {
	items := make([]synthesizedEvent, 0)
	at := route.CreationTimestamp.Time
	if at.IsZero() {
		at = time.Now()
	}

	for _, parent := range route.Spec.ParentRefs {
		if parent.Name == "" {
			continue
		}
		if _, exists := gatewayNames[parent.Name]; exists {
			continue
		}
		items = append(items, synthesizedEvent{
			id:       fmt.Sprintf("route-%s-missing-gateway-%s", route.Name, parent.Name),
			level:    "Error",
			message:  fmt.Sprintf("绑定的网关入口 %s 不存在。", parent.Name),
			resource: fmt.Sprintf("%s/%s", resourceKindRoute, route.Name),
			at:       at,
		})
	}
	for _, rule := range route.Spec.Rules {
		for _, backendRef := range rule.BackendRefs {
			if backendRef.Name == "" {
				continue
			}
			if _, exists := backendNames[backendRef.Name]; exists {
				continue
			}
			items = append(items, synthesizedEvent{
				id:       fmt.Sprintf("route-%s-missing-backend-%s", route.Name, backendRef.Name),
				level:    "Error",
				message:  fmt.Sprintf("转发的后端服务 %s 不存在。", backendRef.Name),
				resource: fmt.Sprintf("%s/%s", resourceKindRoute, route.Name),
				at:       at,
			})
		}
	}
	return items
}

func unresolvedPolicyTargetEvents(
	kind, name string,
	fallback time.Time,
	refs []policyv1alpha1.TargetReference,
	gatewayNames, routeNames, backendNames map[string]struct{},
) []synthesizedEvent {
	items := make([]synthesizedEvent, 0)
	at := fallback
	if at.IsZero() {
		at = time.Now()
	}

	for _, ref := range refs {
		if ref.Name == "" {
			continue
		}
		if policyTargetExists(ref, gatewayNames, routeNames, backendNames) {
			continue
		}
		items = append(items, synthesizedEvent{
			id:       fmt.Sprintf("%s-%s-missing-target-%s-%s", kind, name, ref.Kind, ref.Name),
			level:    "Error",
			message:  fmt.Sprintf("绑定目标 %s 不存在。", targetDisplayName(ref.Kind, ref.Name)),
			resource: fmt.Sprintf("%s/%s", kind, name),
			at:       at,
		})
	}
	return items
}

func policyTargetExists(
	ref policyv1alpha1.TargetReference,
	gatewayNames, routeNames, backendNames map[string]struct{},
) bool {
	switch ref.Kind {
	case resourceKindGateway:
		_, exists := gatewayNames[ref.Name]
		return exists
	case resourceKindRoute:
		_, exists := routeNames[ref.Name]
		return exists
	case resourceKindBackend:
		_, exists := backendNames[ref.Name]
		return exists
	default:
		return true
	}
}

func primaryCondition(conditions []metav1.Condition) (metav1.Condition, string, bool) {
	for _, condition := range conditions {
		if condition.Status == metav1.ConditionFalse {
			return condition, resourceStatusError, true
		}
	}
	for _, condition := range conditions {
		if condition.Status == metav1.ConditionUnknown {
			return condition, resourceStatusWarning, true
		}
	}
	for _, condition := range conditions {
		if condition.Status == metav1.ConditionTrue {
			return condition, resourceStatusHealthy, true
		}
	}
	return metav1.Condition{}, "", false
}

func fallbackConditionMessage(kind, name, status, reason string) string {
	_ = kind
	_ = name
	switch status {
	case resourceStatusHealthy:
		return "已生效。"
	case resourceStatusWarning:
		if label := conditionReasonLabel(reason); label != "" {
			return fmt.Sprintf("仍在处理中：%s。", label)
		}
		return "仍在处理中。"
	case resourceStatusError:
		if label := conditionReasonLabel(reason); label != "" {
			return fmt.Sprintf("生效失败：%s。", label)
		}
		return "生效失败。"
	default:
		return "状态待确认。"
	}
}

func conditionEventMessage(kind, name, status, reason, raw string) string {
	if message := scopedConfigConditionMessage(kind, name, status, reason); message != "" {
		return message
	}
	if message := reasonBasedConditionMessage(reason, status); message != "" {
		return message
	}
	if looksUserFacingMessage(raw) {
		return raw
	}
	return fallbackConditionMessage(kind, name, status, reason)
}

func scopedConfigConditionMessage(kind, name, status, reason string) string {
	switch {
	case kind == resourceKindAuthPolicy && isRouteScopedPolicyName(name, authPolicySuffix):
		return directConfigMessage("认证配置", status, reason)
	case kind == resourceKindTrafficPolicy && isRouteScopedPolicyName(name, trafficPolicySuffix):
		return directConfigMessage("流量保护", status, reason)
	default:
		return ""
	}
}

func directConfigMessage(label, status, reason string) string {
	switch status {
	case resourceStatusHealthy:
		return fmt.Sprintf("当前路由的%s已生效。", label)
	case resourceStatusWarning:
		if reasonLabel := conditionReasonLabel(reason); reasonLabel != "" {
			return fmt.Sprintf("当前路由的%s仍在处理中：%s。", label, reasonLabel)
		}
		return fmt.Sprintf("当前路由的%s仍在处理中。", label)
	case resourceStatusError:
		if reasonLabel := conditionReasonLabel(reason); reasonLabel != "" {
			return fmt.Sprintf("当前路由的%s生效失败：%s。", label, reasonLabel)
		}
		return fmt.Sprintf("当前路由的%s生效失败。", label)
	default:
		return fmt.Sprintf("当前路由的%s状态待确认。", label)
	}
}

func reasonBasedConditionMessage(reason, status string) string {
	switch reason {
	case "Accepted", "Resolved", "ResolvedRefs", "Ready", "Programmed", "Reconciled":
		if status == resourceStatusHealthy {
			return "已生效。"
		}
	case "MissingSecret":
		return "关联的 TLS Secret 不存在。"
	case "Invalid":
		return "配置不合法。"
	case "NoHealthyEndpoints":
		return "没有可用实例。"
	case "Detached":
		return "尚未绑定到目标。"
	case "Conflict":
		return "配置存在冲突。"
	case "RefNotPermitted":
		return "当前引用不被允许。"
	}
	return ""
}

func conditionReasonLabel(reason string) string {
	switch reason {
	case "Accepted", "Resolved", "ResolvedRefs", "Ready", "Programmed", "Reconciled":
		return "控制面已接受配置"
	case "MissingSecret":
		return "关联的 TLS Secret 不存在"
	case "Invalid":
		return "配置不合法"
	case "NoHealthyEndpoints":
		return "没有可用实例"
	case "Detached":
		return "尚未绑定到目标"
	case "Conflict":
		return "配置存在冲突"
	case "RefNotPermitted":
		return "当前引用不被允许"
	default:
		return ""
	}
}

func looksUserFacingMessage(message string) bool {
	if strings.TrimSpace(message) == "" {
		return false
	}
	for _, r := range message {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func targetDisplayName(kind, name string) string {
	if name == "" {
		return resourceDisplayName(kind)
	}
	return fmt.Sprintf("%s %s", resourceDisplayName(kind), name)
}

const (
	routeScopedPrefix   = "routecfg-"
	authPolicySuffix    = "-auth"
	trafficPolicySuffix = "-traffic"
)

func isRouteScopedPolicyName(name, suffix string) bool {
	return strings.HasPrefix(name, routeScopedPrefix) && strings.HasSuffix(name, suffix)
}

func resourceDisplayName(kind string) string {
	switch kind {
	case resourceKindGateway:
		return "网关入口"
	case resourceKindRoute:
		return "路由规则"
	case resourceKindBackend:
		return "后端服务"
	case resourceKindAuthPolicy:
		return "认证模板"
	case resourceKindTrafficPolicy:
		return "流量保护模板"
	default:
		return kind
	}
}

func eventLevel(status string) string {
	switch status {
	case resourceStatusError:
		return "Error"
	case resourceStatusWarning, resourceStatusPending:
		return "Warning"
	default:
		return "Info"
	}
}

func formatEventTime(at time.Time) string {
	if at.IsZero() {
		return "-"
	}
	return at.Local().Format(eventTimeLayout)
}
