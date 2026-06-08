// Package dataplane 封装插件到 ingate-dataplane 的调用
package dataplane

import (
	"encoding/json"
	"fmt"

	dataplaneratelimit "github.com/lgc202/ingate/pkg/dataplane/ratelimit"
	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/policy"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
)

const defaultTimeoutMillis = 50

// CheckCallback 表示数据面服务返回限流检查结果后的回调
type CheckCallback func(dataplaneratelimit.CheckResponse, error)

// Client 调用 ingate-dataplane
type Client struct {
	config config.DataPlane
}

// New 创建数据面服务 client
func New(config config.DataPlane) Client {
	return Client{config: config}
}

// CheckGlobal 发送 global limit 检查请求
func (c Client) CheckGlobal(redisStores []config.RedisStore, checks []policy.GlobalCheck, callback CheckCallback) error {
	request, err := NewCheckRequest(redisStores, checks)
	if err != nil {
		return err
	}
	return c.check(request, timeoutMillis(c.config, checks), callback)
}

func (c Client) check(request dataplaneratelimit.CheckRequest, timeoutMillis int, callback CheckCallback) error {
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshal rate limit dataplane request: %w", err)
	}

	_, err = proxywasm.DispatchHttpCall(
		c.config.ClusterName,
		[][2]string{
			{":method", "POST"},
			{":path", c.config.Path},
			{":authority", c.config.ClusterName},
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

func timeoutMillis(config config.DataPlane, checks []policy.GlobalCheck) int {
	timeout := defaultTimeoutMillis
	timeout = max(timeout, config.TimeoutMillis)
	for _, check := range checks {
		timeout = max(timeout, check.RedisTimeoutMs)
	}
	return timeout
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
