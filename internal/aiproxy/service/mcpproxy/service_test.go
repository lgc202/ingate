package mcpproxy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"testing/synctest"

	"github.com/lgc202/ingate/internal/aiproxy/pkg/aiconfig"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/configtest"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

const testMCPBody = `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`

type roundTripFunc func(*http.Request) (*http.Response, error)

type countedBody struct {
	reads  int
	closes int
}

func TestForwardSendsPreparedRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, _ := runTestService(t, fixtureConfig(t))
		body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_weather","_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`)
		original := bytes.Clone(body)
		headers := http.Header{
			"Content-Type":           {"application/json"},
			"Accept":                 {"application/json, text/event-stream"},
			"Mcp-Protocol-Version":   {"2026-07-28"},
			"Mcp-Method":             {"tools/call"},
			"Mcp-Name":               {"get_weather"},
			"Mcp-Param-Region":       {"us-west1"},
			"Origin":                 {"https://gateway.example.com"},
			"Authorization":          {"private credential"},
			"Cookie":                 {"private cookie"},
			"Connection":             {"keep-alive, X-Private-Hop"},
			"X-Private-Hop":          {"private"},
			aidataplane.TargetHeader: {"spoofed"},
		}
		params := RequestParams{
			ListenerPort: 8080, HostName: "gateway/team-a/public/listener/api",
			Method: http.MethodPost, Path: "/mcp", Header: headers, Body: body,
		}

		response, err := service.Forward(t.Context(), params)
		if err != nil {
			t.Fatalf("Forward(tools/call) error = %v", err)
		}

		data := readResponse(t, response)

		if string(data) != string(original) {
			t.Errorf("Forward() body = %s, want unchanged %s", data, original)
		}

		if response.Header.Get("X-Test-Method") != http.MethodPost || response.Header.Get("X-Test-Path") != "/stream" {
			t.Errorf("Forward() outbound = %s %s, want POST to the configured backend path", response.Header.Get("X-Test-Method"), response.Header.Get("X-Test-Path"))
		}

		if target := response.Header.Get("X-Test-Target"); target != "ai/mcp-target/f302e6faf73db6eca1dab4385f17012e06b05d90e0ca416ddfcec9aa73fb4af3" {
			t.Errorf("Forward() target = %q, want the tools network target", target)
		}

		for _, name := range []string{"Authorization", "Cookie", "Connection", "X-Private-Hop", aidataplane.ListenerPortHeader, aidataplane.HostNameHeader} {
			if value := response.Header.Get("X-Test-Header-" + name); value != "" {
				t.Errorf("Forward() retained client or internal header %q = %q", name, value)
			}
		}

		for _, name := range []string{"Content-Type", "Accept", "Mcp-Protocol-Version", "Mcp-Method", "Mcp-Name", "Mcp-Param-Region", "Origin"} {
			if value := response.Header.Get("X-Test-Header-" + name); value == "" {
				t.Errorf("Forward() dropped forwardable header %q", name)
			}
		}

		if !bytes.Equal(body, original) {
			t.Errorf("Forward() modified input to %s, want %s", body, original)
		}
	})
}

func TestForwardPreservesMethodAndQueryFreePath(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, _ := runTestService(t, fixtureConfig(t))

		response, err := service.Forward(t.Context(), RequestParams{
			ListenerPort: 8080, HostName: "gateway/team-a/public/listener/api",
			Method: http.MethodPost, Path: "/mcp", Header: testMCPHeaders(), Body: []byte(testMCPBody),
		})
		if err != nil {
			t.Fatalf("Forward(POST) error = %v", err)
		}

		readResponse(t, response)

		if response.Header.Get("X-Test-Method") != http.MethodPost || response.Header.Get("X-Test-Path") != "/stream" || response.Header.Get("X-Test-RawQuery") != "" {
			t.Errorf("Forward(POST) = %s %s?%s, want original method on the configured path without query",
				response.Header.Get("X-Test-Method"), response.Header.Get("X-Test-Path"), response.Header.Get("X-Test-RawQuery"))
		}
	})
}

func TestForwardRejectsBeforeClientCall(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, _ := runTestService(t, fixtureConfig(t))
		service.client.Transport = rejectUnexpectedRequest(t)
		for _, tt := range []struct {
			name string
			port int32
			host string
			path string
		}{
			{name: "trailing slash", port: 8080, host: "gateway/team-a/public/listener/api", path: "/mcp/"},
			{name: "prefix only", port: 8080, host: "gateway/team-a/public/listener/api", path: "/mcpx"},
			{name: "other namespace", port: 8080, host: "gateway/team-b/public/listener/api", path: "/mcp"},
			{name: "other port", port: 8081, host: "gateway/team-a/public/listener/api", path: "/mcp"},
			{name: "public hostname", port: 8080, host: "gateway.example.com", path: "/mcp"},
		} {
			response, err := service.Forward(t.Context(), RequestParams{
				ListenerPort: tt.port, HostName: tt.host, Method: http.MethodPost, Path: tt.path, Header: testMCPHeaders(), Body: []byte(testMCPBody),
			})

			if response != nil || !errors.Is(err, ErrRouteNotFound) || !errors.Is(err, aiconfig.ErrMCPRouteNotFound) {
				t.Errorf("Forward(%s) = (%v, %v), want unmatched entry without outbound request", tt.name, response, err)
			}
		}
	})
}

func TestForwardDoesNotFallThroughUnavailableBackend(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		config := fixtureConfig(t)
		compiled := cloneTestConfig(t, config)
		fallback := compiled.Listeners[0].Hosts[0].MCPRoutes[0]
		fallback.Name = "team-a/fallback"
		fallback.BackendPath = "/fallback"
		compiled.Listeners[0].Hosts[0].MCPRoutes = append(compiled.Listeners[0].Hosts[0].MCPRoutes, fallback)
		compiled.Listeners[0].Hosts[0].MCPRoutes[0].BackendName = new("")
		*config = *compiled
		service, _ := runTestService(t, config)
		service.client.Transport = rejectUnexpectedRequest(t)

		response, err := service.Forward(t.Context(), RequestParams{
			ListenerPort: 8080, HostName: "gateway/team-a/public/listener/api",
			Method: http.MethodPost, Path: "/mcp", Header: testMCPHeaders(), Body: []byte(testMCPBody),
		})

		if response != nil || !errors.Is(err, ErrUnavailable) || !errors.Is(err, aiconfig.ErrMCPBackendUnavailable) {
			t.Errorf("Forward(unavailable backend) = (%v, %v), want backend error without fallback", response, err)
		}
	})
}

func TestForwardConfigurationChanges(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		config := fixtureConfig(t)
		service, source := runTestService(t, config)
		params := RequestParams{
			ListenerPort: 8080, HostName: "gateway/team-a/public/listener/api",
			Method: http.MethodPost, Path: "/mcp", Header: testMCPHeaders(), Body: []byte(testMCPBody),
		}
		previous, err := service.Forward(t.Context(), params)
		if err != nil {
			t.Fatalf("Forward(initial configuration) error = %v", err)
		}

		previousPath, previousTarget := previous.Header.Get("X-Test-Path"), previous.Header.Get("X-Test-Target")
		readResponse(t, previous)

		compiled := cloneTestConfig(t, config)
		compiled.Listeners[0].Hosts[0].MCPRoutes[0].BackendPath = "/v2/stream"
		compiled.NetworkBackends[1].Endpoints[0].Address = "tools-v2.internal.example"
		*config = *compiled
		source.Publish(t, *config)

		synctest.Wait()

		current, err := service.Forward(t.Context(), params)
		if err != nil {
			t.Fatalf("Forward(updated configuration) error = %v", err)
		}

		currentPath, currentTarget := current.Header.Get("X-Test-Path"), current.Header.Get("X-Test-Target")
		readResponse(t, current)

		if previousPath != "/stream" || currentPath != "/v2/stream" || previousTarget == currentTarget {
			t.Errorf("Forward(config update) paths/targets = %q/%q and %q/%q, want new path and target", previousPath, currentPath, previousTarget, currentTarget)
		}

		source.Publish(t, aidataplane.Config{})

		synctest.Wait()
		service.client.Transport = rejectUnexpectedRequest(t)

		response, err := service.Forward(t.Context(), params)

		if response != nil || !errors.Is(err, ErrRouteNotFound) {
			t.Errorf("Forward(removed routes) = (%v, %v), want route not found without outbound request", response, err)
		}
	})
}

func TestForwardBeforeConfigurationAndCancelled(t *testing.T) {
	loader := aiconfig.NewLoader(nil, "test", slog.Default())
	service := New(loader, &http.Client{Transport: rejectUnexpectedRequest(t)})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	response, err := service.Forward(t.Context(), RequestParams{})

	if response != nil || !errors.Is(err, ErrUnavailable) {
		t.Errorf("Forward(before load) = (%v, %v), want unavailable", response, err)
	}

	response, err = service.Forward(ctx, RequestParams{})

	if response != nil || !errors.Is(err, context.Canceled) {
		t.Errorf("Forward(canceled) = (%v, %v), want canceled before selecting or networking", response, err)
	}
}

func TestForwardTransfersResponseBodyWithoutReading(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, _ := runTestService(t, fixtureConfig(t))
		body := &countedBody{}
		upstream := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: body, ContentLength: -1}
		service.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { return upstream, nil })

		response, err := service.Forward(t.Context(), RequestParams{
			ListenerPort: 8080, HostName: "gateway/team-a/public/listener/api",
			Method: http.MethodPost, Path: "/mcp", Header: testMCPHeaders(), Body: []byte(testMCPBody),
		})

		if err != nil || response != upstream || body.reads != 0 || body.closes != 0 {
			t.Fatalf("Forward(stream response) = (%v, %v, reads %d, closes %d), want original response without consumption", response, err, body.reads, body.closes)
		}

		if err := response.Body.Close(); err != nil || body.closes != 1 {
			t.Errorf("Close(transferred response) = (%v, closes %d), want caller-owned single close", err, body.closes)
		}
	})
}

func TestForwardRejectsUnexpectedProtocolUpgrade(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, _ := runTestService(t, fixtureConfig(t))
		body := &countedBody{}
		service.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusSwitchingProtocols, Header: make(http.Header), Body: body}, nil
		})

		response, err := service.Forward(t.Context(), RequestParams{
			ListenerPort: 8080, HostName: "gateway/team-a/public/listener/api",
			Method: http.MethodPost, Path: "/mcp", Header: testMCPHeaders(), Body: []byte(testMCPBody),
		})

		if response != nil || !errors.Is(err, ErrForwardFailed) || body.closes != 1 {
			t.Errorf("Forward(unrequested upgrade) = (%v, %v, closes %d), want rejection and connection cleanup", response, err, body.closes)
		}
	})
}

func TestForwardPreservesClientFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		service, _ := runTestService(t, fixtureConfig(t))
		cause := errors.New("private connection details")
		service.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, cause })

		response, err := service.Forward(t.Context(), RequestParams{
			ListenerPort: 8080, HostName: "gateway/team-a/public/listener/api",
			Method: http.MethodPost, Path: "/mcp", Header: testMCPHeaders(), Body: []byte(testMCPBody),
		})

		if response != nil || !errors.Is(err, ErrForwardFailed) || !errors.Is(err, cause) {
			t.Errorf("Forward(client failure) = (%v, %v), want forwarding error with original cause", response, err)
		}
	})
}

func TestForwardKeepsCapturedTargetDuringConfigurationUpdate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		config := fixtureConfig(t)
		service, source := runTestService(t, config)
		compiled := cloneTestConfig(t, config)
		compiled.Listeners[0].Hosts[0].MCPRoutes[0].BackendPath = "/updated"
		compiled.NetworkBackends[1].Endpoints[0].Address = "tools-v2.internal.example"
		*config = *compiled
		var previousTarget string
		calls := 0
		service.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				previousTarget = request.Header.Get(aidataplane.TargetHeader)
				source.Publish(t, *config)

				synctest.Wait()
				if request.URL.Path != "/stream" || request.Header.Get(aidataplane.TargetHeader) != previousTarget {
					t.Error("in-flight request mixed configurations")
				}
			} else if request.URL.Path != "/updated" || request.Header.Get(aidataplane.TargetHeader) == previousTarget {
				t.Error("new request did not consume the updated target and path together")
			}

			return &http.Response{StatusCode: 200, Body: http.NoBody, Header: make(http.Header)}, nil
		})
		params := RequestParams{ListenerPort: 8080, HostName: "gateway/team-a/public/listener/api", Method: http.MethodPost, Path: "/mcp", Header: testMCPHeaders(), Body: []byte(testMCPBody)}

		previous, err := service.Forward(t.Context(), params)
		if err != nil {
			t.Fatal(err)
		}

		readResponse(t, previous)
		current, err := service.Forward(t.Context(), params)
		if err != nil {
			t.Fatal(err)
		}

		readResponse(t, current)

		if calls != 2 {
			t.Fatalf("calls=%d, want two independent requests", calls)
		}
	})
}

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	// Service 构造的 bytes.Reader 正文由原生 NopCloser 包装，Close 始终成功。
	defer func() { _ = request.Body.Close() }()

	return f(request)
}

func (b *countedBody) Read([]byte) (int, error) {
	b.reads++

	return 0, io.EOF
}

func (b *countedBody) Close() error {
	b.closes++

	return nil
}

// runTestService 在 synctest 中使用真实 Loader，仅把远端 HTTP 替换为原生 Transport 测试替身。
func runTestService(t *testing.T, config *aidataplane.Config) (*Service, *configtest.Source) {
	t.Helper()
	source := configtest.New(t)
	source.Publish(t, *config)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loader := aiconfig.NewLoader(source.Client, "test", logger)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- loader.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("Run(stopped loader) error = %v, want context.Canceled", err)
		}
	})
	synctest.Wait()

	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}

		if request.URL.Host != "ingate-ai-egress" || request.URL.RawQuery != "" || request.GetBody != nil || len(request.TransferEncoding) != 0 {
			t.Errorf("Forward() outbound request = %s, replay %t, want fixed target without query or replay", request.URL, request.GetBody != nil)
		}

		headers := http.Header{
			"X-Test-Method":   {request.Method},
			"X-Test-Path":     {request.URL.EscapedPath()},
			"X-Test-RawQuery": {request.URL.RawQuery},
			"X-Test-Target":   {request.Header.Get(aidataplane.TargetHeader)},
		}
		for _, name := range []string{"Content-Type", "Accept", "Mcp-Protocol-Version", "Mcp-Method", "Mcp-Name", "Mcp-Param-Region", "Origin", "Authorization", "Cookie", "Connection", "X-Private-Hop", aidataplane.TargetHeader, aidataplane.ListenerPortHeader, aidataplane.HostNameHeader} {
			headers.Set("X-Test-Header-"+name, request.Header.Get(name))
		}

		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)), Header: headers}, nil
	})}

	return New(loader, client), source
}

func rejectUnexpectedRequest(t *testing.T) http.RoundTripper {
	t.Helper()

	return roundTripFunc(func(request *http.Request) (*http.Response, error) {
		t.Errorf("unexpected outbound request %s %s; selection must reject before calling the Client", request.Method, request.URL)
		return nil, errors.New("unexpected outbound request")
	})
}

func readResponse(t *testing.T, response *http.Response) []byte {
	t.Helper()
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("Close(test response) error = %v", err)
		}
	}()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll(test response) error = %v", err)
	}

	return data
}

func fixtureConfig(t *testing.T) *aidataplane.Config {
	t.Helper()
	data, err := os.ReadFile("../../../pkg/aidataplane/testdata/v1.json")
	if err != nil {
		t.Fatalf("ReadFile(configuration fixture) error = %v", err)
	}

	config, err := aidataplane.Decode(data)
	if err != nil {
		t.Fatalf("Decode(configuration fixture) error = %v", err)
	}

	return config
}

func cloneTestConfig(t *testing.T, config *aidataplane.Config) *aidataplane.Config {
	t.Helper()
	data, err := aidataplane.Encode(*config)
	if err != nil {
		t.Fatalf("Encode(test configuration) error = %v", err)
	}

	compiled, err := aidataplane.Decode(data)
	if err != nil {
		t.Fatalf("Decode(test configuration) error = %v", err)
	}

	return compiled
}

func testMCPHeaders() http.Header {
	return http.Header{"Mcp-Protocol-Version": {"2026-07-28"}, "Mcp-Method": {"tools/list"}}
}
