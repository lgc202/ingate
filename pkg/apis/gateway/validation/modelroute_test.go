package validation

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestValidateModelRoute(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*gateway.ModelRoute)
		wantField string
		wantType  field.ErrorType
	}{
		{name: "unresolved references", mutate: func(*gateway.ModelRoute) {}},
		{name: "omitted matches", mutate: func(r *gateway.ModelRoute) { r.Spec.Rules[0].Matches = nil }},
		{name: "empty matches", mutate: func(r *gateway.ModelRoute) { r.Spec.Rules[0].Matches = []gateway.ModelRouteMatch{} }},
		{name: "empty match", mutate: func(r *gateway.ModelRoute) { r.Spec.Rules[0].Matches = []gateway.ModelRouteMatch{{}} }},
		{
			name:   "header name case",
			mutate: func(r *gateway.ModelRoute) { r.Spec.Rules[0].Matches[0].Headers[0].Name = "X-AI-EG-MODEL" },
		},
		{
			name:   "default match type",
			mutate: func(r *gateway.ModelRoute) { r.Spec.Rules[0].Matches[0].Headers[0].Type = "" },
		},
		{
			name:   "unmodified model value",
			mutate: func(r *gateway.ModelRoute) { r.Spec.Rules[0].Matches[0].Headers[0].Value = " Coding " },
		},
		{
			name:      "namespace",
			mutate:    func(r *gateway.ModelRoute) { r.Namespace = "default" },
			wantField: "metadata.namespace",
			wantType:  field.ErrorTypeForbidden,
		},
		{
			name:      "no parent",
			mutate:    func(r *gateway.ModelRoute) { r.Spec.ParentRefs = nil },
			wantField: "spec.parentRefs",
			wantType:  field.ErrorTypeRequired,
		},
		{
			name:      "invalid parent",
			mutate:    func(r *gateway.ModelRoute) { r.Spec.ParentRefs[0].Name = "Invalid" },
			wantField: "spec.parentRefs[0].name",
			wantType:  field.ErrorTypeInvalid,
		},
		{
			name:      "duplicate parent",
			mutate:    func(r *gateway.ModelRoute) { r.Spec.ParentRefs = append(r.Spec.ParentRefs, r.Spec.ParentRefs[0]) },
			wantField: "spec.parentRefs[1]",
			wantType:  field.ErrorTypeDuplicate,
		},
		{
			name: "whole gateway and section overlap",
			mutate: func(r *gateway.ModelRoute) {
				r.Spec.ParentRefs = append(r.Spec.ParentRefs, gateway.ParentReference{Name: "missing-gateway", SectionName: "http"})
			},
			wantField: "spec.parentRefs[1]",
			wantType:  field.ErrorTypeInvalid,
		},
		{
			name:      "no rules",
			mutate:    func(r *gateway.ModelRoute) { r.Spec.Rules = nil },
			wantField: "spec.rules",
			wantType:  field.ErrorTypeRequired,
		},
		{
			name:      "invalid rule name",
			mutate:    func(r *gateway.ModelRoute) { r.Spec.Rules[0].Name = "Invalid" },
			wantField: "spec.rules[0].name",
			wantType:  field.ErrorTypeInvalid,
		},
		{
			name:      "duplicate rule name",
			mutate:    func(r *gateway.ModelRoute) { r.Spec.Rules = append(r.Spec.Rules, r.Spec.Rules[0]) },
			wantField: "spec.rules[1].name",
			wantType:  field.ErrorTypeDuplicate,
		},
		{
			name:      "no AIBackend",
			mutate:    func(r *gateway.ModelRoute) { r.Spec.Rules[0].BackendRefs = nil },
			wantField: "spec.rules[0].backendRefs",
			wantType:  field.ErrorTypeInvalid,
		},
		{
			name: "multiple AIBackends",
			mutate: func(r *gateway.ModelRoute) {
				r.Spec.Rules[0].BackendRefs = append(r.Spec.Rules[0].BackendRefs, gateway.AIBackendReference{Name: "another-service"})
			},
			wantField: "spec.rules[0].backendRefs",
			wantType:  field.ErrorTypeInvalid,
		},
		{
			name:      "invalid AIBackend name",
			mutate:    func(r *gateway.ModelRoute) { r.Spec.Rules[0].BackendRefs[0].Name = "Invalid" },
			wantField: "spec.rules[0].backendRefs[0].name",
			wantType:  field.ErrorTypeInvalid,
		},
		{
			name: "multiple headers",
			mutate: func(r *gateway.ModelRoute) {
				match := &r.Spec.Rules[0].Matches[0]
				match.Headers = append(match.Headers, match.Headers[0])
			},
			wantField: "spec.rules[0].matches[0].headers",
			wantType:  field.ErrorTypeTooMany,
		},
		{
			name:      "empty header name",
			mutate:    func(r *gateway.ModelRoute) { r.Spec.Rules[0].Matches[0].Headers[0].Name = "" },
			wantField: "spec.rules[0].matches[0].headers[0].name",
			wantType:  field.ErrorTypeRequired,
		},
		{
			name:      "unsupported header",
			mutate:    func(r *gateway.ModelRoute) { r.Spec.Rules[0].Matches[0].Headers[0].Name = "Authorization" },
			wantField: "spec.rules[0].matches[0].headers[0].name",
			wantType:  field.ErrorTypeNotSupported,
		},
		{
			name:      "regex match",
			mutate:    func(r *gateway.ModelRoute) { r.Spec.Rules[0].Matches[0].Headers[0].Type = "RegularExpression" },
			wantField: "spec.rules[0].matches[0].headers[0].type",
			wantType:  field.ErrorTypeNotSupported,
		},
		{
			name:      "empty model",
			mutate:    func(r *gateway.ModelRoute) { r.Spec.Rules[0].Matches[0].Headers[0].Value = "" },
			wantField: "spec.rules[0].matches[0].headers[0].value",
			wantType:  field.ErrorTypeRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := validModelRoute()
			tt.mutate(route)
			before := route.DeepCopy()
			errs := ValidateModelRoute(route)
			if diff := cmp.Diff(before, route); diff != "" {
				t.Errorf("ValidateModelRoute(%s) changed input (-want +got):\n%s", tt.name, diff)
			}
			if tt.wantField == "" {
				if len(errs) != 0 {
					t.Errorf("ValidateModelRoute(%s) = %v, want no errors", tt.name, errs)
				}
				return
			}
			if len(errs) != 1 || errs[0].Field != tt.wantField || errs[0].Type != tt.wantType {
				t.Errorf("ValidateModelRoute(%s) = %v, want one %s error at %s", tt.name, errs, tt.wantType, tt.wantField)
			}
		})
	}
}

