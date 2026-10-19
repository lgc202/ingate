package gateway

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestProjectGateway(t *testing.T) {
	resource := &gatewayv1alpha1.Gateway{
		Spec: gatewayv1alpha1.GatewaySpec{
			DisplayName: "对外入口",
			Listeners:   []gatewayv1alpha1.Listener{{Name: "http", Protocol: gatewayv1alpha1.ProtocolHTTP, Port: 8080}},
		},
		Status: gatewayv1alpha1.GatewayStatus{Conditions: []metav1.Condition{{Type: "Accepted"}}},
	}
	want := &gatewayv1alpha1.Gateway{Spec: gatewayv1alpha1.GatewaySpec{
		Listeners: []gatewayv1alpha1.Listener{{Name: "http", Protocol: gatewayv1alpha1.ProtocolHTTP, Port: 8080}},
	}}

	testResourceProjection(t, resource, projectGateway, want, func(gateway *gatewayv1alpha1.Gateway) {
		gateway.Spec.Listeners[0].Port = 9090
	})
}

func TestProjectHTTPRoute(t *testing.T) {
	resource := &gatewayv1alpha1.HTTPRoute{
		Spec: gatewayv1alpha1.HTTPRouteSpec{
			DisplayName: "订单路由",
			ParentRefs:  []gatewayv1alpha1.ParentReference{{Name: "edge", SectionName: "http"}},
			Rules: []gatewayv1alpha1.HTTPRouteRule{{
				Matches:     []gatewayv1alpha1.HTTPRouteMatch{{Path: &gatewayv1alpha1.PathMatch{Value: "/orders"}}},
				BackendRefs: []gatewayv1alpha1.BackendReference{{Name: "orders"}},
			}},
		},
		Status: gatewayv1alpha1.HTTPRouteStatus{Parents: []gatewayv1alpha1.RouteParentStatus{{ControllerName: controllerName}}},
	}
	want := &gatewayv1alpha1.HTTPRoute{Spec: gatewayv1alpha1.HTTPRouteSpec{
		ParentRefs: []gatewayv1alpha1.ParentReference{{Name: "edge", SectionName: "http"}},
		Rules: []gatewayv1alpha1.HTTPRouteRule{{
			Matches:     []gatewayv1alpha1.HTTPRouteMatch{{Path: &gatewayv1alpha1.PathMatch{Value: "/orders"}}},
			BackendRefs: []gatewayv1alpha1.BackendReference{{Name: "orders"}},
		}},
	}}

	testResourceProjection(t, resource, projectHTTPRoute, want, func(route *gatewayv1alpha1.HTTPRoute) {
		route.Spec.ParentRefs[0].SectionName = "other"
		route.Spec.Rules[0].Matches[0].Path.Value = "/v2/orders"
		route.Spec.Rules[0].BackendRefs[0].Name = "other"
	})
}

func TestProjectAIBackend(t *testing.T) {
	resource := &gatewayv1alpha1.AIBackend{
		Spec: gatewayv1alpha1.AIBackendSpec{
			DisplayName: "模型提供方",
			BackendRef:  gatewayv1alpha1.BackendReference{Name: "provider"},
			Schema:      gatewayv1alpha1.APISchema{Name: gatewayv1alpha1.APISchemaOpenAI, Prefix: "/v1"},
		},
		Status: gatewayv1alpha1.AIBackendStatus{Conditions: []metav1.Condition{{Type: "Accepted"}}},
	}
	want := &gatewayv1alpha1.AIBackend{Spec: gatewayv1alpha1.AIBackendSpec{
		BackendRef: gatewayv1alpha1.BackendReference{Name: "provider"},
		Schema:     gatewayv1alpha1.APISchema{Name: gatewayv1alpha1.APISchemaOpenAI, Prefix: "/v1"},
	}}

	testResourceProjection(t, resource, projectAIBackend, want, func(backend *gatewayv1alpha1.AIBackend) {
		backend.Spec.BackendRef.Name = "other"
		backend.Spec.Schema.Prefix = "/models/v1"
	})
}

func TestProjectModelRoute(t *testing.T) {
	resource := &gatewayv1alpha1.ModelRoute{
		Spec: gatewayv1alpha1.ModelRouteSpec{
			DisplayName: "编程模型",
			ParentRefs:  []gatewayv1alpha1.ParentReference{{Name: "edge"}},
			Rules: []gatewayv1alpha1.ModelRouteRule{{
				Matches: []gatewayv1alpha1.ModelRouteMatch{{Headers: []gatewayv1alpha1.HTTPHeaderMatch{{
					Name: "x-ai-eg-model", Type: gatewayv1alpha1.HeaderMatchExact, Value: "coding",
				}}}},
				BackendRefs: []gatewayv1alpha1.AIBackendReference{{Name: "provider", ModelNameOverride: "code-v2"}},
			}},
		},
		Status: gatewayv1alpha1.ModelRouteStatus{Parents: []gatewayv1alpha1.RouteParentStatus{{ControllerName: controllerName}}},
	}
	want := &gatewayv1alpha1.ModelRoute{Spec: gatewayv1alpha1.ModelRouteSpec{
		ParentRefs: []gatewayv1alpha1.ParentReference{{Name: "edge"}},
		Rules: []gatewayv1alpha1.ModelRouteRule{{
			Matches: []gatewayv1alpha1.ModelRouteMatch{{Headers: []gatewayv1alpha1.HTTPHeaderMatch{{
				Name: "x-ai-eg-model", Type: gatewayv1alpha1.HeaderMatchExact, Value: "coding",
			}}}},
			BackendRefs: []gatewayv1alpha1.AIBackendReference{{Name: "provider", ModelNameOverride: "code-v2"}},
		}},
	}}

	testResourceProjection(t, resource, projectModelRoute, want, func(route *gatewayv1alpha1.ModelRoute) {
		route.Spec.ParentRefs[0].Name = "other"
		route.Spec.Rules[0].Matches[0].Headers[0].Value = "other"
		route.Spec.Rules[0].BackendRefs[0].ModelNameOverride = "other"
	})
}

