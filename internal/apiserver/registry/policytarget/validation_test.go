package policytarget

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

func TestValidateRefs(t *testing.T) {
	tests := []struct {
		name    string
		refs    []resource.PolicyTargetRef
		wantErr bool
	}{
		{name: "empty"},
		{
			name: "gateway and route",
			refs: []resource.PolicyTargetRef{
				{Kind: resource.KindGateway, Name: "gateway-a"},
				{Kind: resource.KindRoute, Name: "route-a"},
			},
		},
		{
			name:    "unsupported kind",
			refs:    []resource.PolicyTargetRef{{Kind: resource.KindUpstream, Name: "upstream-a"}},
			wantErr: true,
		},
		{
			name:    "missing name",
			refs:    []resource.PolicyTargetRef{{Kind: resource.KindGateway}},
			wantErr: true,
		},
		{
			name: "duplicate",
			refs: []resource.PolicyTargetRef{
				{Kind: resource.KindRoute, Name: "route-a"},
				{Kind: resource.KindRoute, Name: "route-a"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateRefs(tt.refs, field.NewPath("spec", "targetRefs"))
			if gotErr := len(errs) > 0; gotErr != tt.wantErr {
				t.Errorf("ValidateRefs(%v) errors = %v, want error presence = %t", tt.refs, errs, tt.wantErr)
			}
		})
	}
}
