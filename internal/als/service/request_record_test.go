package service

import (
	"testing"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"

	aiprotocol "github.com/lgc202/ingate/internal/pkg/aiextproc"
)

func TestParseRequestRecord(t *testing.T) {
	entry := validAccessLogMessage().GetHttpLogs().GetLogEntry()[0]
	entry.Request.Authority = "example.com:8443"
	entry.Request.Path = "/health?token=secret"

	first, err := parseRequestRecord("envoy-1", entry)
	if err != nil {
		t.Fatalf("parseRequestRecord() error = %v, want nil", err)
	}
	second, err := parseRequestRecord("envoy-1", entry)
	if err != nil {
		t.Fatalf("parseRequestRecord() error = %v, want nil", err)
	}

	for _, value := range []string{first.GetId(), second.GetId()} {
		id, err := uuid.Parse(value)
		if err != nil {
			t.Errorf("uuid.Parse(parseRequestRecord() ID %q) error = %v, want nil", value, err)
			continue
		}
		if id.Version() != uuid.Version(4) || id.String() != value {
			t.Errorf("parseRequestRecord() ID = %q, want a canonical UUIDv4", value)
		}
	}
	if first.GetId() == second.GetId() {
		t.Errorf("two parseRequestRecord() calls returned ID %q, want distinct IDs", first.GetId())
	}
	if first.GetHost() != "example.com" || first.GetPath() != "/health" {
		t.Errorf("parseRequestRecord() request target = (%q, %q), want (%q, %q)",
			first.GetHost(), first.GetPath(), "example.com", "/health")
	}
}

func TestAIModelCallPresence(t *testing.T) {
	requestMetadata := map[string]*structpb.Value{
		aiprotocol.ClientHostField: structpb.NewStringValue("example.com"),
		aiprotocol.ClientPathField: structpb.NewStringValue("/chat"),
	}
	if call := aiModelCall(requestMetadata); call != nil {
		t.Errorf("aiModelCall(request metadata) = %v, want nil", call)
	}

	usageMetadata := map[string]*structpb.Value{
		aiprotocol.InputTokensField: structpb.NewNumberValue(0),
	}
	call := aiModelCall(usageMetadata)
	if call == nil || call.InputTokens == nil || call.GetInputTokens() != 0 {
		t.Errorf("aiModelCall(explicit zero usage) = %v, want an explicit zero input token count", call)
	}
}
