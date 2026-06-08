package dataplane

import (
	"encoding/json"
	"fmt"

	dataplaneratelimit "github.com/lgc202/ingate/pkg/dataplane/ratelimit"
	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
)

// HTTPTransport 通过 Proxy-Wasm 标准 HTTP call 访问 ingate-dataplane
//
// 这里保持 HTTP/JSON 只是当前 transport 实现细节。dataplane 作为插件运行时能力层
// 仍然是稳定产品边界，后续可以把该实现替换为 unary gRPC over dispatch_http_call，
// 或在自有 Envoy 发行版中替换为 hostcall。
type HTTPTransport struct {
	config config.DataPlane
}

// NewHTTPTransport 创建基于 dispatch_http_call 的 dataplane transport
func NewHTTPTransport(config config.DataPlane) HTTPTransport {
	return HTTPTransport{config: config}
}

// CheckRateLimit 发送限流检查请求
func (t HTTPTransport) CheckRateLimit(request dataplaneratelimit.CheckRequest, timeoutMillis int, callback CheckCallback) error {
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshal rate limit dataplane request: %w", err)
	}

	_, err = proxywasm.DispatchHttpCall(
		t.config.ClusterName,
		[][2]string{
			{":method", "POST"},
			{":path", t.config.Path},
			{":authority", t.config.ClusterName},
			{"content-type", "application/json"},
		},
		body,
		nil,
		uint32(timeoutMillis),
		func(numHeaders, bodySize, numTrailers int) {
			callback(parseCheckResponse(bodySize))
		},
	)
	if err != nil {
		return fmt.Errorf("dispatch rate limit dataplane request: %w", err)
	}
	return nil
}

func parseCheckResponse(bodySize int) (dataplaneratelimit.CheckResponse, error) {
	if status := responseStatus(); status != 200 {
		return dataplaneratelimit.CheckResponse{}, fmt.Errorf("dataplane returned status %d", status)
	}

	body, err := proxywasm.GetHttpCallResponseBody(0, bodySize)
	if err != nil {
		return dataplaneratelimit.CheckResponse{}, fmt.Errorf("read dataplane response: %w", err)
	}
	var response dataplaneratelimit.CheckResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return dataplaneratelimit.CheckResponse{}, fmt.Errorf("parse dataplane response: %w", err)
	}
	return response, nil
}

func responseStatus() int {
	headers, err := proxywasm.GetHttpCallResponseHeaders()
	if err != nil {
		return 0
	}
	return responseStatusFromHeaders(headers)
}

func responseStatusFromHeaders(headers [][2]string) int {
	for _, header := range headers {
		if header[0] != ":status" {
			continue
		}
		var status int
		for _, ch := range header[1] {
			if ch < '0' || ch > '9' {
				return 0
			}
			status = status*10 + int(ch-'0')
		}
		return status
	}
	return 0
}