func TestValidateModelRouteCollectionLimits(t *testing.T) {
	for _, tt := range []struct {
		name   string
		limit  int
		mutate func(*gateway.ModelRoute, int)
		field  string
	}{
		{
			name:  "parent references",
			limit: 16,
			mutate: func(r *gateway.ModelRoute, count int) {
				r.Spec.ParentRefs = make([]gateway.ParentReference, count)
				for i := range r.Spec.ParentRefs {
					r.Spec.ParentRefs[i].Name = fmt.Sprintf("gateway-%d", i)
				}
			},
			field: "spec.parentRefs",
		},
		{
			name:  "rules",
			limit: 15,
			mutate: func(r *gateway.ModelRoute, count int) {
				r.Spec.Rules = make([]gateway.ModelRouteRule, count)
				for i := range r.Spec.Rules {
					r.Spec.Rules[i].BackendRefs = []gateway.AIBackendReference{{Name: "missing-service"}}
				}
			},
			field: "spec.rules",
		},
		{
			name:  "matches",
			limit: 128,
			mutate: func(r *gateway.ModelRoute, count int) {
				r.Spec.Rules[0].Matches = make([]gateway.ModelRouteMatch, count)
			},
			field: "spec.rules[0].matches",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			route := validModelRoute()
			tt.mutate(route, tt.limit)
			if errs := ValidateModelRoute(route); len(errs) != 0 {
				t.Errorf("ValidateModelRoute(%s count %d) = %v, want no errors", tt.name, tt.limit, errs)
			}
			tt.mutate(route, tt.limit+1)
			errs := ValidateModelRoute(route)
			if len(errs) != 1 || errs[0].Field != tt.field || errs[0].Type != field.ErrorTypeTooMany {
				t.Errorf("ValidateModelRoute(%s count %d) = %v, want one TooMany error at %s", tt.name, tt.limit+1, errs, tt.field)
			}
		})
	}
}

func TestValidateModelRouteUpdate(t *testing.T) {
	previous := validModelRoute()
	previous.ResourceVersion = "1"
	next := previous.DeepCopy()
	next.Spec.DisplayName = "更新的展示名称"
	next.Spec.Rules[0].BackendRefs[0].ModelNameOverride = "Code-V2"
	if errs := ValidateModelRouteUpdate(next, previous); len(errs) != 0 {
		t.Errorf("ValidateModelRouteUpdate(changed spec) = %v, want no errors", errs)
	}
	next.Name = "renamed"
	errs := ValidateModelRouteUpdate(next, previous)
	if len(errs) != 1 || errs[0].Field != "metadata.name" || errs[0].Type != field.ErrorTypeInvalid {
		t.Errorf("ValidateModelRouteUpdate(renamed) = %v, want one Invalid error at metadata.name", errs)
	}
}

