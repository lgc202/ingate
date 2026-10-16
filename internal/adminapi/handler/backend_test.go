package handler

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestBackendToProtoOrdersEndpointsWithoutMutatingInput(t *testing.T) {
	endpoints := []gatewayv1alpha1.Endpoint{
		{Address: "b.example.com", Port: 8080, Weight: 3},
		{Address: "a.example.com", Port: 9090, Weight: 2},
		{Address: "a.example.com", Port: 8080, Weight: 1},
	}
	backend := repository.BackendInfo{Endpoints: slices.Clone(endpoints)}

	response := backendToProto(backend)
	got := make([]gatewayv1alpha1.Endpoint, len(response.GetEndpoints()))
	for i, endpoint := range response.GetEndpoints() {
		got[i] = gatewayv1alpha1.Endpoint{
			Address: endpoint.GetAddress(),
			Port:    endpoint.GetPort(),
			Weight:  endpoint.GetWeight(),
		}
	}

	want := []gatewayv1alpha1.Endpoint{
		{Address: "a.example.com", Port: 8080, Weight: 1},
		{Address: "a.example.com", Port: 9090, Weight: 2},
		{Address: "b.example.com", Port: 8080, Weight: 3},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("backendToProto() endpoints mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(endpoints, backend.Endpoints); diff != "" {
		t.Errorf("backendToProto() modified input (-want +got):\n%s", diff)
	}
}
