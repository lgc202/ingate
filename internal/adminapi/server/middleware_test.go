package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/lgc202/ingate/internal/adminapi/errno"
)

func TestRequestLogging(t *testing.T) {
	privateCause := errors.New("private database failure")
	for _, tt := range []struct {
		name      string
		level     slog.Level
		err       error
		wantLog   bool
		wantCause bool
	}{
		{name: "success at info", level: slog.LevelInfo},
		{name: "client failure at info", level: slog.LevelInfo, err: errno.ErrInvalidArgument.WithCause(errors.New("private-input"))},
		{name: "success at debug", level: slog.LevelDebug, wantLog: true},
		{name: "client failure at debug", level: slog.LevelDebug, err: errno.ErrInvalidArgument.WithCause(errors.New("private-input")), wantLog: true},
		{name: "server failure at info", level: slog.LevelInfo, err: errno.Fallback(privateCause), wantLog: true, wantCause: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: tt.level}))
			calls := 0
			reply := &struct{}{}
			handler := requestLogging(logger)(func(_ context.Context, req any) (any, error) {
				calls++
				if req != "private-token" {
					t.Errorf("downstream request = %v, want original request", req)
				}

				return reply, tt.err
			})

			got, err := handler(t.Context(), "private-token")
			if calls != 1 || got != reply || err != tt.err {
				t.Errorf("requestLogging() calls/reply/error = %d/%v/%v, want 1/%v/%v", calls, got, err, reply, tt.err)
			}

			if logged := output.Len() > 0; logged != tt.wantLog {
				t.Fatalf("requestLogging(%s) output = %q, want log = %t", tt.name, output.String(), tt.wantLog)
			}

			if !tt.wantLog {
				return
			}

			var record map[string]any
			if err := json.Unmarshal(output.Bytes(), &record); err != nil {
				t.Fatalf("requestLogging(%s) output = %q, want one JSON record: %v", tt.name, output.String(), err)
			}

			if record["msg"] != "server request" || record["kind"] != "server" || record["latency"] == nil {
				t.Errorf("requestLogging(%s) record = %v, want fixed message, kind and latency", tt.name, record)
			}

			if strings.Contains(output.String(), "private-token") || strings.Contains(output.String(), "private-input") {
				t.Errorf("requestLogging(%s) leaked client data: %s", tt.name, output.String())
			}

			if hasCause := strings.Contains(output.String(), privateCause.Error()); hasCause != tt.wantCause {
				t.Errorf("requestLogging(%s) internal cause present = %t, want %t", tt.name, hasCause, tt.wantCause)
			}
		})
	}
}
