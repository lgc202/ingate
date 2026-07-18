package bearer

import "testing"

func TestValidToken(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "OpenAI API key", value: "sk-proj_abc.def~ghi+jkl/mno==", want: true},
		{name: "empty", value: ""},
		{name: "ordinary space", value: "secret value"},
		{name: "newline", value: "secret\r\ninjected"},
		{name: "unicode control", value: "secret\u0085value"},
		{name: "unicode separator", value: "secret\u2028value"},
		{name: "unsupported punctuation", value: "secret:value"},
		{name: "padding only", value: "=="},
		{name: "padding in the middle", value: "secret=value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidToken(tt.value); got != tt.want {
				t.Errorf("ValidToken(%q) = %t, want %t", tt.value, got, tt.want)
			}
		})
	}
}
