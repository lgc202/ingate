package httpheader

import "testing"

func TestValidValue(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{value: "", want: true},
		{value: "sk-abc", want: true},
		{value: "key with spaces/+_=", want: true},
		{value: "中文密钥", want: true},
		{value: "bad\nvalue"},
		{value: "bad\rvalue"},
		{value: "bad\tvalue"},
		{value: "bad\x00value"},
		{value: "bad\x7fvalue"},
	}

	for _, tt := range tests {
		if got := ValidValue(tt.value); got != tt.want {
			t.Errorf("ValidValue(%q) = %t, want %t", tt.value, got, tt.want)
		}
	}
}
