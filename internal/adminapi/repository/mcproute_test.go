package repository

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
)

func TestMCPRouteUpdatePreservesAPIConflictCause(t *testing.T) {
	seed := &gatewayv1alpha1.MCPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "tools", ResourceVersion: "7"},
		Spec: gatewayv1alpha1.MCPRouteSpec{
			ParentRefs:  []gatewayv1alpha1.ParentReference{{Name: "edge"}},
			Path:        ptr.To("/tools/mcp"),
			BackendRefs: []gatewayv1alpha1.MCPBackendReference{{Name: "backend", Path: ptr.To("/mcp")}},
		},
	}
	client := clientfake.NewSimpleClientset(seed)
	resource := gatewayv1alpha1.SchemeGroupVersion.WithResource("mcproutes").GroupResource()
	storageErr := apierrors.NewConflict(resource, "tools", errors.New("concurrent update"))
	client.PrependReactor("update", "mcproutes", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, storageErr
	})
	routes := NewMCPRoute(client.GatewayV1alpha1().MCPRoutes(), nil)
	next := seed.DeepCopy()
	next.Spec.DisplayName = "新名称"
	got, err := routes.Update(t.Context(), next)
	if !errors.Is(err, ErrConflict) || !errors.Is(err, storageErr) {
		t.Errorf("MCPRoute.Update(API race) error = %v, want ErrConflict and API cause", err)
	}
	if diff := cmp.Diff(MCPRouteInfo{}, got); diff != "" {
		t.Errorf("MCPRoute.Update(API race) returned partial result (-want +got):\n%s", diff)
	}
	stored, err := client.GatewayV1alpha1().MCPRoutes().Get(t.Context(), "tools", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(MCPRoute after API race) error = %v, want nil", err)
	}
	if diff := cmp.Diff(seed, stored); diff != "" {
		t.Errorf("MCPRoute.Update(API race) changed state (-want +got):\n%s", diff)
	}
}

func TestMCPRouteInfoDoesNotAliasAPIObject(t *testing.T) {
	route := &gatewayv1alpha1.MCPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "tools"},
		Spec: gatewayv1alpha1.MCPRouteSpec{
			ParentRefs:  []gatewayv1alpha1.ParentReference{{Name: "edge"}},
			Path:        ptr.To("/tools/mcp"),
			BackendRefs: []gatewayv1alpha1.MCPBackendReference{{Name: "backend", Path: ptr.To("/mcp")}},
		},
	}
	want := route.DeepCopy()
	info := mcpRouteFromAPI(route)
	info.ParentRefs[0].Name = "other-edge"
	*info.Path = "/other"
	info.BackendRefs[0].Name = "other-backend"
	*info.BackendRefs[0].Path = "/other-upstream"
	if diff := cmp.Diff(want, route); diff != "" {
		t.Errorf("mutating MCPRouteInfo changed API object (-want +got):\n%s", diff)
	}
}
