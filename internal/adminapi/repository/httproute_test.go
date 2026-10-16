package repository

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
)

func TestHTTPRouteRejectsInvalidContractBeforeWriting(t *testing.T) {
	for _, tt := range []struct {
		name   string
		update func(*gatewayv1alpha1.HTTPRoute)
	}{
		{name: "invalid identity", update: func(r *gatewayv1alpha1.HTTPRoute) { r.Name = "INVALID" }},
		{
			name: "duplicate parent",
			update: func(r *gatewayv1alpha1.HTTPRoute) {
				r.Spec.ParentRefs = append(r.Spec.ParentRefs, r.Spec.ParentRefs[0])
			},
		},
		{
			name: "overlapping parent",
			update: func(r *gatewayv1alpha1.HTTPRoute) {
				r.Spec.ParentRefs = append(r.Spec.ParentRefs, gatewayv1alpha1.ParentReference{
					Name:        "edge",
					SectionName: "http",
				})
			},
		},
		{name: "no rule", update: func(r *gatewayv1alpha1.HTTPRoute) { r.Spec.Rules = nil }},
		{
			name: "multiple targets",
			update: func(r *gatewayv1alpha1.HTTPRoute) {
				r.Spec.Rules[0].BackendRefs = append(r.Spec.Rules[0].BackendRefs, gatewayv1alpha1.BackendReference{Name: "other"})
			},
		},
		{
			name: "query path",
			update: func(r *gatewayv1alpha1.HTTPRoute) {
				r.Spec.Rules[0].Matches = []gatewayv1alpha1.HTTPRouteMatch{
					{Path: &gatewayv1alpha1.PathMatch{Value: "/api?q=1"}},
				}
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := clientfake.NewSimpleClientset()
			route := gatewayv1alpha1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "public"},
				Spec: gatewayv1alpha1.HTTPRouteSpec{
					ParentRefs: []gatewayv1alpha1.ParentReference{{Name: "edge"}},
					Rules: []gatewayv1alpha1.HTTPRouteRule{
						{BackendRefs: []gatewayv1alpha1.BackendReference{{Name: "backend"}}},
					},
				},
			}
			tt.update(&route)
			_, err := NewHTTPRoute(client.GatewayV1alpha1(), nil).Create(t.Context(), &route)
			if !errors.Is(err, ErrInvalid) {
				t.Errorf("Create error = %v, want ErrInvalid", err)
			}

			if actions := client.Actions(); len(actions) != 0 {
				t.Errorf("invalid write caused API actions: %v", actions)
			}
		})
	}
}

func TestHTTPRouteDefaultingCopiesInputAndPreservesRuleOrder(t *testing.T) {
	input := gatewayv1alpha1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "public"},
		Spec: gatewayv1alpha1.HTTPRouteSpec{
			ParentRefs: []gatewayv1alpha1.ParentReference{
				{Name: "edge", SectionName: "http"},
				{Name: "edge", SectionName: "admin"},
			},
			Rules: []gatewayv1alpha1.HTTPRouteRule{
				{Name: "z-first", BackendRefs: []gatewayv1alpha1.BackendReference{{Name: "first"}}},
				{
					Name: "a-second",
					Matches: []gatewayv1alpha1.HTTPRouteMatch{
						{Path: &gatewayv1alpha1.PathMatch{Type: gatewayv1alpha1.PathMatchExact, Value: "/api/"}},
						{},
					},
					BackendRefs: []gatewayv1alpha1.BackendReference{{Name: "second"}},
				},
			},
		},
	}
	before := input.DeepCopy()
	want := input.DeepCopy()
	gatewayv1alpha1.SetDefaultsHTTPRoute(want)
	client := clientfake.NewSimpleClientset()
	created, err := NewHTTPRoute(client.GatewayV1alpha1(), nil).Create(t.Context(), &input)
	if err != nil {
		t.Fatal(err)
	}

	if len(created.Rules) != 2 || len(created.ParentRefs) != 2 {
		t.Fatalf("lost route children: %+v", created)
	}

	if created.Rules[0].Name != "z-first" || created.Rules[1].Name != "a-second" {
		t.Errorf("rule order = %+v", created.Rules)
	}

	if got := created.Rules[0].Matches[0].Path; got == nil || got.Type != "PathPrefix" || got.Value != "/" {
		t.Errorf("default path = %+v", got)
	}

	stored, err := client.GatewayV1alpha1().HTTPRoutes("team-a").Get(t.Context(), input.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want.Spec, stored.Spec); diff != "" {
		t.Errorf("saved spec (-want +got):\n%s", diff)
	}

	created.Rules[1].Matches[0].Path.Value = "/changed"
	if diff := cmp.Diff(before, &input); diff != "" {
		t.Errorf("conversion mutated caller input (-want +got):\n%s", diff)
	}
}
