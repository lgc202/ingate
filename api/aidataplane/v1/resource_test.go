package aidataplanev1_test

import (
	"testing"

	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
)

func TestResourcePayloadRoundTrip(t *testing.T) {
	resource := &aidataplanev1.Resource{
		Name: "entry/8080/gateway/team-a/public/listener/api",
		Payload: &aidataplanev1.Resource_Entry{Entry: &aidataplanev1.Entry{
			Port: 8080, HostName: "gateway/team-a/public/listener/api",
		}},
	}

	if err := protovalidate.Validate(resource); err != nil {
		t.Fatalf("Validate(entry) error = %v, want nil", err)
	}

	encoded, err := anypb.New(resource)
	if err != nil {
		t.Fatalf("anypb.New(entry) error = %v, want nil", err)
	}

	if encoded.TypeUrl != "type.googleapis.com/aidataplane.v1.Resource" {
		t.Errorf("anypb.New(entry).TypeUrl = %q, want aidataplane.v1.Resource URL", encoded.TypeUrl)
	}

	var decoded aidataplanev1.Resource
	if err := encoded.UnmarshalTo(&decoded); err != nil {
		t.Fatalf("UnmarshalTo(entry) error = %v, want nil", err)
	}

	if !proto.Equal(resource, &decoded) {
		t.Errorf("UnmarshalTo(entry) = %v, want %v", &decoded, resource)
	}
}

func TestResourceStructuralConstraints(t *testing.T) {
	ref := &aidataplanev1.EntryReference{Port: 8080, HostName: "gateway/team-a/public/listener/api"}
	origin := &aidataplanev1.Origin{Namespace: "team-a", Name: "coding", CreationTime: timestamppb.Now()}
	rule := &aidataplanev1.ModelRule{
		EntryRefs: []*aidataplanev1.EntryReference{ref}, Origin: origin,
		Matches: []*aidataplanev1.ModelMatch{{Condition: &aidataplanev1.ModelMatch_All{All: &emptypb.Empty{}}}},
		Outcome: &aidataplanev1.ModelRule_Unavailable{Unavailable: &emptypb.Empty{}},
	}
	mcp := &aidataplanev1.MCPRoute{
		EntryRefs: []*aidataplanev1.EntryReference{ref}, Origin: origin, Path: "/mcp",
		Outcome: &aidataplanev1.MCPRoute_Unavailable{Unavailable: &emptypb.Empty{}},
	}
	tests := []struct {
		name    string
		message proto.Message
		wantErr bool
	}{
		{name: "unavailable_catch_all", message: rule},
		{name: "unavailable_mcp", message: mcp},
		{name: "no_resource_payload", message: &aidataplanev1.Resource{Name: "missing"}, wantErr: true},
		{name: "empty_resource_name", message: &aidataplanev1.Resource{Payload: &aidataplanev1.Resource_ModelRule{ModelRule: rule}}, wantErr: true},
		{name: "invalid_port", message: &aidataplanev1.Entry{Port: 65536, HostName: "entry"}, wantErr: true},
		{name: "no_entry_refs", message: &aidataplanev1.ModelRule{Origin: origin, Matches: rule.Matches, Outcome: rule.Outcome}, wantErr: true},
		{name: "no_model_outcome", message: &aidataplanev1.ModelRule{Origin: origin, EntryRefs: rule.EntryRefs, Matches: rule.Matches}, wantErr: true},
		{name: "nil_model_target", message: &aidataplanev1.ModelRule{Origin: origin, EntryRefs: rule.EntryRefs, Matches: rule.Matches, Outcome: &aidataplanev1.ModelRule_Target{}}, wantErr: true},
		{name: "no_model_condition", message: &aidataplanev1.ModelMatch{}, wantErr: true},
		{name: "empty_exact_model", message: &aidataplanev1.ModelMatch{Condition: &aidataplanev1.ModelMatch_Exact{}}, wantErr: true},
		{name: "no_mcp_outcome", message: &aidataplanev1.MCPRoute{Origin: origin, EntryRefs: mcp.EntryRefs, Path: "/mcp"}, wantErr: true},
		{name: "relative_mcp_path", message: &aidataplanev1.MCPRoute{Origin: origin, EntryRefs: mcp.EntryRefs, Path: "mcp", Outcome: mcp.Outcome}, wantErr: true},
		{name: "unknown_schema", message: &aidataplanev1.APISchema{Name: 7, Prefix: "/v1"}, wantErr: true},
		{name: "unspecified_schema", message: &aidataplanev1.APISchema{Prefix: "/v1"}, wantErr: true},
		{name: "relative_schema_prefix", message: &aidataplanev1.APISchema{Name: aidataplanev1.APISchemaName_API_SCHEMA_NAME_OPENAI, Prefix: "v1"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := protovalidate.Validate(tt.message)

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(%v) error = %v, want error presence %t", tt.message, err, tt.wantErr)
			}
		})
	}
}
