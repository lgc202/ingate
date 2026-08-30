package compiler

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

const defaultRetryOn = "connect-failure,refused-stream,reset,5xx"

func (c *compilation) buildWeightedClusters(
	route *gatewayv1.Route,
	compiledUpstreams map[string]bool,
) ([]*routev3.WeightedCluster_ClusterWeight, bool) {
	if len(route.Spec.UpstreamRefs) == 0 {
		c.addRouteError(
			route.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("route %q must reference at least one upstream", route.Name),
		)
		return nil, false
	}
	if len(route.Spec.UpstreamRefs) > routeconfig.MaxServiceTargets {
		c.addRouteError(
			route.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("route %q has too many upstream targets", route.Name),
		)
		return nil, false
	}
	upstreamRefs := slices.Clone(route.Spec.UpstreamRefs)
	slices.SortFunc(upstreamRefs, func(a, b gatewayv1.UpstreamRef) int {
		return cmp.Compare(a.Name, b.Name)
	})
	clusters := make([]*routev3.WeightedCluster_ClusterWeight, 0, len(upstreamRefs))
	seenUpstreamIDs := make(map[string]bool, len(upstreamRefs))
	valid := true
	for _, upstreamRef := range upstreamRefs {
		upstream, exists := c.upstreams[upstreamRef.Name]
		compiledSuccessfully := compiledUpstreams[upstreamRef.Name]
		duplicateUpstream := seenUpstreamIDs[upstreamRef.Name]
		if upstreamRef.Name != "" {
			seenUpstreamIDs[upstreamRef.Name] = true
		}

		var reason Reason
		var message string
		switch {
		case !resourceconfig.IsCanonicalID(upstreamRef.Name) || duplicateUpstream ||
			upstreamRef.Weight < routeconfig.MinTargetWeight ||
			upstreamRef.Weight > routeconfig.MaxTargetWeight:
			reason = ReasonInvalidSpec
			message = fmt.Sprintf("route %q has an invalid upstream reference %q", route.Name, upstreamRef.Name)
		case !exists:
			reason = ReasonReferenceNotFound
			message = fmt.Sprintf("route %q references missing upstream %q", route.Name, upstreamRef.Name)
		case !compiledSuccessfully:
			reason = ReasonInvalidReference
			message = fmt.Sprintf("route %q references invalid upstream %q", route.Name, upstreamRef.Name)
		case upstream.Spec.Model != nil:
			reason = ReasonInvalidReference
			message = fmt.Sprintf(
				"route %q must publish model upstream %q through ai.models",
				route.Name,
				upstreamRef.Name,
			)
		default:
			clusters = append(clusters, &routev3.WeightedCluster_ClusterWeight{
				Name:   upstreamRef.Name,
				Weight: wrapperspb.UInt32(uint32(upstreamRef.Weight)),
			})
			continue
		}
		c.addRouteError(route.Name, reason, message)
		valid = false
	}
	return clusters, valid
}

func (c *compilation) buildRouteAction(
	route *gatewayv1.Route,
	clusters []*routev3.WeightedCluster_ClusterWeight,
) (*routev3.RouteAction, bool) {
	action := &routev3.RouteAction{
		ClusterSpecifier: &routev3.RouteAction_WeightedClusters{
			WeightedClusters: &routev3.WeightedCluster{Clusters: clusters},
		},
	}
	if !c.applyHostRewrite(route, action) {
		return nil, false
	}
	requestTimeoutMillis := route.Spec.Timeout.RequestMillis
	if requestTimeoutMillis < routeconfig.MinRequestTimeoutMillis ||
		requestTimeoutMillis > routeconfig.MaxRequestTimeoutMillis {
		c.addRouteError(
			route.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("route %q must declare a valid timeout", route.Name),
		)
		return nil, false
	}
	action.Timeout = durationpb.New(time.Duration(requestTimeoutMillis) * time.Millisecond)
	if route.Spec.Retry != nil {
		retry, ok := c.buildRetryPolicy(route, requestTimeoutMillis)
		if !ok {
			return nil, false
		}
		action.RetryPolicy = retry
	}
	return action, true
}

// applyHostRewrite 使用 Route 级语义生成 Envoy Host 重写，上游主机模式会跟随实际选中的端点。
func (c *compilation) applyHostRewrite(route *gatewayv1.Route, action *routev3.RouteAction) bool {
	rewrite := route.Spec.HostRewrite
	if rewrite.Mode == gatewayv1.HostRewritePreserve {
		return true
	}

	switch rewrite.Mode {
	case gatewayv1.HostRewriteUpstreamHost:
		action.HostRewriteSpecifier = &routev3.RouteAction_AutoHostRewrite{AutoHostRewrite: wrapperspb.Bool(true)}
	case gatewayv1.HostRewriteCustom:
		hostname, ok := hostnameutil.Normalize(rewrite.Hostname)
		if !ok || hostname == "*" {
			c.addRouteError(
				route.Name,
				ReasonInvalidSpec,
				fmt.Sprintf("route %q has an invalid host rewrite", route.Name),
			)
			return false
		}
		action.HostRewriteSpecifier = &routev3.RouteAction_HostRewriteLiteral{
			HostRewriteLiteral: hostname,
		}
	default:
		c.addRouteError(
			route.Name,
			ReasonUnsupported,
			fmt.Sprintf("route %q uses unsupported host rewrite mode %q", route.Name, rewrite.Mode),
		)
		return false
	}
	return true
}

func (c *compilation) buildRetryPolicy(
	route *gatewayv1.Route,
	requestTimeoutMillis int,
) (*routev3.RetryPolicy, bool) {
	retry := route.Spec.Retry
	if retry.Attempts < routeconfig.MinRetryAttempts ||
		retry.Attempts > routeconfig.MaxRetryAttempts ||
		retry.PerTryTimeoutMillis < routeconfig.MinPerTryTimeoutMillis ||
		retry.PerTryTimeoutMillis > routeconfig.MaxPerTryTimeoutMillis ||
		retry.PerTryTimeoutMillis > requestTimeoutMillis {
		c.addRouteError(
			route.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("route %q has an invalid retry policy", route.Name),
		)
		return nil, false
	}
	return &routev3.RetryPolicy{
		RetryOn:       defaultRetryOn,
		NumRetries:    wrapperspb.UInt32(uint32(retry.Attempts)),
		PerTryTimeout: durationpb.New(time.Duration(retry.PerTryTimeoutMillis) * time.Millisecond),
	}, true
}
