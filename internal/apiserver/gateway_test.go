package apiserver

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/lgc202/ingate/pkg/apis/gateway"
	"github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestGatewayAPIGroupInfo(t *testing.T) {
	scheme, err := newScheme()
	if err != nil {
		t.Fatalf("newScheme() error = %v, want nil", err)
	}

	mainREST := &testStorage{}
	statusREST := &testStorage{}
	storage := map[string]rest.Storage{
		"gateways":        mainREST,
		"gateways/status": statusREST,
	}

	info := gatewayAPIGroupInfo(scheme, storage)
	if info.Scheme != scheme {
		t.Errorf("gatewayAPIGroupInfo() scheme = %p, want %p", info.Scheme, scheme)
	}
	if len(info.PrioritizedVersions) != 1 || info.PrioritizedVersions[0] != v1alpha1.SchemeGroupVersion {
		t.Errorf("gatewayAPIGroupInfo() versions = %v, want [%s]",
			info.PrioritizedVersions, v1alpha1.SchemeGroupVersion)
	}
	gotStorage := info.VersionedResourcesStorageMap[v1alpha1.Version]
	if len(gotStorage) != len(storage) {
		t.Errorf("gatewayAPIGroupInfo() storage count = %d, want %d", len(gotStorage), len(storage))
	}
	if gotStorage["gateways"] != mainREST {
		t.Errorf("gatewayAPIGroupInfo() gateways storage = %T, want *testStorage", gotStorage["gateways"])
	}
	if gotStorage["gateways/status"] != statusREST {
		t.Errorf("gatewayAPIGroupInfo() gateways/status storage = %T, want *testStorage", gotStorage["gateways/status"])
	}

	values, err := info.ParameterCodec.EncodeParameters(
		&metav1.ListOptions{LabelSelector: "environment=production"},
		v1alpha1.SchemeGroupVersion,
	)
	if err != nil {
		t.Fatalf("gatewayAPIGroupInfo().ParameterCodec.EncodeParameters() error = %v, want nil", err)
	}
	if got := values.Get("labelSelector"); got != "environment=production" {
		t.Errorf("gatewayAPIGroupInfo().ParameterCodec.EncodeParameters() labelSelector = %q, want %q",
			got, "environment=production")
	}
}

type testStorage struct{}

func (*testStorage) New() runtime.Object { return &gateway.Gateway{} }

func (*testStorage) Destroy() {}
