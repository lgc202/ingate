package gateway

import (
	"testing"

	"k8s.io/apiserver/pkg/registry/rest"

	routestorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway/route"
	upstreamstorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway/upstream"
)

func TestStatusRESTOnlyExposesStatusOperations(t *testing.T) {
	for _, tt := range []struct {
		name    string
		storage rest.Storage
	}{
		{name: "Route", storage: &routestorage.StatusREST{}},
		{name: "Upstream", storage: &upstreamstorage.StatusREST{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := tt.storage.(rest.Creater); ok {
				t.Error("StatusREST implements rest.Creater, want false")
			}
			if _, ok := tt.storage.(rest.Lister); ok {
				t.Error("StatusREST implements rest.Lister, want false")
			}
			if _, ok := tt.storage.(rest.GracefulDeleter); ok {
				t.Error("StatusREST implements rest.GracefulDeleter, want false")
			}
			if _, ok := tt.storage.(rest.Watcher); ok {
				t.Error("StatusREST implements rest.Watcher, want false")
			}
		})
	}
}
