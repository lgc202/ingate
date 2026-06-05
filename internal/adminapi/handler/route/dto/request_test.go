package dto

import "testing"

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
