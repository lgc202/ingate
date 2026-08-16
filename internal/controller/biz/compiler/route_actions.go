package compiler

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	minRetryAttempts       = 1
	maxRetryAttempts       = 5
	minPerTryTimeoutMillis = 100
	maxPerTryTimeoutMillis = 60000
	defaultRetryOn         = "connect-failure,refused-stream,reset,5xx"
)

func (c *compilation) weightedClusters(
	route *gatewayv1.Route,
	compiledUpstreams map[string]bool,
) ([]*routev3.WeightedCluster_ClusterWeight, bool) {
	if len(route.Spec.UpstreamRefs) == 0 {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonInvalidSpec, fmt.Sprintf("route %q must reference at least one upstream", route.Name))
		return nil, false
	}
	refs := slices.Clone(route.Spec.UpstreamRefs)
	slices.SortFunc(refs, func(a, b gatewayv1.UpstreamRef) int { return cmp.Compare(a.Name, b.Name) })
	clusters := make([]*routev3.WeightedCluster_ClusterWeight, 0, len(refs))
	seen := make(map[string]bool, len(refs))
	valid := true
	for _, ref := range refs {
		exists := compiledUpstreams[ref.Name]
		if ref.Name == "" || seen[ref.Name] || !exists || ref.Weight < 1 || ref.Weight > 1000 {
			reason := ReasonInvalidSpec
			if ref.Name != "" && !exists {
				reason = ReasonReferenceNotFound
			}
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, reason, fmt.Sprintf("route %q has an invalid upstream reference %q", route.Name, ref.Name))
			valid = false
			continue
		}
		seen[ref.Name] = true
		clusters = append(clusters, &routev3.WeightedCluster_ClusterWeight{Name: ref.Name, Weight: wrapperspb.UInt32(uint32(ref.Weight))})
	}
	return clusters, valid
}

// applyHostRewrite 使用 Route 级语义生成 Envoy Host 重写，服务地址模式会跟随实际选中的端点
func (c *compilation) applyHostRewrite(route *gatewayv1.Route, action *routev3.RouteAction) bool {
	rewrite := route.Spec.HostRewrite
	if rewrite == nil || rewrite.Mode == gatewayv1.HostRewritePreserve {
		return true
	}

	switch rewrite.Mode {
	case gatewayv1.HostRewriteServiceAddress:
		action.HostRewriteSpecifier = &routev3.RouteAction_AutoHostRewrite{AutoHostRewrite: wrapperspb.Bool(true)}
	case gatewayv1.HostRewriteCustom:
		if !validEndpointAddress(rewrite.Hostname) {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonInvalidSpec, fmt.Sprintf("route %q has an invalid host rewrite", route.Name))
			return false
		}
		action.HostRewriteSpecifier = &routev3.RouteAction_HostRewriteLiteral{HostRewriteLiteral: rewrite.Hostname}
	default:
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonUnsupported, fmt.Sprintf("route %q uses unsupported host rewrite mode %q", route.Name, rewrite.Mode))
		return false
	}
	return true
}

func (c *compilation) routeRetryPolicy(route *gatewayv1.Route) (*routev3.RetryPolicy, bool) {
	retry := route.Spec.Retry
	if retry.Attempts < minRetryAttempts || retry.Attempts > maxRetryAttempts ||
		retry.PerTryTimeoutMillis < minPerTryTimeoutMillis || retry.PerTryTimeoutMillis > maxPerTryTimeoutMillis ||
		(route.Spec.Timeout != nil && retry.PerTryTimeoutMillis > route.Spec.Timeout.RequestMillis) {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonInvalidSpec, fmt.Sprintf("route %q has an invalid retry policy", route.Name))
		return nil, false
	}
	return &routev3.RetryPolicy{
		RetryOn:       defaultRetryOn,
		NumRetries:    wrapperspb.UInt32(uint32(retry.Attempts)),
		PerTryTimeout: durationpb.New(time.Duration(retry.PerTryTimeoutMillis) * time.Millisecond),
	}, true
}
