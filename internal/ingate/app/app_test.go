package app_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lgc202/ingate-next/internal/ingate/app"
)

func TestRunBuildDebugFromStdin(t *testing.T) {
	input := strings.NewReader(`{
		"gateways": [
			{
				"metadata": {"name": "public"},
				"spec": {
					"listeners": [
						{"name": "http", "protocol": "HTTP", "port": 80, "hostname": "example.com"}
					]
				}
			}
		],
		"routes": [
			{
				"metadata": {"name": "app"},
				"spec": {
					"parentRefs": ["public"],
					"hostnames": ["example.com"],
					"rules": [
						{
							"pathPrefix": "/app",
							"upstreamRefs": [{"name": "app", "weight": 100}]
						}
					]
				}
			}
		],
		"upstreams": [
			{
				"metadata": {"name": "app"},
				"spec": {
					"endpoints": [{"address": "10.0.0.10", "port": 8080}]
				}
			}
		]
	}`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := app.Run([]string{"build", "--file", "-", "--gateway", "public", "--target", "debug"}, input, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v, stderr = %s", err, stderr.String())
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not json: %v\n%s", err, stdout.String())
	}
	if got["target"] != "debug" {
		t.Fatalf("target = %v, want debug", got["target"])
	}
	if got["gateway"] != "public" {
		t.Fatalf("gateway = %v, want public", got["gateway"])
	}
	if _, ok := got["config"].(map[string]any); !ok {
		t.Fatalf("config type = %T, want object", got["config"])
	}
	config := got["config"].(map[string]any)
	if _, ok := config["listeners"]; !ok {
		t.Fatalf("config keys = %#v, want listeners", config)
	}
}

func TestRunBuildMissingTarget(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := app.Run([]string{"build", "--file", "-", "--gateway", "public", "--target", "missing"}, strings.NewReader(`{}`), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run() error = nil")
	}
	if !strings.Contains(err.Error(), `target "missing" not registered`) {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	err := app.Run([]string{"unknown"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run() error = nil")
	}
	if !strings.Contains(err.Error(), `unknown command "unknown"`) {
		t.Fatalf("Run() error = %v", err)
	}
}
