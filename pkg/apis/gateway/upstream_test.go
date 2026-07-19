package gateway

import "testing"

func TestModelProviderProtocol(t *testing.T) {
	tests := []struct {
		provider ModelProvider
		want     UpstreamProtocol
		wantOK   bool
	}{
		{provider: ModelProviderOpenAI, want: UpstreamProtocolOpenAI, wantOK: true},
		{provider: ModelProviderDeepSeek, want: UpstreamProtocolOpenAI, wantOK: true},
		{provider: ModelProviderQwen, want: UpstreamProtocolOpenAI, wantOK: true},
		{provider: ModelProviderCustom, want: UpstreamProtocolOpenAI, wantOK: true},
		{provider: ModelProviderAnthropic, want: UpstreamProtocolAnthropic, wantOK: true},
		{provider: ModelProviderGemini, want: UpstreamProtocolGemini, wantOK: true},
		{provider: ModelProvider("unknown")},
	}

	for _, tt := range tests {
		protocol, ok := tt.provider.Protocol()
		if protocol != tt.want || ok != tt.wantOK {
			t.Errorf("ModelProvider(%q).Protocol() = (%q, %t), want (%q, %t)", tt.provider, protocol, ok, tt.want, tt.wantOK)
		}
	}
}

func TestUpstreamProtocolIsSupported(t *testing.T) {
	tests := []struct {
		protocol UpstreamProtocol
		want     bool
	}{
		{protocol: UpstreamProtocolHTTP, want: true},
		{protocol: UpstreamProtocolOpenAI, want: true},
		{protocol: UpstreamProtocolAnthropic, want: true},
		{protocol: UpstreamProtocolGemini, want: true},
		{protocol: UpstreamProtocol("unknown")},
	}

	for _, tt := range tests {
		if got := tt.protocol.IsSupported(); got != tt.want {
			t.Errorf("UpstreamProtocol(%q).IsSupported() = %t, want %t", tt.protocol, got, tt.want)
		}
	}
}
