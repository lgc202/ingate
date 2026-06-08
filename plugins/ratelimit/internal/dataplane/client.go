// Package dataplane 封装插件到 ingate-dataplane 的调用
package dataplane

import (
	dataplaneratelimit "github.com/lgc202/ingate/pkg/dataplane/ratelimit"
	config "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	"github.com/lgc202/ingate/plugins/ratelimit/internal/policy"
)

const defaultTimeoutMillis = 50

// CheckCallback 表示数据面服务返回限流检查结果后的回调
type CheckCallback func(dataplaneratelimit.CheckResponse, error)

// Transport 表示插件访问 ingate-dataplane 的传输通道
//
// 当前实现使用 Proxy-Wasm 标准 dispatch_http_call，不需要定制 Envoy。
// 未来如果 Ingate 维护自己的数据面发行版，可以在这里替换为 hostcall transport，
// RateLimit runtime 和 policy 不应该感知具体传输方式。
type Transport interface {
	CheckRateLimit(request dataplaneratelimit.CheckRequest, timeoutMillis int, callback CheckCallback) error
}

// Client 调用 ingate-dataplane 提供的运行时能力
type Client struct {
	baseTimeoutMillis int
	transport         Transport
}

// New 创建数据面服务 client
func New(config config.DataPlane) Client {
	return Client{
		baseTimeoutMillis: config.TimeoutMillis,
		transport:         NewHTTPTransport(config),
	}
}

// CheckGlobal 发送 global limit 检查请求
func (c Client) CheckGlobal(redisStores []config.RedisStore, checks []policy.GlobalCheck, callback CheckCallback) error {
	request, err := NewCheckRequest(redisStores, checks)
	if err != nil {
		return err
	}
	return c.transport.CheckRateLimit(request, timeoutMillis(c.baseTimeoutMillis, checks), callback)
}

func timeoutMillis(baseTimeoutMillis int, checks []policy.GlobalCheck) int {
	timeout := defaultTimeoutMillis
	timeout = max(timeout, baseTimeoutMillis)
	for _, check := range checks {
		timeout = max(timeout, check.RedisTimeoutMs)
	}
	return timeout
}
