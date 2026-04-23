package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	appoptions "github.com/lgc202/ingate/cmd/controller-manager/app/options"
	controllerconfig "github.com/lgc202/ingate/internal/controlplane/controller/config"
	controllerhealth "github.com/lgc202/ingate/internal/controlplane/controller/health"
)

func TestNewControllerManagerCommand(t *testing.T) {
	cmd := NewControllerManagerCommand()
	if cmd == nil {
		t.Fatal("expected command")
	}

	if got, want := cmd.Use, "ingate-controller-manager"; got != want {
		t.Fatalf("unexpected command use: got %q, want %q", got, want)
	}

	cmd.SetArgs([]string{"unexpected"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected positional args to be rejected")
	}
	if !strings.Contains(err.Error(), "does not take any arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestControllerManagerBootstrapValidationAndConfig(t *testing.T) {
	t.Run("validation failures are aggregated", func(t *testing.T) {
		errs := controllerconfig.CompletedOptions{
			APIServerAddress:        "",
			MetricsBindAddress:      "",
			HealthzBindAddress:      "",
			Workers:                 0,
			LeaderElectionEnabled:   true,
			LeaderElectionName:      "",
			LeaderElectionNamespace: "",
		}.Validate()

		if len(errs) == 0 {
			t.Fatal("expected validation errors")
		}

		agg := utilerrors.NewAggregate(errs)
		for _, want := range []string{
			"apiserver-address must not be empty",
			"metrics-bind-address must not be empty",
			"healthz-bind-address must not be empty",
			"workers must be at least 1",
			"leader-election-name must not be empty when leader election is enabled",
			"leader-election-namespace must not be empty when leader election is enabled",
		} {
			if !strings.Contains(agg.Error(), want) {
				t.Fatalf("aggregate error %q missing %q", agg.Error(), want)
			}
		}
	})

	t.Run("completion and config construction succeed", func(t *testing.T) {
		completed, err := controllerconfig.NewOptions().Complete()
		if err != nil {
			t.Fatalf("complete options: %v", err)
		}

		if completed.APIServerAddress == "" {
			t.Fatal("expected default apiserver address")
		}
		if completed.HealthzBindAddress == "" {
			t.Fatal("expected default healthz address")
		}
		if completed.Workers < 1 {
			t.Fatal("expected positive worker count")
		}

		cfg, err := controllerconfig.NewConfig(completed)
		if err != nil {
			t.Fatalf("build config: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected config")
		}
		if cfg.HealthServer == nil {
			t.Fatal("expected health server")
		}
		if cfg.Options != completed {
			t.Fatalf("unexpected completed options: got %#v, want %#v", cfg.Options, completed)
		}
	})
}

func TestHealthServerEndpointsAndShutdown(t *testing.T) {
	srv, err := controllerhealth.NewServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("new health server: %v", err)
	}

	handler := srv.Handler()
	if handler == nil {
		t.Fatal("expected handler")
	}

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	for _, path := range []string{"/healthz", "/readyz"} {
		resp, err := http.Get(ts.URL + path) // #nosec G107 -- local test server
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			t.Fatalf("read %s body: %v", path, readErr)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("unexpected status for %s: got %d", path, resp.StatusCode)
		}
		if got := strings.TrimSpace(string(body)); got != "ok" {
			t.Fatalf("unexpected body for %s: got %q", path, got)
		}
	}

	healthServer, err := controllerhealth.NewServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("new health server: %v", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- healthServer.RunWithListener(ctx, listener)
	}()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("shutdown returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for shutdown")
	}
}

func TestRunWritesToProvidedWriter(t *testing.T) {
	addr := reservedLoopbackAddress(t)
	opts := appoptions.CompletedOptions{
		APIServerAddress:        "https://127.0.0.1:18443",
		Kubeconfig:              "",
		LeaderElectionEnabled:   false,
		LeaderElectionName:      "ingate-controller-manager",
		LeaderElectionNamespace: "ingate-system",
		MetricsBindAddress:      "127.0.0.1:18080",
		HealthzBindAddress:      addr,
		WatchNamespace:          "",
		Workers:                 1,
	}

	var output bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, &output, opts)
	}()

	waitForHTTP(t, "http://"+addr+"/healthz")
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for shutdown")
	}

	if !strings.Contains(output.String(), "starting ingate-controller-manager") {
		t.Fatalf("expected startup output, got %q", output.String())
	}
}

func TestCommandWritesStartupOutputToStdout(t *testing.T) {
	addr := reservedLoopbackAddress(t)
	cmd := NewControllerManagerCommand()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{fmt.Sprintf("--healthz-bind-address=%s", addr)})

	ctx, cancel := context.WithCancel(context.Background())
	cmd.SetContext(ctx)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Execute()
	}()

	waitForHTTP(t, "http://"+addr+"/healthz")
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for command shutdown")
	}

	if !strings.Contains(stdout.String(), "starting ingate-controller-manager") {
		t.Fatalf("expected startup output on stdout, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no startup output on stderr, got %q", stderr.String())
	}
}

func reservedLoopbackAddress(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve loopback address: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release reserved listener: %v", err)
	}
	return addr
}

func waitForHTTP(t *testing.T, url string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) // #nosec G107 -- local test server
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", url)
}
