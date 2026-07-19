package route

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestCreateRouteReqSpec(t *testing.T) {
	enabled := false
	request := CreateRouteReq{
		Name:       " Orders ",
		GatewayIDs: []string{"gateway-1"},
		Hostnames:  []string{"API.EXAMPLE.COM"},
		Enabled:    &enabled,
		Rules: []RouteRule{
			{
				Name:       "main",
				PathPrefix: "/orders",
				Methods:    []string{"GET"},
				Headers:    []HeaderMatchReq{{Name: "X-Tenant", Value: "acme"}},
				Targets:    []RouteTarget{{UpstreamID: "upstream-1", Weight: 100}},
				RequestHeaderModifier: &HeaderModifierReq{
					Set:    []HeaderValueReq{{Name: "X-Request-ID", Value: "request-id"}},
					Remove: []string{"X-Legacy"},
				},
				ResponseHeaderModifier: &HeaderModifierReq{
					Set: []HeaderValueReq{{Name: "X-Source", Value: "ingate"}},
				},
				Timeout: &RouteTimeoutReq{RequestMillis: 30000},
				Retry:   &RouteRetryReq{Attempts: 2, PerTryTimeoutMillis: 1000},
			},
		},
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("CreateRouteReq.Validate() error = %v, want nil", err)
	}
	want := resource.RouteSpec{
		DisplayName: "Orders",
		Enabled:     false,
		ParentRefs:  []resource.ParentRef{{Name: "gateway-1"}},
		Hostnames:   []string{"api.example.com"},
		Rules: []resource.RouteRule{
			{
				Name:         "main",
				PathPrefix:   "/orders",
				Methods:      []string{"GET"},
				Headers:      []resource.HeaderMatch{{Name: "x-tenant", Value: "acme"}},
				UpstreamRefs: []resource.UpstreamRef{{Name: "upstream-1", Weight: 100}},
				Filters: []resource.RouteFilter{
					{
						Type: resource.RouteFilterRequestHeaderModifier,
						RequestHeaderModifier: &resource.HeaderModifier{
							Set:    []resource.HeaderValue{{Name: "x-request-id", Value: "request-id"}},
							Remove: []string{"x-legacy"},
						},
					},
					{
						Type: resource.RouteFilterResponseHeaderModifier,
						ResponseHeaderModifier: &resource.HeaderModifier{
							Set:    []resource.HeaderValue{{Name: "x-source", Value: "ingate"}},
							Remove: []string{},
						},
					},
				},
				Timeout: &resource.RouteTimeout{RequestMillis: 30000},
				Retry:   &resource.RouteRetry{Attempts: 2, PerTryTimeoutMillis: 1000},
			},
		},
	}

	if diff := cmp.Diff(want, request.Spec()); diff != "" {
		t.Errorf("CreateRouteReq.Spec() mismatch (-want +got):\n%s", diff)
	}
}

func TestCreateRouteReqSpecModelRouting(t *testing.T) {
	request := CreateRouteReq{
		Name:       "模型路由",
		GatewayIDs: []string{"gateway-1"},
		Rules:      []RouteRule{modelRouteRule()},
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("CreateRouteReq.Validate() error = %v, want nil", err)
	}
	want := resource.RouteSpec{
		DisplayName: "模型路由",
		Enabled:     true,
		ParentRefs:  []resource.ParentRef{{Name: "gateway-1"}},
		Hostnames:   []string{},
		Rules: []resource.RouteRule{
			{
				Name:         "chat",
				PathPrefix:   "/v1/chat/completions",
				Methods:      []string{"POST"},
				Headers:      []resource.HeaderMatch{},
				UpstreamRefs: []resource.UpstreamRef{},
				ModelRouting: &resource.ModelRouting{
					Models: []resource.ModelRoute{
						{Model: "chat-default", UpstreamRef: "model-1", UpstreamModel: "gpt-4o-mini"},
					},
				},
			},
		},
	}

	if diff := cmp.Diff(want, request.Spec()); diff != "" {
		t.Errorf("CreateRouteReq.Spec() model route mismatch (-want +got):\n%s", diff)
	}
}
