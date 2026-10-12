package validation

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestValidateMCPRoute(t *testing.T) {
	for _, tt := range []struct {
		name      string
		mutate    func(*gateway.MCPRoute)
		wantField string
		wantType  field.ErrorType
	}{
		{name: "unresolved references", mutate: func(*gateway.MCPRoute) {}},
		{
			name:      "namespace",
			mutate:    func(r *gateway.MCPRoute) { r.Namespace = "default" },
			wantField: "metadata.namespace",
			wantType:  field.ErrorTypeForbidden,
		},
		{
			name:      "no parent",
			mutate:    func(r *gateway.MCPRoute) { r.Spec.ParentRefs = nil },
			wantField: "spec.parentRefs",
			wantType:  field.ErrorTypeRequired,
		},
		{
			name:      "invalid parent",
			mutate:    func(r *gateway.MCPRoute) { r.Spec.ParentRefs[0].Name = "Invalid" },
			wantField: "spec.parentRefs[0].name",
			wantType:  field.ErrorTypeInvalid,
		},
		{
			name:      "invalid section",
			mutate:    func(r *gateway.MCPRoute) { r.Spec.ParentRefs[0].SectionName = "Invalid" },
			wantField: "spec.parentRefs[0].sectionName",
			wantType:  field.ErrorTypeInvalid,
		},
		{
			name:      "duplicate parent",
			mutate:    func(r *gateway.MCPRoute) { r.Spec.ParentRefs = append(r.Spec.ParentRefs, r.Spec.ParentRefs[0]) },
			wantField: "spec.parentRefs[1]",
			wantType:  field.ErrorTypeDuplicate,
		},
		{
			name: "whole gateway and section overlap",
			mutate: func(r *gateway.MCPRoute) {
				r.Spec.ParentRefs = append(r.Spec.ParentRefs, gateway.ParentReference{Name: "missing-gateway", SectionName: "http"})
			},
			wantField: "spec.parentRefs[1]",
			wantType:  field.ErrorTypeInvalid,
		},
		{
			name:      "no Backend",
			mutate:    func(r *gateway.MCPRoute) { r.Spec.BackendRefs = nil },
			wantField: "spec.backendRefs",
			wantType:  field.ErrorTypeInvalid,
		},
		{
			name: "multiple Backends",
			mutate: func(r *gateway.MCPRoute) {
				r.Spec.BackendRefs = append(r.Spec.BackendRefs, gateway.MCPBackendReference{Name: "another"})
			},
			wantField: "spec.backendRefs",
			wantType:  field.ErrorTypeInvalid,
		},
		{
			name:      "empty Backend name",
			mutate:    func(r *gateway.MCPRoute) { r.Spec.BackendRefs[0].Name = "" },
			wantField: "spec.backendRefs[0].name",
			wantType:  field.ErrorTypeRequired,
		},
		{
			name:      "invalid Backend name",
			mutate:    func(r *gateway.MCPRoute) { r.Spec.BackendRefs[0].Name = "Invalid" },
			wantField: "spec.backendRefs[0].name",
			wantType:  field.ErrorTypeInvalid,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			route := validMCPRoute()
			tt.mutate(route)
			before := route.DeepCopy()
			errs := ValidateMCPRoute(route)
			if diff := cmp.Diff(before, route); diff != "" {
				t.Errorf("ValidateMCPRoute(%s) changed input (-want +got):\n%s", tt.name, diff)
			}
			if tt.wantField == "" && len(errs) != 0 {
				t.Errorf("ValidateMCPRoute(%s) = %v, want no errors", tt.name, errs)
			}
			if tt.wantField != "" && (len(errs) != 1 || errs[0].Field != tt.wantField || errs[0].Type != tt.wantType) {
				t.Errorf("ValidateMCPRoute(%s) = %v, want one %s error at %s", tt.name, errs, tt.wantType, tt.wantField)
			}
		})
	}
}