func TestProjectMCPRoute(t *testing.T) {
	resource := &gatewayv1alpha1.MCPRoute{
		Spec: gatewayv1alpha1.MCPRouteSpec{
			DisplayName: "工具入口",
			ParentRefs:  []gatewayv1alpha1.ParentReference{{Name: "edge"}},
			Path:        new("/tools"),
			BackendRefs: []gatewayv1alpha1.MCPBackendReference{{Name: "tools", Path: new("/mcp")}},
		},
		Status: gatewayv1alpha1.MCPRouteStatus{Parents: []gatewayv1alpha1.RouteParentStatus{{ControllerName: controllerName}}},
	}
	want := &gatewayv1alpha1.MCPRoute{Spec: gatewayv1alpha1.MCPRouteSpec{
		ParentRefs:  []gatewayv1alpha1.ParentReference{{Name: "edge"}},
		Path:        new("/tools"),
		BackendRefs: []gatewayv1alpha1.MCPBackendReference{{Name: "tools", Path: new("/mcp")}},
	}}

	testResourceProjection(t, resource, projectMCPRoute, want, func(route *gatewayv1alpha1.MCPRoute) {
		route.Spec.ParentRefs[0].Name = "other"
		*route.Spec.Path = "/other"
		*route.Spec.BackendRefs[0].Path = "/other"
	})
}

func TestProjectResourceMetaChanges(t *testing.T) {
	tests := []struct {
		name   string
		change func(*gatewayv1alpha1.Gateway)
		want   bool
	}{
		{name: "resource version", change: func(resource *gatewayv1alpha1.Gateway) { resource.ResourceVersion = "2" }},
		{name: "labels", change: func(resource *gatewayv1alpha1.Gateway) { resource.Labels = map[string]string{"team": "b"} }},
		{name: "annotations", change: func(resource *gatewayv1alpha1.Gateway) { resource.Annotations = map[string]string{"note": "changed"} }},
		{name: "finalizers", change: func(resource *gatewayv1alpha1.Gateway) { resource.Finalizers = []string{"example.com/cleanup"} }},
		{name: "generation", change: func(resource *gatewayv1alpha1.Gateway) { resource.Generation++ }, want: true},
		{name: "identity", change: func(resource *gatewayv1alpha1.Gateway) { resource.UID = "replacement" }, want: true},
		{name: "creation time", change: func(resource *gatewayv1alpha1.Gateway) { resource.CreationTimestamp = metav1.Unix(2, 0) }, want: true},
		{name: "deletion starts", change: func(resource *gatewayv1alpha1.Gateway) { resource.DeletionTimestamp = new(metav1.Unix(3, 0)) }, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := &gatewayv1alpha1.Gateway{ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "edge", Generation: 1}}
			current := previous.DeepCopy()
			tt.change(current)

			got := !cmp.Equal(projectGateway(previous), projectGateway(current))

			if got != tt.want {
				t.Errorf("projectGateway(%s) changed = %t, want %t", tt.name, got, tt.want)
			}
		})
	}
}

func testResourceProjection[T client.Object](
	t *testing.T,
	resource T,
	project func(T) T,
	want T,
	changeSpec func(T),
) {
	t.Helper()
	resource.SetNamespace("team-a")
	resource.SetName("current")
	resource.SetUID("instance")
	resource.SetGeneration(3)
	resource.SetCreationTimestamp(metav1.Unix(1, 0))
	resource.SetDeletionTimestamp(new(metav1.Unix(2, 0)))
	resource.SetResourceVersion("8")
	resource.SetLabels(map[string]string{"team": "a"})
	resource.SetAnnotations(map[string]string{"note": "display only"})
	resource.SetFinalizers([]string{"example.com/cleanup"})
	before := resource.DeepCopyObject()
	want.SetNamespace("team-a")
	want.SetName("current")
	want.SetUID("instance")
	want.SetGeneration(3)
	want.SetCreationTimestamp(metav1.Unix(1, 0))
	want.SetDeletionTimestamp(new(metav1.Unix(2, 0)))

	got := project(resource)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("runtime projection mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(before, resource); diff != "" {
		t.Errorf("source changed during projection (-want +got):\n%s", diff)
	}

	changeSpec(got)
	got.GetDeletionTimestamp().Time = metav1.Unix(4, 0).Time

	if diff := cmp.Diff(before, resource); diff != "" {
		t.Errorf("source shares mutable projection fields (-want +got):\n%s", diff)
	}

	changed := resource.DeepCopyObject().(T)
	changeSpec(changed)
	if cmp.Equal(project(resource), project(changed)) {
		t.Error("runtime projection after spec change = previous projection, want different")
	}
}
