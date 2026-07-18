package dto

import (
	"testing"
)

func TestValidatePolicyTargets(t *testing.T) {
	tests := []struct {
		name    string
		targets []PolicyTargetReq
		wantErr bool
	}{
		{
			name: "empty targets",
		},
		{
			name: "gateway and route",
			targets: []PolicyTargetReq{
				{Kind: PolicyTargetKindGateway, ID: " gateway-1 "},
				{Kind: PolicyTargetKindRoute, ID: "route-1"},
			},
		},
		{
			name:    "missing id",
			targets: []PolicyTargetReq{{Kind: PolicyTargetKindGateway}},
			wantErr: true,
		},
		{
			name:    "unsupported kind",
			targets: []PolicyTargetReq{{Kind: "Upstream", ID: "upstream-1"}},
			wantErr: true,
		},
		{
			name: "duplicate target",
			targets: []PolicyTargetReq{
				{Kind: PolicyTargetKindRoute, ID: "route-1"},
				{Kind: PolicyTargetKindRoute, ID: "route-1"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePolicyTargets(tt.targets)
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Fatalf("ValidatePolicyTargets(%v) error = %v, want error presence = %t", tt.targets, err, tt.wantErr)
			}
			if !tt.wantErr && len(tt.targets) > 0 && tt.targets[0].ID != "gateway-1" {
				t.Errorf("ValidatePolicyTargets(%v) first id = %q, want %q", tt.targets, tt.targets[0].ID, "gateway-1")
			}
		})
	}
}
