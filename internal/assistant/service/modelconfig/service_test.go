package modelconfig

import (
	"testing"
	"time"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/go-cmp/cmp"

	assistantv1 "github.com/lgc202/ingate/api/assistant/v1"
	modelconfigbiz "github.com/lgc202/ingate/internal/assistant/biz/modelconfig"
)

// TestModelUpdateCredentialIntent 验证产品协议中的省略、替换和清空凭据映射到不同更新意图。
func TestModelUpdateCredentialIntent(t *testing.T) {
	for _, test := range []struct {
		name     string
		apiKey   *string
		clearKey bool
		wantKey  *string
	}{
		{name: "keep"},
		{name: "replace", apiKey: new("new-key"), wantKey: new("new-key")},
		{name: "explicit_empty", apiKey: new(""), wantKey: new("")},
		{name: "clear", clearKey: true, wantKey: new("")},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := &assistantv1.UpdateModelConnectionRequest{
				ConnectionMode:        assistantv1.ModelConnectionMode_MODEL_CONNECTION_MODE_DIRECT,
				Protocol:              assistantv1.ModelProtocol_MODEL_PROTOCOL_ANTHROPIC,
				Endpoint:              "https://model.example/v1",
				Model:                 "model-name",
				TimeoutSeconds:        90,
				MaxOutputTokens:       8192,
				ReasoningBudgetTokens: 2048,
				ApiKey:                test.apiKey,
				ClearApiKey:           test.clearKey,
			}
			want := modelconfigbiz.Update{
				Mode:                  modelconfigbiz.ModeDirect,
				Protocol:              modelconfigbiz.ProtocolAnthropic,
				Endpoint:              request.Endpoint,
				Model:                 request.Model,
				Timeout:               90 * time.Second,
				MaxOutputTokens:       8192,
				ReasoningBudgetTokens: 2048,
				APIKey:                test.wantKey,
			}
			got, err := modelUpdate(request)
			if err != nil {
				t.Fatalf("modelUpdate(%s) error = %v, want nil", test.name, err)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("modelUpdate(%s) mismatch (-want +got):\n%s", test.name, diff)
			}
		})
	}
}

// TestModelUpdateRejectsConflictingCredentials 验证显式凭据和清空开关不能同时出现。
func TestModelUpdateRejectsConflictingCredentials(t *testing.T) {
	_, err := modelUpdate(&assistantv1.UpdateModelConnectionRequest{
		ApiKey:      new(""),
		ClearApiKey: true,
	})
	if got := kerrors.Code(err); got != 400 {
		t.Errorf("modelUpdate(conflicting credentials) code = %d, want 400", got)
	}
}
