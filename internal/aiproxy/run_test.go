//go:build unix

package aiproxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"k8s.io/apimachinery/pkg/util/wait"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
	"github.com/lgc202/ingate/internal/aiproxy/conf"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/aiconfig"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/configtest"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/envoyclient"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

type runtimeADS struct {
	discoveryv3.UnimplementedAggregatedDiscoveryServiceServer
	started chan struct{}
	stopped chan struct{}
}

func TestRunStartsUnixServerAndStopsConfigWatch(t *testing.T) {
	deadline, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	ctx, cancel := context.WithCancel(deadline)
	defer cancel()
	watchStarted := make(chan struct{})
	watchStopped := make(chan struct{})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen(test ADS) error = %v", err)
	}

	grpcServer := grpc.NewServer()
	discoveryv3.RegisterAggregatedDiscoveryServiceServer(grpcServer, &runtimeADS{started: watchStarted, stopped: watchStopped})
	var serving sync.WaitGroup
	serving.Go(func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Errorf("Serve(test ADS) error = %v", err)
		}
	})
	t.Cleanup(func() { grpcServer.Stop(); serving.Wait() })

	socket := filepath.Join(unixSocketDirectory(t), "ingress.sock")
	configFile := writeRuntimeConfig(t, socket, listener.Addr().String())
	result := make(chan error, 1)
	var worker sync.WaitGroup
	worker.Go(func() { result <- Run(ctx, configFile) })
	t.Cleanup(func() {
		cancel()
		worker.Wait()
	})

	select {
	case <-watchStarted:
	case err := <-result:
		t.Fatalf("Run(runtime config) returned before Watch started: %v", err)
	case <-deadline.Done():
		t.Fatal("Run(runtime config) did not start the configuration Watch")
	}

	client, err := envoyclient.New(socket)
	if err != nil {
		t.Fatalf("New(Unix ingress client) error = %v", err)
	}

	t.Cleanup(client.CloseIdleConnections)
	request, err := http.NewRequestWithContext(deadline, http.MethodPost, "http://ingate-ai-ingress/v1/chat/completions", strings.NewReader(`{"model":"coding"}`))
	if err != nil {
		t.Fatalf("NewRequestWithContext(runtime ingress) error = %v", err)
	}

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("Do(runtime Unix ingress) error = %v", err)
	}

	if err := response.Body.Close(); err != nil {
		t.Errorf("Close(runtime response) error = %v", err)
	}

	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("Run(runtime ingress without entry headers) status = %d, want 400", response.StatusCode)
	}

	cancel()

	select {
	case err := <-result:
		if err != nil {
			t.Errorf("Run(cancelled runtime) error = %v, want normal shutdown", err)
		}
	case <-deadline.Done():
		t.Fatal("Run(cancelled runtime) did not stop")
	}

	select {
	case <-watchStopped:
	case <-deadline.Done():
		t.Error("Run(cancelled runtime) left an API Watch request active")
	}

	if _, err := os.Stat(socket); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Run(cancelled runtime) socket path error = %v, want owned socket removed", err)
	}
}

func TestRunRejectsClientConfigBeforeBinding(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "ingress.sock")
	data := strings.ReplaceAll(validStartupYAML, "/run/ingate/ingress.sock", socket)
	data = strings.ReplaceAll(data, "127.0.0.1:18000", "invalid-address")
	configFile := writeStartupConfig(t, data)

	err := Run(t.Context(), configFile)
	if err == nil {
		t.Fatal("Run(invalid discovery address) error = nil, want startup failure")
	}

	if _, err := os.Stat(socket); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Run(invalid discovery address) socket path error = %v, want no bound socket", err)
	}
}

func TestRunRejectsLogFileBeforeCreatingDiscoveryClient(t *testing.T) {
	directory := t.TempDir()
	socket := filepath.Join(directory, "ingress.sock")
	data := strings.ReplaceAll(validStartupYAML, "/run/ingate/ingress.sock", socket)
	data += fmt.Sprintf("log:\n  outputs: [file]\n  file:\n    path: %q\n", directory)
	configFile := writeStartupConfig(t, data)

	err := Run(t.Context(), configFile)

	if err == nil || !strings.Contains(err.Error(), "initialize AI Proxy log") {
		t.Fatalf("Run(directory as log file) error = %v, want log initialization failure", err)
	}

	if _, err := os.Stat(socket); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Run(invalid log file) socket error = %v, want no bound socket", err)
	}
}

func TestRunRejectsCancelledContextBeforeLoading(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := Run(ctx, filepath.Join(t.TempDir(), "missing.yaml"))

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Run(cancelled context) error = %v, want context.Canceled before config access", err)
	}
}

