package projection

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestModelRouteProjectionPreservesOrderAndCatchAll(t *testing.T) {
	route := &gatewayv1alpha1.ModelRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "models", ResourceVersion: "9"},
		Spec: gatewayv1alpha1.ModelRouteSpec{
			ParentRefs: []gatewayv1alpha1.ParentReference{
				{Name: "edge", SectionName: "z"},
				{Name: "edge", SectionName: "a"},
			},
			Rules: []gatewayv1alpha1.ModelRouteRule{
				{
					Name: "z-rule",
					Matches: []gatewayv1alpha1.ModelRouteMatch{
						{Headers: []gatewayv1alpha1.HTTPHeaderMatch{{
							Name:  "X-Ai-Eg-Model",
							Type:  gatewayv1alpha1.HeaderMatchExact,
							Value: " Model-Z ",
						}}},
						{},
					},
					BackendRefs: []gatewayv1alpha1.AIBackendReference{{Name: "models-z", ModelNameOverride: "provider-model"}},
				},
				{Name: "a-rule", BackendRefs: []gatewayv1alpha1.AIBackendReference{{Name: "models-a"}}},
			},
		},
	}
	before := route.DeepCopy()
	got := modelRouteToMySQL(route)
	want := mysql.ModelRoute{
		Name: "models",
		Parents: []mysql.ModelRouteParent{
			{Name: "edge", SectionName: "z"},
			{Name: "edge", SectionName: "a"},
		},
		Rules: []mysql.ModelRouteRule{
			{
				Name: "z-rule",
				Matches: []mysql.ModelRouteMatch{
					{Headers: []mysql.ModelRouteHeaderMatch{{
						Name:  "X-Ai-Eg-Model",
						Type:  "Exact",
						Value: " Model-Z ",
					}}},
					{},
				},
				BackendRefs: []mysql.ModelRouteBackendRef{{Name: "models-z", ModelNameOverride: "provider-model"}},
			},
			{Name: "a-rule", BackendRefs: []mysql.ModelRouteBackendRef{{Name: "models-a"}}},
		},
		Version: "9",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("modelRouteToMySQL(order, catch-all and model override) mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(before, route); diff != "" {
		t.Errorf("modelRouteToMySQL() changed input (-want +got):\n%s", diff)
	}
}
