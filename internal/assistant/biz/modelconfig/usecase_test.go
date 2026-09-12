package modelconfig

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

type recordingStore struct {
	update Update
	calls  int
	result Connection
}

func (s *recordingStore) GetModelConnection(context.Context) (Connection, error) {
	return s.result, nil
}

func (s *recordingStore) UpdateModelConnection(_ context.Context, update Update) (Connection, error) {
	s.update = update
	s.calls++
	return s.result, nil
}

// TestUsecaseUpdatePreservesCredentialIntent 验证更新会规范化连接地址，但不会改变凭据更新意图。
func TestUsecaseUpdatePreservesCredentialIntent(t *testing.T) {
	for _, test := range []struct {
		name   string
		apiKey *string
	}{
		{name: "keep", apiKey: nil},
		{name: "clear", apiKey: new("")},
		{name: "replace_without_trimming", apiKey: new(" new-key ")},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := Update{
				Mode:                  ModeDirect,
				Protocol:              ProtocolAnthropic,
				Endpoint:              " https://model.example/v1/// ",
				Model:                 " model-name ",
				Timeout:               90 * time.Second,
				MaxOutputTokens:       8192,
				ReasoningBudgetTokens: 2048,
				APIKey:                test.apiKey,
			}
			want := input
			want.Endpoint = "https://model.example/v1"
			want.Model = "model-name"
			store := &recordingStore{result: Connection{Configured: true, UpdatedAt: time.Unix(10, 0)}}
			got, err := NewUsecase(store).Update(t.Context(), input)
			if err != nil {
				t.Fatalf("Update(%s) error = %v, want nil", test.name, err)
			}
			if store.calls != 1 {
				t.Errorf("Update(%s) store calls = %d, want 1", test.name, store.calls)
			}
			if diff := cmp.Diff(want, store.update); diff != "" {
				t.Errorf("Update(%s) stored input mismatch (-want +got):\n%s", test.name, diff)
			}
			if diff := cmp.Diff(store.result, got); diff != "" {
				t.Errorf("Update(%s) result mismatch (-want +got):\n%s", test.name, diff)
			}
		})
	}
}

// TestUsecaseUpdateRejectsInvalidConnection 验证无效更新不会进入持久化边界。
func TestUsecaseUpdateRejectsInvalidConnection(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Update)
	}{
		{name: "endpoint_query", change: func(update *Update) { update.Endpoint += "?key=value" }},
		{name: "empty_model", change: func(update *Update) { update.Model = "  " }},
		{name: "ingate_anthropic", change: func(update *Update) { update.Protocol = ProtocolAnthropic }},
		{name: "zero_timeout", change: func(update *Update) { update.Timeout = 0 }},
		{name: "zero_output_tokens", change: func(update *Update) { update.MaxOutputTokens = 0 }},
		{name: "unsupported_reasoning", change: func(update *Update) { update.ReasoningBudgetTokens = 1024 }},
		{name: "oversized_key", change: func(update *Update) { update.APIKey = new(strings.Repeat("x", maxAPIKeyLength+1)) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := Update{
				Mode:            ModeIngate,
				Protocol:        ProtocolOpenAICompatible,
				Endpoint:        "https://model.example/v1",
				Model:           "model-name",
				Timeout:         DefaultTimeout,
				MaxOutputTokens: DefaultMaxOutputTokens,
			}
			test.change(&input)
			store := &recordingStore{}
			_, err := NewUsecase(store).Update(t.Context(), input)
			if !errors.Is(err, ErrInvalidConnection) {
				t.Errorf("Update(%s) error = %v, want ErrInvalidConnection", test.name, err)
			}
			if store.calls != 0 {
				t.Errorf("Update(%s) store calls = %d, want 0", test.name, store.calls)
			}
		})
	}
}
