package upstream

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/lgc202/ingate/internal/adminapi/domain"
)

func TestUpstreamToProtoOrdersEndpointsWithoutMutatingInput(t *testing.T) {
	input := []domain.Endpoint{
		{Address: "b.example.com", Port: 8080, Weight: 3},
		{Address: "a.example.com", Port: 9090, Weight: 2},
		{Address: "a.example.com", Port: 8080, Weight: 1},
	}
	upstream := domain.Upstream{Endpoints: slices.Clone(input)}

	response := upstreamToProto(upstream)
	got := make([]domain.Endpoint, len(response.GetEndpoints()))
	for i, endpoint := range response.GetEndpoints() {
		got[i] = domain.Endpoint{
			Address: endpoint.GetAddress(),
			Port:    endpoint.GetPort(),
			Weight:  endpoint.GetWeight(),
		}
	}

	want := []domain.Endpoint{
		{Address: "a.example.com", Port: 8080, Weight: 1},
		{Address: "a.example.com", Port: 9090, Weight: 2},
		{Address: "b.example.com", Port: 8080, Weight: 3},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("upstreamToProto() endpoints mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(input, upstream.Endpoints); diff != "" {
		t.Errorf("upstreamToProto() modified input (-want +got):\n%s", diff)
	}
}
