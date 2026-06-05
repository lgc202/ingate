package dto

import (
	"testing"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestRouteRequestResourceUsesWeightedTargets(t *testing.T) {
	t.Parallel()

	request := RouteRequest{
		Methods:      []HTTPMethod{HTTPMethodGET},
		Path:         "/v1/orders",
		GatewayNames: []string{"gw"},
		ServiceName:  "order-svc",
		Targets: []TargetService{
			{Name: "order-svc", Weight: 90},
			{Name: "order-canary", Weight: 10},
		},
		Enabled: true,
	}

	if err := request.Validate(); err != nil {
		t.Fatalf("validate route request: %v", err)
	}

	route, err := request.Resource()
	if err != nil {
		t.Fatalf("build route resource: %v", err)
	}

	refs := route.Spec.Rules[0].UpstreamRefs
	if got, want := len(refs), 2; got != want {
		t.Fatalf("upstream refs length = %d, want %d", got, want)
	}
	if got, want := refs[1].Name, "order-canary"; got != want {
		t.Fatalf("second upstream name = %q, want %q", got, want)
	}
	if got, want := refs[1].Weight, 10; got != want {
		t.Fatalf("second upstream weight = %d, want %d", got, want)
	}
}

func TestRouteRequestResourceUsesTypedRouteNativePolicies(t *testing.T) {
	t.Parallel()

	request := RouteRequest{
		Methods:      []HTTPMethod{HTTPMethodGET},
		Path:         "/v1/orders",
		GatewayNames: []string{"gw"},
		ServiceName:  "order-svc",
		Targets: []TargetService{
			{Name: "order-svc", Weight: 100},
		},
		Enabled: true,
		PolicyBindings: []PolicyBindingRequest{
			{
				Capability: routePolicyRequestHeaderModifier,
				Source:     routePolicySourceNative,
				Parameters: map[string]any{
					paramSetHeadersOn:    []any{"x-ingate-tenant"},
					paramHeaderValue:     "acme",
					paramRemoveHeadersOn: []any{"x-debug-token"},
				},
			},
			{
				Capability: routePolicyTimeout,
				Source:     routePolicySourceNative,
				Parameters: map[string]any{
					paramTimeoutMillis: "1500",
				},
			},
			{
				Capability: routePolicyRetry,
				Source:     routePolicySourceNative,
				Parameters: map[string]any{
					paramRetryAttempts:       "2",
					paramPerTryTimeoutMillis: "500",
				},
			},
		},
	}

	if err := request.Validate(); err != nil {
		t.Fatalf("RouteRequest.Validate() error = %v", err)
	}

	route, err := request.Resource()
	if err != nil {
		t.Fatalf("RouteRequest.Resource() error = %v", err)
	}

	if _, ok := route.Annotations["route.ingate.io/policy-bindings"]; ok {
		t.Fatalf("RouteRequest.Resource() annotations include legacy policy binding annotation")
	}
	rule := route.Spec.Rules[0]
	if rule.Timeout == nil || rule.Timeout.RequestMillis != 1500 {
		t.Fatalf("RouteRequest.Resource() timeout = %#v, want requestMillis 1500", rule.Timeout)
	}
	if rule.Retry == nil || rule.Retry.Attempts != 2 || rule.Retry.PerTryTimeoutMillis != 500 {
		t.Fatalf("RouteRequest.Resource() retry = %#v, want attempts 2 and perTryTimeoutMillis 500", rule.Retry)
	}
	if got, want := len(rule.Filters), 1; got != want {
		t.Fatalf("RouteRequest.Resource() filters length = %d, want %d", got, want)
	}
	filter := rule.Filters[0]
	if got, want := filter.Type, resource.RouteFilterRequestHeaderModifier; got != want {
		t.Fatalf("RouteRequest.Resource() filter type = %q, want %q", got, want)
	}
	if filter.RequestHeaderModifier == nil {
		t.Fatalf("RouteRequest.Resource() request header modifier is nil")
	}
	if got, want := filter.RequestHeaderModifier.Set[0], (resource.HeaderValue{Name: "x-ingate-tenant", Value: "acme"}); got != want {
		t.Fatalf("RouteRequest.Resource() request header set = %#v, want %#v", got, want)
	}
	if got, want := filter.RequestHeaderModifier.Remove, []string{"x-debug-token"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("RouteRequest.Resource() request header remove = %#v, want %#v", got, want)
	}
}
