package repository

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
)

func TestModelRouteUpdatePreservesAPIConflictCause(t *testing.T) {
	seed := &gatewayv1alpha1.ModelRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "models", ResourceVersion: "7"},
		Spec: gatewayv1alpha1.ModelRouteSpec{
			ParentRefs: []gatewayv1alpha1.ParentReference{{Name: "edge"}},
			Rules: []gatewayv1alpha1.ModelRouteRule{{
				BackendRefs: []gatewayv1alpha1.AIBackendReference{{Name: "backend"}},
			}},
		},
	}
	client := clientfake.NewSimpleClientset(seed)
	resource := gatewayv1alpha1.SchemeGroupVersion.WithResource("modelroutes").GroupResource()
	storageErr := apierrors.NewConflict(resource, "models", errors.New("concurrent update"))
	client.PrependReactor("update", "modelroutes", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, storageErr
	})
	routes := NewModelRoute(client.GatewayV1alpha1().ModelRoutes(), nil)
	next := seed.DeepCopy()
	next.Spec.DisplayName = "新名称"
	got, err := routes.Update(t.Context(), next)
	if !errors.Is(err, ErrConflict) || !errors.Is(err, storageErr) {
		t.Errorf("ModelRoute.Update(API race) error = %v, want ErrConflict and API cause", err)
	}
	if diff := cmp.Diff(ModelRouteInfo{}, got); diff != "" {
		t.Errorf("ModelRoute.Update(API race) returned partial result (-want +got):\n%s", diff)
	}
	stored, err := client.GatewayV1alpha1().ModelRoutes().Get(t.Context(), "models", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(ModelRoute after API race) error = %v, want nil", err)
	}
	if diff := cmp.Diff(seed, stored); diff != "" {
		t.Errorf("ModelRoute.Update(API race) changed state (-want +got):\n%s", diff)
	}
}

func TestModelRouteInfoDoesNotAliasAPIObject(t *testing.T) {
	route := &gatewayv1alpha1.ModelRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "models"},
		Spec: gatewayv1alpha1.ModelRouteSpec{
			ParentRefs: []gatewayv1alpha1.ParentReference{{Name: "edge"}},
			Rules: []gatewayv1alpha1.ModelRouteRule{{
				Matches: []gatewayv1alpha1.ModelRouteMatch{{
					Headers: []gatewayv1alpha1.HTTPHeaderMatch{{
						Name:  "x-ai-eg-model",
						Type:  gatewayv1alpha1.HeaderMatchExact,
						Value: "model",
					}},
				}},
				BackendRefs: []gatewayv1alpha1.AIBackendReference{{Name: "backend", ModelNameOverride: "provider-model"}},
			}},
		},
	}
	want := route.DeepCopy()
	view := modelRouteFromAPI(route)
	view.ParentRefs[0].Name = "other-edge"
	view.Rules[0].Matches[0].Headers[0].Value = "other-model"
	view.Rules[0].BackendRefs[0].ModelNameOverride = "other-provider-model"
	if diff := cmp.Diff(want, route); diff != "" {
		t.Errorf("mutating ModelRouteInfo changed API object (-want +got):\n%s", diff)
	}
}