func TestValidateMCPRoutePaths(t *testing.T) {
	for _, tt := range []struct {
		name     string
		value    *string
		wantType field.ErrorType
	}{
		{name: "omitted"},
		{name: "empty", value: ptr.To(""), wantType: field.ErrorTypeRequired},
		{name: "root", value: ptr.To("/")},
		{name: "trailing slash", value: ptr.To("/tools/mcp/")},
		{name: "encoded path", value: ptr.To("/tools/%E5%B7%A5%E5%85%B7")},
		{name: "internal double slash", value: ptr.To("/tools//mcp")},
		{name: "dot segment", value: ptr.To("/tools/../mcp")},
		{name: "maximum ASCII length", value: ptr.To("/" + strings.Repeat("a", 1023))},
		{name: "excess ASCII length", value: ptr.To("/" + strings.Repeat("a", 1024)), wantType: field.ErrorTypeTooLong},
		{name: "maximum Unicode length", value: ptr.To("/" + strings.Repeat("工", 1023))},
		{name: "excess Unicode length", value: ptr.To("/" + strings.Repeat("工", 1024)), wantType: field.ErrorTypeTooLong},
		{name: "relative", value: ptr.To("mcp"), wantType: field.ErrorTypeInvalid},
		{name: "URL", value: ptr.To("https://example.com/mcp"), wantType: field.ErrorTypeInvalid},
		{name: "authority", value: ptr.To("//example.com/mcp"), wantType: field.ErrorTypeInvalid},
		{name: "query", value: ptr.To("/mcp?token=x"), wantType: field.ErrorTypeInvalid},
		{name: "empty query", value: ptr.To("/mcp?"), wantType: field.ErrorTypeInvalid},
		{name: "fragment", value: ptr.To("/mcp#part"), wantType: field.ErrorTypeInvalid},
		{name: "empty fragment", value: ptr.To("/mcp#"), wantType: field.ErrorTypeInvalid},
		{name: "invalid escape", value: ptr.To("/mcp%ZZ"), wantType: field.ErrorTypeInvalid},
		{name: "control character", value: ptr.To("/mcp\n"), wantType: field.ErrorTypeInvalid},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, target := range []string{"spec.path", "spec.backendRefs[0].path"} {
				route := validMCPRoute()
				if target == "spec.path" {
					route.Spec.Path = tt.value
				} else {
					route.Spec.BackendRefs[0].Path = tt.value
				}
				errs := ValidateMCPRoute(route)
				if tt.wantType == "" && len(errs) != 0 {
					t.Errorf("ValidateMCPRoute(%s at %s) = %v, want no errors", tt.name, target, errs)
				}
				if tt.wantType != "" && (len(errs) != 1 || errs[0].Field != target || errs[0].Type != tt.wantType) {
					t.Errorf("ValidateMCPRoute(%s at %s) = %v, want one %s error", tt.name, target, errs, tt.wantType)
				}
			}
		})
	}
}

func TestMCPRouteParentLimit(t *testing.T) {
	route := validMCPRoute()
	route.Spec.ParentRefs = nil
	for i := range 16 {
		route.Spec.ParentRefs = append(route.Spec.ParentRefs, gateway.ParentReference{Name: fmt.Sprintf("gateway-%d", i)})
	}
	if errs := ValidateMCPRoute(route); len(errs) != 0 {
		t.Errorf("ValidateMCPRoute(16 parents) = %v, want no errors", errs)
	}
	route.Spec.ParentRefs = append(route.Spec.ParentRefs, gateway.ParentReference{Name: "gateway-16"})
	errs := ValidateMCPRoute(route)
	if len(errs) != 1 || errs[0].Field != "spec.parentRefs" || errs[0].Type != field.ErrorTypeTooMany {
		t.Errorf("ValidateMCPRoute(17 parents) = %v, want one TooMany error at spec.parentRefs", errs)
	}
}

func TestValidateMCPRouteUpdate(t *testing.T) {
	previous := validMCPRoute()
	previous.ResourceVersion = "1"
	next := previous.DeepCopy()
	next.Spec.DisplayName = "更新展示名称"
	next.Spec.Path = ptr.To("/tools/mcp/")
	if errs := ValidateMCPRouteUpdate(next, previous); len(errs) != 0 {
		t.Errorf("ValidateMCPRouteUpdate(changed spec) = %v, want no errors", errs)
	}
	next.Name = "renamed"
	errs := ValidateMCPRouteUpdate(next, previous)
	if len(errs) != 1 || errs[0].Field != "metadata.name" || errs[0].Type != field.ErrorTypeInvalid {
		t.Errorf("ValidateMCPRouteUpdate(renamed) = %v, want one Invalid error at metadata.name", errs)
	}
}

func TestMCPRouteStatusRejectsDuplicateParents(t *testing.T) {
	previous := validMCPRoute()
	previous.ResourceVersion = "1"
	next := previous.DeepCopy()
	parent := gateway.RouteParentStatus{
		ParentRef:      gateway.ParentReference{Name: "missing-gateway"},
		ControllerName: "gateway.ingate.io/test",
	}
	next.Status.Parents = []gateway.RouteParentStatus{parent, parent}
	errs := ValidateMCPRouteStatusUpdate(next, previous)
	if len(errs) != 1 || errs[0].Field != "status.parents[1]" || errs[0].Type != field.ErrorTypeDuplicate {
		t.Errorf("ValidateMCPRouteStatusUpdate(duplicate parents) = %v, want one Duplicate error at status.parents[1]", errs)
	}
}

func validMCPRoute() *gateway.MCPRoute {
	return &gateway.MCPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "tools"},
		Spec: gateway.MCPRouteSpec{
			DisplayName: "工具入口",
			ParentRefs:  []gateway.ParentReference{{Name: "missing-gateway"}},
			BackendRefs: []gateway.MCPBackendReference{{Name: "missing-backend"}},
		},
	}
}
