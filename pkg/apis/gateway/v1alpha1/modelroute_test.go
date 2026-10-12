package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestModelRouteDefaultingPreservesMatches(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v, want nil", err)
	}
	for _, tt := range []struct {
		name  string
		input HeaderMatchType
		want  HeaderMatchType
	}{
		{name: "omitted type", want: HeaderMatchExact},
		{name: "explicit exact", input: HeaderMatchExact, want: HeaderMatchExact},
		{name: "unsupported type preserved for validation", input: "RegularExpression", want: "RegularExpression"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			route := &ModelRoute{Spec: ModelRouteSpec{
				Rules: []ModelRouteRule{{
					Matches: []ModelRouteMatch{{
						Headers: []HTTPHeaderMatch{{
							Name:  "X-AI-EG-MODEL",
							Type:  tt.input,
							Value: " Coding ",
						}},
					}},
					BackendRefs: []AIBackendReference{{Name: "model-service", ModelNameOverride: "Code-V2"}},
				}},
			}}
			want := route.DeepCopy()
			want.Spec.Rules[0].Matches[0].Headers[0].Type = tt.want
			scheme.Default(route)
			if diff := cmp.Diff(want, route); diff != "" {
				t.Errorf("Scheme.Default(ModelRoute %s) mismatch (-want +got):\n%s", tt.name, diff)
			}

			list := &ModelRouteList{Items: []ModelRoute{*route.DeepCopy()}}
			list.Items[0].Spec.Rules[0].Matches[0].Headers[0].Type = tt.input
			scheme.Default(list)
			if diff := cmp.Diff(*want, list.Items[0]); diff != "" {
				t.Errorf("Scheme.Default(ModelRouteList %s) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}

	for _, matches := range [][]ModelRouteMatch{nil, {}, {{}}} {
		route := &ModelRoute{Spec: ModelRouteSpec{Rules: []ModelRouteRule{{Matches: matches}}}}
		want := route.DeepCopy()
		scheme.Default(route)
		if diff := cmp.Diff(want, route); diff != "" {
			t.Errorf("Scheme.Default(ModelRoute catch-all %+v) changed input (-want +got):\n%s", matches, diff)
		}
	}
}

func TestModelRouteConversion(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := gateway.AddToScheme(scheme); err != nil {
		t.Fatalf("gateway.AddToScheme() error = %v, want nil", err)
	}
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v, want nil", err)
	}
	input := &ModelRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "models", ResourceVersion: "12", Generation: 3},
		Spec: ModelRouteSpec{
			DisplayName: "模型路由",
			ParentRefs:  []ParentReference{{Name: "edge", SectionName: "http"}},
			Rules: []ModelRouteRule{
				{
					Name: "coding",
					Matches: []ModelRouteMatch{{
						Headers: []HTTPHeaderMatch{{
							Name:  "x-ai-eg-model",
							Type:  HeaderMatchExact,
							Value: "coding",
						}},
					}},
					BackendRefs: []AIBackendReference{{Name: "model-service", ModelNameOverride: "code-v2"}},
				},
				{BackendRefs: []AIBackendReference{{Name: "fallback-service"}}},
			},
		},
		Status: ModelRouteStatus{Parents: []RouteParentStatus{{
			ParentRef:      ParentReference{Name: "edge", SectionName: "http"},
			ControllerName: "gateway.ingate.io/test",
			Conditions:     []metav1.Condition{{Type: "ResolvedRefs", ObservedGeneration: 3}},
		}}},
	}
	var internal gateway.ModelRoute
	if err := scheme.Convert(input, &internal, nil); err != nil {
		t.Fatalf("Scheme.Convert(ModelRoute, internal) error = %v, want nil", err)
	}
	wantSpec := gateway.ModelRouteSpec{
		DisplayName: "模型路由",
		ParentRefs:  []gateway.ParentReference{{Name: "edge", SectionName: "http"}},
		Rules: []gateway.ModelRouteRule{
			{
				Name: "coding",
				Matches: []gateway.ModelRouteMatch{{
					Headers: []gateway.HTTPHeaderMatch{{
						Name:  "x-ai-eg-model",
						Type:  gateway.HeaderMatchExact,
						Value: "coding",
					}},
				}},
				BackendRefs: []gateway.AIBackendReference{{Name: "model-service", ModelNameOverride: "code-v2"}},
			},
			{BackendRefs: []gateway.AIBackendReference{{Name: "fallback-service"}}},
		},
	}
	if diff := cmp.Diff(wantSpec, internal.Spec); diff != "" {
		t.Errorf("Scheme.Convert(ModelRoute, internal).Spec mismatch (-want +got):\n%s", diff)
	}
	var roundTrip ModelRoute
	if err := scheme.Convert(&internal, &roundTrip, nil); err != nil {
		t.Fatalf("Scheme.Convert(internal ModelRoute, external) error = %v, want nil", err)
	}
	if diff := cmp.Diff(input, &roundTrip); diff != "" {
		t.Errorf("ModelRoute conversion round trip mismatch (-want +got):\n%s", diff)
	}
}
