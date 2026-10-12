package gateway

import (
	"testing"

	"k8s.io/apiserver/pkg/registry/rest"

	aibackendstorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway/aibackend"
	backendstorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway/backend"
	routestorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway/httproute"
	mcproutestorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway/mcproute"
	modelroutestorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway/modelroute"
)

func TestStatusRESTOnlyExposesStatusOperations(t *testing.T) {
	for _, tt := range []struct {
		name    string
		storage rest.Storage
	}{
		{name: "HTTPRoute", storage: &routestorage.StatusREST{}},
		{name: "Backend", storage: &backendstorage.StatusREST{}},
		{name: "AIBackend", storage: &aibackendstorage.StatusREST{}},
		{name: "ModelRoute", storage: &modelroutestorage.StatusREST{}},
		{name: "MCPRoute", storage: &mcproutestorage.StatusREST{}},
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
