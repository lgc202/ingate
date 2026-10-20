package aiconfig

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
)

func TestResourceValidationRejectsMalformedFields(t *testing.T) {
	_, rule := testResources()
	for _, tt := range []struct {
		name   string
		change func(*aidataplanev1.Resource)
	}{
		{name: "missing payload", change: func(r *aidataplanev1.Resource) { r.Payload = nil }},
		{name: "invalid port", change: func(r *aidataplanev1.Resource) { r.GetModelRule().EntryRefs[0].Port = 0 }},
		{name: "missing origin", change: func(r *aidataplanev1.Resource) { r.GetModelRule().Origin = nil }},
		{name: "invalid time", change: func(r *aidataplanev1.Resource) {
			r.GetModelRule().Origin.CreationTime = &timestamppb.Timestamp{Seconds: 1 << 60}
		}},
		{name: "duplicate refs", change: func(r *aidataplanev1.Resource) {
			r.GetModelRule().EntryRefs = append(r.GetModelRule().EntryRefs, r.GetModelRule().EntryRefs[0])
		}},
		{name: "empty exact", change: func(r *aidataplanev1.Resource) {
			r.GetModelRule().Matches[0].Condition = &aidataplanev1.ModelMatch_Exact{}
		}},
		{name: "missing outcome", change: func(r *aidataplanev1.Resource) { r.GetModelRule().Outcome = nil }},
		{name: "unknown schema", change: func(r *aidataplanev1.Resource) { r.GetModelRule().GetTarget().Schema.Name = 99 }},
		{name: "authority prefix", change: func(r *aidataplanev1.Resource) { r.GetModelRule().GetTarget().Schema.Prefix = "//other.example/path" }},
		{name: "query prefix", change: func(r *aidataplanev1.Resource) { r.GetModelRule().GetTarget().Schema.Prefix = "/v1?key=value" }},
		{name: "malformed escaping", change: func(r *aidataplanev1.Resource) { r.GetModelRule().GetTarget().Schema.Prefix = "/%zz" }},
		{name: "invalid target header", change: func(r *aidataplanev1.Resource) { r.GetModelRule().GetTarget().TargetName = "target\r\nInjected: value" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resource := proto.CloneOf(rule)
			tt.change(resource)

			if err := validateResource(resource); err == nil {
				t.Errorf("validateResource(%s) error = nil, want rejection", tt.name)
			}
		})
	}
}

func TestResourceBatchRejectsAmbiguousIdentityAndReusedVersion(t *testing.T) {
	for _, kind := range []string{"entry identity", "rule identity", "version"} {
		t.Run(kind, func(t *testing.T) {
			entry, rule := testResources()
			state, err := new(resourceState).apply(testBatch(t, entry, rule))
			if err != nil {
				t.Fatal(err)
			}

			duplicate := proto.CloneOf(rule)
			if kind == "entry identity" {
				duplicate = proto.CloneOf(entry)
			}

			duplicate.Name += "-duplicate"
			response := testBatch(t, duplicate)
			if kind == "version" {
				duplicate.Name = rule.Name
				duplicate.GetModelRule().GetTarget().ModelNameOverride = "changed"
				response = testBatch(t, duplicate)
				response.Resources[0].Version = state.resources[rule.Name].version
			}

			candidate, err := state.apply(response)
			if err == nil || candidate != nil {
				t.Errorf("apply(%s) = (%v, %v), want no candidate and rejection", kind, candidate, err)
			}
		})
	}
}
