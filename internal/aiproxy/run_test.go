//go:build unix

package aiproxy

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/lgc202/ingate/internal/aiproxy/conf"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/aiconfig"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/envoyclient"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

func TestRunStartsUnixServerAndStopsConfigWatch(t *testing.T) {
	deadline, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	ctx, cancel := context.WithCancel(deadline)
	defer cancel()
	watchStarted := make(chan struct{})
	watchStopped := make(chan struct{})
	var started, stopped sync.Once
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/namespaces/control-plane/configmaps" || r.URL.Query().Get("fieldSelector") != "metadata.name="+aidataplane.ConfigMapName {
			t.Errorf("AI config API request = %s, want scoped ConfigMap List/Watch", r.URL)
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("watch") == "true" {
			w.WriteHeader(http.StatusOK)
			if err := http.NewResponseController(w).Flush(); err != nil {
				t.Errorf("Flush(config watch) error = %v", err)
				return
			}

			started.Do(func() { close(watchStarted) })
			<-r.Context().Done()
			stopped.Do(func() { close(watchStopped) })

			return
		}

		if _, err := io.WriteString(w, `{"apiVersion":"v1","kind":"ConfigMapList","metadata":{"resourceVersion":"1"},"items":[]}`); err != nil {
			t.Errorf("WriteString(config list) error = %v", err)
		}
	}))
	t.Cleanup(api.Close)
	socket := filepath.Join(unixSocketDirectory(t), "ingress.sock")
	configFile := writeRuntimeConfig(t, socket, api.URL)
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
	data = strings.ReplaceAll(data, "/etc/ingate/ai-proxy.kubeconfig", filepath.Join(t.TempDir(), "missing.kubeconfig"))
	configFile := writeStartupConfig(t, data)

	err := Run(t.Context(), configFile)
	if err == nil {
		t.Fatal("Run(missing kubeconfig) error = nil, want startup failure")
	}

	if _, err := os.Stat(socket); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Run(missing kubeconfig) socket path error = %v, want no bound socket", err)
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

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "control-plane", Name: aidataplane.ConfigMapName, UID: "shutdown"},
		Data:       map[string]string{aidataplane.ConfigMapDataKey: string(data)},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := aiconfig.NewLoader(fake.NewClientset(configMap), "control-plane", logger)

	config := &conf.Bootstrap{
		Server: &conf.Server{
			Socket:              ingress.Addr().String(),
			MaxRequestBodyBytes: 1024,
			ShutdownTimeout:     durationpb.New(20 * time.Millisecond),
		},
		ApiServer: &conf.APIServer{AiConfigNamespace: "control-plane"},
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
	kubeconfig := filepath.Join(t.TempDir(), "api.kubeconfig")
	connection := clientcmdapi.Config{
		Clusters:       map[string]*clientcmdapi.Cluster{"ingate": {Server: endpoint}},
		Contexts:       map[string]*clientcmdapi.Context{"ingate": {Cluster: "ingate"}},
		CurrentContext: "ingate",
	}
	if err := clientcmd.WriteToFile(connection, kubeconfig); err != nil {
		t.Fatalf("WriteToFile(runtime kubeconfig) error = %v", err)
	}

	data := strings.ReplaceAll(validStartupYAML, "/run/ingate/ingress.sock", socket)
	data = strings.ReplaceAll(data, "/etc/ingate/ai-proxy.kubeconfig", kubeconfig)

	return writeStartupConfig(t, data)
}