func TestRouteParentStatusValidationContract(t *testing.T) {
	for _, tt := range []struct {
		name      string
		mutate    func(*gateway.RouteParentStatus)
		wantField string
	}{
		{name: "valid", mutate: func(*gateway.RouteParentStatus) {}},
		{
			name:      "invalid parent",
			mutate:    func(p *gateway.RouteParentStatus) { p.ParentRef.Name = "Invalid" },
			wantField: "status.parents[0].parentRef.name",
		},
		{
			name:      "missing controller",
			mutate:    func(p *gateway.RouteParentStatus) { p.ControllerName = "" },
			wantField: "status.parents[0].controllerName",
		},
		{
			name:      "future generation",
			mutate:    func(p *gateway.RouteParentStatus) { p.Conditions[0].ObservedGeneration = 4 },
			wantField: "status.parents[0].conditions[0].observedGeneration",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			previous := validModelRoute()
			previous.ResourceVersion = "1"
			previous.Generation = 3
			next := previous.DeepCopy()
			parent := gateway.RouteParentStatus{
				ParentRef:      gateway.ParentReference{Name: "missing-gateway"},
				ControllerName: "gateway.ingate.io/test",
				Conditions: []metav1.Condition{{
					Type:               "ResolvedRefs",
					Status:             metav1.ConditionTrue,
					Reason:             "ResolvedRefs",
					Message:            "references resolved",
					ObservedGeneration: 3,
					LastTransitionTime: metav1.NewTime(time.Unix(1, 0)),
				}},
			}
			tt.mutate(&parent)
			next.Status.Parents = []gateway.RouteParentStatus{parent}
			errs := ValidateModelRouteStatusUpdate(next, previous)
			if tt.wantField == "" && len(errs) != 0 {
				t.Errorf("ValidateModelRouteStatusUpdate(%s) = %v, want no errors", tt.name, errs)
			}
			if tt.wantField != "" && (len(errs) != 1 || errs[0].Field != tt.wantField) {
				t.Errorf("ValidateModelRouteStatusUpdate(%s) = %v, want one error at %s", tt.name, errs, tt.wantField)
			}

			httpPrevious := &gateway.HTTPRoute{ObjectMeta: previous.ObjectMeta}
			httpNext := &gateway.HTTPRoute{
				ObjectMeta: next.ObjectMeta,
				Status:     gateway.HTTPRouteStatus{Parents: next.Status.Parents},
			}
			httpErrs := ValidateHTTPRouteStatusUpdate(httpNext, httpPrevious)
			if diff := cmp.Diff(httpErrs, errs); diff != "" {
				t.Errorf("route status validation(%s) differs (-HTTPRoute +ModelRoute):\n%s", tt.name, diff)
			}
			mcpPrevious := &gateway.MCPRoute{ObjectMeta: previous.ObjectMeta}
			mcpNext := &gateway.MCPRoute{
				ObjectMeta: next.ObjectMeta,
				Status:     gateway.MCPRouteStatus{Parents: next.Status.Parents},
			}
			mcpErrs := ValidateMCPRouteStatusUpdate(mcpNext, mcpPrevious)
			if diff := cmp.Diff(errs, mcpErrs); diff != "" {
				t.Errorf("route status validation(%s) differs (-ModelRoute +MCPRoute):\n%s", tt.name, diff)
			}
		})
	}
}

func TestModelRouteStatusRejectsDuplicateParents(t *testing.T) {
	previous := validModelRoute()
	previous.ResourceVersion = "1"
	next := previous.DeepCopy()
	parent := gateway.RouteParentStatus{
		ParentRef:      gateway.ParentReference{Name: "missing-gateway"},
		ControllerName: "gateway.ingate.io/test",
	}
	next.Status.Parents = []gateway.RouteParentStatus{parent, parent}
	errs := ValidateModelRouteStatusUpdate(next, previous)
	if len(errs) != 1 || errs[0].Field != "status.parents[1]" || errs[0].Type != field.ErrorTypeDuplicate {
		t.Errorf("ValidateModelRouteStatusUpdate(duplicate parents) = %v, want one Duplicate error at status.parents[1]", errs)
	}
}

func validModelRoute() *gateway.ModelRoute {
	return &gateway.ModelRoute{
		ObjectMeta: metav1.ObjectMeta{Name: "models"},
		Spec: gateway.ModelRouteSpec{
			DisplayName: "模型路由",
			ParentRefs:  []gateway.ParentReference{{Name: "missing-gateway"}},
			Rules: []gateway.ModelRouteRule{{
				Name: "coding",
				Matches: []gateway.ModelRouteMatch{{
					Headers: []gateway.HTTPHeaderMatch{{
						Name:  "x-ai-eg-model",
						Type:  gateway.HeaderMatchExact,
						Value: "coding",
					}},
				}},
				BackendRefs: []gateway.AIBackendReference{{Name: "missing-ai-backend"}},
			}},
		},
	}
}
