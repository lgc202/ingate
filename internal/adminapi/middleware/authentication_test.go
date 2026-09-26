package middleware

import "testing"

func TestBearerToken(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
		wantToken     string
		wantOK        bool
	}{
		{name: "standard", authorization: "Bearer abc", wantToken: "abc", wantOK: true},
		{name: "case insensitive", authorization: "bearer abc", wantToken: "abc", wantOK: true},
		{name: "missing", authorization: "Bearer", wantOK: false},
		{name: "wrong scheme", authorization: "Basic abc", wantOK: false},
		{name: "extra field", authorization: "Bearer abc def", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, ok := bearerToken(tt.authorization)
			if token != tt.wantToken || ok != tt.wantOK {
				t.Errorf("bearerToken(%q) = (%q, %t), want (%q, %t)", tt.authorization, token, ok, tt.wantToken, tt.wantOK)
			}
		})
	}
}
