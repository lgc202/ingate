package dto

import (
	"testing"

	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFromListResultReturnsRoutesOnly(t *testing.T) {
	t.Parallel()

	response := FromListResult(&routeservice.ListResult{
		Routes: []resource.Route{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "users"},
				Spec: resource.RouteSpec{
					ParentRefs: []string{"gw"},
					Hostnames:  []string{"api.example.com"},
					Rules: []resource.RouteRule{{
						PathPrefix: "/v1/users",
						Methods:    []string{},
						UpstreamRefs: []resource.UpstreamRef{{
							Name:   "app",
							Weight: 100,
						}, {
							Name:   "canary",
							Weight: 20,
						}},
					}},
				},
			},
		},
	})

	if got, want := len(response.Routes), 1; got != want {
		t.Fatalf("routes length = %d, want %d", got, want)
	}
	if got, want := response.Routes[0].Path, "/v1/users"; got != want {
		t.Fatalf("route path = %q, want %q", got, want)
	}
	if got, want := response.Routes[0].ServiceName, "app"; got != want {
		t.Fatalf("route service = %q, want %q", got, want)
	}
	if got, want := len(response.Routes[0].Targets), 2; got != want {
		t.Fatalf("route targets length = %d, want %d", got, want)
	}
	if got, want := response.Routes[0].Targets[1].Name, "canary"; got != want {
		t.Fatalf("route second target = %q, want %q", got, want)
	}
}

func TestPolicyCapabilitiesReturnsSupportedRoutePolicies(t *testing.T) {
	t.Parallel()

	response := PolicyCapabilities()

	if got, want := len(response.Policies), 3; got != want {
		t.Fatalf("policies length = %d, want %d", got, want)
	}
	if got, want := response.Policies[0].Name, "请求 Header 改写"; got != want {
		t.Fatalf("first policy name = %q, want %q", got, want)
	}
}