func TestAppStopTimeoutCancelsOpenStream(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ended := make(chan struct{})
	egress := unixListener(t, "egress.sock")
	peer := &httptest.Server{Listener: egress, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(ended)
		w.Header().Set("Content-Type", "text/event-stream")
		if _, err := io.WriteString(w, "data: first\n\n"); err != nil {
			t.Errorf("WriteString(shutdown stream) error = %v", err)
			return
		}

		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("Flush(shutdown stream) error = %v", err)
			return
		}

		<-r.Context().Done()
	})}}
	peer.Start()
	t.Cleanup(peer.Close)
	ingress := unixListener(t, "ingress.sock")
	data, err := os.ReadFile("../pkg/aidataplane/testdata/v1.json")
	if err != nil {
		t.Fatalf("ReadFile(shutdown config fixture) error = %v", err)
	}

	compiled, err := aidataplane.Decode(data)
	if err != nil {
		t.Fatalf("Decode(shutdown config) error = %v", err)
	}

	source := configtest.New(t)
	source.Publish(t, *compiled)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := aiconfig.NewLoader(source.Client, "shutdown", logger)

	config := &conf.Bootstrap{
		Server: &conf.Server{
			Socket:              ingress.Addr().String(),
			MaxRequestBodyBytes: 1024,
			ShutdownTimeout:     durationpb.New(20 * time.Millisecond),
		},
		Discovery: &conf.Discovery{NodeId: "shutdown"},
		Envoy:     &conf.Envoy{EgressSocket: egress.Addr().String()},
	}
	app, closeIdle, err := newApp(ctx, config, ingress, loader, logger)
	if err != nil {
		t.Fatalf("newApp(shutdown test) error = %v", err)
	}

	t.Cleanup(closeIdle)
	result := make(chan error, 1)
	var worker sync.WaitGroup
	worker.Go(func() { result <- runApp(ctx, app, loader) })
	t.Cleanup(func() {
		cancel()
		worker.Wait()
	})
	if err := wait.PollUntilContextTimeout(ctx, time.Millisecond, time.Second, true, func(context.Context) (bool, error) {
		_, loaded := loader.Capture()
		return loaded, nil
	}); err != nil {
		t.Fatalf("Capture(shutdown test fixture) did not become available: %v", err)
	}

	client, err := envoyclient.New(ingress.Addr().String())
	if err != nil {
		t.Fatalf("New(shutdown test client) error = %v", err)
	}

	t.Cleanup(client.CloseIdleConnections)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://ingate-ai-ingress/v1/chat/completions", strings.NewReader(`{"model":"coding","stream":true}`))
	if err != nil {
		t.Fatalf("NewRequestWithContext(shutdown test) error = %v", err)
	}

	request.Header.Set(aidataplane.ListenerPortHeader, "8080")
	request.Header.Set(aidataplane.HostNameHeader, "gateway/team-a/public/listener/api")

	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("Do(shutdown stream) error = %v", err)
	}

	t.Cleanup(func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("Close(shutdown response) error = %v", err)
		}
	})
	first, err := bufio.NewReader(response.Body).ReadString('\n')
	if err != nil || first != "data: first\n" {
		t.Fatalf("ReadString(shutdown stream) = (%q, %v), want first event before upstream completion", first, err)
	}

	if err := app.Stop(); err != nil {
		t.Fatalf("Stop(application with open SSE) error = %v", err)
	}

	select {
	case err := <-result:
		if err != nil {
			t.Errorf("runApp(shutdown deadline) error = %v, want native HTTP force-close completion", err)
		}
	case <-ctx.Done():
		t.Fatal("runApp(shutdown deadline) did not terminate open SSE")
	}

	if ctx.Err() != nil {
		t.Error("runApp stopped because the outer test expired, not the configured shutdown timeout")
	}

	select {
	case <-ended:
	case <-ctx.Done():
		t.Error("shutdown timeout did not cancel the upstream Unix request")
	}

	if _, loaded := loader.Capture(); loaded {
		t.Error("runApp returned before the configuration Loader cleared its state")
	}
}

func writeRuntimeConfig(t *testing.T, socket, endpoint string) string {
	t.Helper()
	data := strings.ReplaceAll(validStartupYAML, "/run/ingate/ingress.sock", socket)
	data = strings.ReplaceAll(data, "127.0.0.1:18000", endpoint)

	return writeStartupConfig(t, data)
}

func (s *runtimeADS) DeltaAggregatedResources(stream discoveryv3.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
	if _, err := stream.Recv(); err != nil {
		return err
	}

	close(s.started)
	defer close(s.stopped)
	if err := stream.Send(&discoveryv3.DeltaDiscoveryResponse{TypeUrl: aidataplanev1.ResourceType, Nonce: "empty"}); err != nil {
		return err
	}

	<-stream.Context().Done()

	return stream.Context().Err()
}
