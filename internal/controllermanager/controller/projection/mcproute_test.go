package projection

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestMCPRouteToMySQLPreservesConfig(t *testing.T) {
	createdAt := time.Date(2026, 10, 12, 9, 2, 3, 0, time.FixedZone("UTC+8", 8*60*60))
	route := &gatewayv1alpha1.MCPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         "team-a",
			Name:              "tools",
			ResourceVersion:   "9",
			CreationTimestamp: metav1.NewTime(createdAt),
		},
		Spec: gatewayv1alpha1.MCPRouteSpec{
			DisplayName: "工具路由",
			ParentRefs: []gatewayv1alpha1.ParentReference{
				{Name: "edge", SectionName: "z"},
				{Name: "edge", SectionName: "a"},
			},
			Path:        ptr.To("/工具/%2F/"),
			BackendRefs: []gatewayv1alpha1.MCPBackendReference{{Name: "network", Path: ptr.To("/")}},
		},
	}
	unchanged := route.DeepCopy()
	got := mcpRouteToMySQL(route)
	want := mysql.MCPRoute{
		Namespace:   "team-a",
		Name:        "tools",
		DisplayName: "工具路由",
		Version:     "9",
		CreatedAt:   createdAt.UTC(),
		Parents: []mysql.MCPRouteParent{
			{Name: "edge", SectionName: "z"},
			{Name: "edge", SectionName: "a"},
		},
		Path:        "/工具/%2F/",
		BackendName: "network",
		BackendPath: "/",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mcpRouteToMySQL(config) mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(unchanged, route); diff != "" {
		t.Errorf("mcpRouteToMySQL changed API object (-want +got):\n%s", diff)
	}
}
