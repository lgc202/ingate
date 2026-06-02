package dto

import (
	"testing"

	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFromWorkspaceResultUsesRouteDataForComposer(t *testing.T) {
	t.Parallel()

	response := FromWorkspaceResult(&routeservice.WorkspaceResult{
		Gateways: []resource.Gateway{
			{ObjectMeta: metav1.ObjectMeta{Name: "gw"}},
		},
		Upstreams: []resource.Upstream{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app"},
				Spec: resource.UpstreamSpec{
					Endpoints: []resource.Endpoint{{Address: "127.0.0.1", Port: 8080}},
				},
			},
		},
		Routes: []resource.Route{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "users",
				},
				Spec: resource.RouteSpec{
					ParentRefs: []string{"gw"},
					Hostnames:  []string{"api.example.com"},
					Rules: []resource.RouteRule{{
						PathPrefix: "/v1/users",
						Methods:    []string{},
						UpstreamRefs: []resource.UpstreamRef{{
							Name:   "app",
							Weight: 100,
						}},
					}},
				},
			},
		},
	})

	if got, want := response.Routes[0].Methods, []HTTPMethod{}; len(got) != len(want) {
		t.Fatalf("route methods = %v, want %v", got, want)
	}
	if got, want := response.Composer.Methods, response.Routes[0].Methods; len(got) != len(want) {
		t.Fatalf("composer methods = %v, want route methods %v", got, want)
	}
	if got, want := response.Composer.Path, "/v1/users"; got != want {
		t.Fatalf("composer path = %q, want %q", got, want)
	}
	if got, want := response.Composer.ServiceName, "app"; got != want {
		t.Fatalf("composer service = %q, want %q", got, want)
	}
	if got, want := response.Composer.GatewayNames, []string{"gw"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("composer gateways = %v, want %v", got, want)
	}
	if got, want := response.Composer.Hostnames, []string{"api.example.com"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("composer hostnames = %v, want %v", got, want)
	}
}

func TestFromWorkspaceResultDefaultsComposerToAllMethods(t *testing.T) {
	t.Parallel()

	response := FromWorkspaceResult(&routeservice.WorkspaceResult{
		Gateways: []resource.Gateway{
			{ObjectMeta: metav1.ObjectMeta{Name: "gw"}},
		},
		Upstreams: []resource.Upstream{
			{ObjectMeta: metav1.ObjectMeta{Name: "app"}},
		},
	})

	if got := response.Composer.Methods; len(got) != 0 {
		t.Fatalf("composer methods = %v, want empty all-methods match", got)
	}
	if got, want := response.Composer.Path, "/"; got != want {
		t.Fatalf("composer path = %q, want %q", got, want)
	}
	if got, want := response.Composer.GatewayNames, []string{"gw"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("composer gateways = %v, want %v", got, want)
	}
	if got, want := response.Composer.ServiceName, "app"; got != want {
		t.Fatalf("composer service = %q, want %q", got, want)
	}
}
