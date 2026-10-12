package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestMCPRouteDefaultingPreservesExplicitPaths(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v, want nil", err)
	}
	for _, tt := range []struct {
		name  string
		input *string
		want  string
	}{
		{name: "omitted", want: "/mcp"},
		{name: "explicit empty", input: ptr.To(""), want: ""},
		{name: "root", input: ptr.To("/"), want: "/"},
		{name: "trailing slash", input: ptr.To("/tools/mcp/"), want: "/tools/mcp/"},
		{name: "invalid relative path", input: ptr.To("mcp"), want: "mcp"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			route := &MCPRoute{Spec: MCPRouteSpec{
				DisplayName: "工具入口",
				ParentRefs:  []ParentReference{{Name: "edge", SectionName: "http"}},
				Path:        tt.input,
				BackendRefs: []MCPBackendReference{{Name: "tool-service", Path: tt.input}},
			}}
			want := route.DeepCopy()
			want.Spec.Path = ptr.To(tt.want)
			want.Spec.BackendRefs[0].Path = ptr.To(tt.want)
			list := &MCPRouteList{Items: []MCPRoute{*route.DeepCopy()}}
			scheme.Default(route)
			if diff := cmp.Diff(want, route); diff != "" {
				t.Errorf("Scheme.Default(MCPRoute %s) mismatch (-want +got):\n%s", tt.name, diff)
			}
			scheme.Default(list)
			if diff := cmp.Diff(*want, list.Items[0]); diff != "" {
				t.Errorf("Scheme.Default(MCPRouteList %s) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestMCPRouteDefaultingTreatsPathsIndependently(t *testing.T) {
	for _, tt := range []struct {
		name         string
		entryPath    *string
		upstreamPath *string
		wantEntry    string
		wantUpstream string
	}{
		{
			name:         "only upstream omitted",
			entryPath:    ptr.To("/tools"),
			wantEntry:    "/tools",
			wantUpstream: "/mcp",
		},
		{
			name:         "only entry omitted",
			upstreamPath: ptr.To("/server/"),
			wantEntry:    "/mcp",
			wantUpstream: "/server/",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			route := &MCPRoute{Spec: MCPRouteSpec{
				Path:        tt.entryPath,
				BackendRefs: []MCPBackendReference{{Name: "tool-service", Path: tt.upstreamPath}},
			}}
			SetDefaultsMCPRoute(route)
			want := MCPRouteSpec{
				Path:        ptr.To(tt.wantEntry),
				BackendRefs: []MCPBackendReference{{Name: "tool-service", Path: ptr.To(tt.wantUpstream)}},
			}
			if diff := cmp.Diff(want, route.Spec); diff != "" {
				t.Errorf("SetDefaultsMCPRoute(%s) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestMCPRouteConversion(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := gateway.AddToScheme(scheme); err != nil {
		t.Fatalf("gateway.AddToScheme() error = %v, want nil", err)
	}
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v, want nil", err)
	}
	input := &MCPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "tools", ResourceVersion: "12", Generation: 3},
		Spec: MCPRouteSpec{
			DisplayName: "工具入口",
			ParentRefs:  []ParentReference{{Name: "edge", SectionName: "http"}},
			Path:        ptr.To("/tools/mcp/"),
			BackendRefs: []MCPBackendReference{{Name: "tool-service", Path: ptr.To("/server/mcp")}},
		},
		Status: MCPRouteStatus{Parents: []RouteParentStatus{{
			ParentRef:      ParentReference{Name: "edge", SectionName: "http"},
			ControllerName: "gateway.ingate.io/test",
			Conditions:     []metav1.Condition{{Type: "ResolvedRefs", ObservedGeneration: 3}},
		}}},
	}
	var internal gateway.MCPRoute
	if err := scheme.Convert(input, &internal, nil); err != nil {
		t.Fatalf("Scheme.Convert(MCPRoute, internal) error = %v, want nil", err)
	}
	wantSpec := gateway.MCPRouteSpec{
		DisplayName: "工具入口",
		ParentRefs:  []gateway.ParentReference{{Name: "edge", SectionName: "http"}},
		Path:        ptr.To("/tools/mcp/"),
		BackendRefs: []gateway.MCPBackendReference{{Name: "tool-service", Path: ptr.To("/server/mcp")}},
	}
	if diff := cmp.Diff(wantSpec, internal.Spec); diff != "" {
		t.Errorf("Scheme.Convert(MCPRoute, internal).Spec mismatch (-want +got):\n%s", diff)
	}
	var roundTrip MCPRoute
	if err := scheme.Convert(&internal, &roundTrip, nil); err != nil {
		t.Fatalf("Scheme.Convert(internal MCPRoute, external) error = %v, want nil", err)
	}
	if diff := cmp.Diff(input, &roundTrip); diff != "" {
		t.Errorf("MCPRoute conversion round trip mismatch (-want +got):\n%s", diff)
	}
}
