package xds

import (
	"testing"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
)

func TestPublishAIAttemptsAIProxyWhenEnvoyBuildFails(t *testing.T) {
	publisher := NewPublisher("", "")
	config := emptyAIConfig()
	config.FormatVersion = "invalid"

	err := publisher.PublishAI(t.Context(), config, nil)
	if err == nil {
		t.Error("PublishAI(invalid Envoy candidate) error = nil, want build error")
	}

	if !publisher.aiResources.initialized.Load() {
		t.Error("PublishAI(Envoy build failure) did not install AI Proxy empty candidate")
	}

	if err := publisher.envoy.CheckReady(); err == nil {
		t.Error("PublishAI(Envoy build failure) initialized invalid Envoy snapshot")
	}
}

func TestPublishAIDoesNotRollBackEnvoyOnAIProxyBuildFailure(t *testing.T) {
	publisher := NewPublisher("", "")
	resources := []*aidataplanev1.Resource{{Name: "\xff"}}

	err := publisher.PublishAI(t.Context(), emptyAIConfig(), resources)
	if err == nil {
		t.Error("PublishAI(invalid AI Proxy candidate) error = nil, want encoding error")
	}

	if err := publisher.envoy.CheckReady(); err != nil {
		t.Errorf("PublishAI(AI Proxy build failure) lost valid Envoy candidate: %v", err)
	}

	if publisher.aiResources.initialized.Load() {
		t.Error("PublishAI(AI Proxy build failure) initialized invalid AI Proxy cache")
	}

	if err := publisher.PublishAI(t.Context(), emptyAIConfig(), nil); err != nil {
		t.Fatalf("PublishAI(valid retry) error = %v", err)
	}

	if err := publisher.checkReady(); err != nil {
		t.Errorf("checkReady(after valid retry) error = %v, want nil", err)
	}
}
