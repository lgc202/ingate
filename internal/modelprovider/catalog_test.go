package modelprovider

import (
	"testing"

	"github.com/lgc202/ingate/pkg/llm"
)

func TestCatalog(t *testing.T) {
	definitions := Catalog()
	if len(definitions) != 6 {
		t.Fatalf("len(Catalog()) = %d, want 6", len(definitions))
	}
	anthropic, ok := Lookup(IDAnthropic)
	if !ok {
		t.Fatal("Lookup(IDAnthropic) did not find definition")
	}
	if anthropic.Protocol != llm.ProtocolAnthropicMessages || anthropic.Authentication.Header != "x-api-key" || anthropic.StaticHeaders["anthropic-version"] != "2023-06-01" {
		t.Errorf("Lookup(IDAnthropic) = %#v, want Anthropic auth and version metadata", anthropic)
	}

	anthropic.StaticHeaders["anthropic-version"] = "changed"
	again, _ := Lookup(IDAnthropic)
	if again.StaticHeaders["anthropic-version"] != "2023-06-01" {
		t.Error("Lookup returned shared mutable StaticHeaders")
	}
}

func TestValidAPIKey(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{value: "", want: false},
		{value: "sk-abc", want: true},
		{value: "key with spaces/+_=", want: true},
		{value: "bad\nkey", want: false},
		{value: "bad\rkey", want: false},
		{value: "bad\tkey", want: false},
		{value: "bad\x7fkey", want: false},
	}
	for _, test := range tests {
		if got := ValidAPIKey(test.value); got != test.want {
			t.Errorf("ValidAPIKey(%q) = %t, want %t", test.value, got, test.want)
		}
	}
}
