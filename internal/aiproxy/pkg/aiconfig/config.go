package aiconfig

import (
	"errors"
	"fmt"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

var (
	// ErrModelRouteNotFound 表示指定 Listener 内没有命中的模型路由。
	ErrModelRouteNotFound = errors.New("model route not found")
	// ErrModelBackendUnavailable 表示首个命中路由的后端不可用，禁止跳到下一候选。
	ErrModelBackendUnavailable = errors.New("model backend unavailable")
	// ErrMCPRouteNotFound 表示指定入口没有路径完全相等的 MCP 路由。
	ErrMCPRouteNotFound = errors.New("MCP route not found")
	// ErrMCPBackendUnavailable 表示命中的 MCP 路由后端不可用，禁止回退到其他入口。
	ErrMCPBackendUnavailable = errors.New("MCP backend unavailable")
)

// Config 是已接受资源生成的不可变查询视图，可由多个在途请求共同持有。
// 更新只替换实例，不修改已发布的入口 map 或 entryConfig；查询结果不携带可变引用。
// Loader 保护指针交接，解锁后的安全性依赖此只读约束，不是继续持有 Loader 的锁。
type Config struct {
	entries map[hostKey]*entryConfig
}

// ModelBackend 是一次模型匹配的完整协议与网络绑定，不包含可变引用或网络端点。
// Schema 与 ModelNameOverride 用于出站协议处理，网络连接仍由 Envoy 管理。
type ModelBackend struct {
	RouteName string
	// Name 是规则级协议后端的编译标识，不是 AIBackend 资源名称。
	Name   string
	Schema aidataplane.APISchema
	// ModelNameOverride 非空时是出站请求应使用的模型名；为空时保留请求原模型名。
	// 这里只交付覆盖值，实际请求体改写由模型请求处理方完成。
	ModelNameOverride  string
	NetworkBackendName string
	// TargetName 同时绑定协议参数和网络快照，只能交给专用 Envoy 出口。
	TargetName string
}

// MCPBackend 是一次 MCP 精确路径匹配的网络与上游路径绑定，不包含会话或工具状态。
type MCPBackend struct {
	RouteName string
	// NetworkBackendName 是 Backend 的编译标识，不是领域资源名称。
	NetworkBackendName string
	// BackendPath 是已默认化的上游路径；Go 只拼接这一条固定路径，不从请求派生目标。
	BackendPath string
	// TargetName 绑定网络快照，只能交给专用 Envoy 出口。
	TargetName string
}

type hostKey struct {
	port int32
	name string
}

// ResolveModel 根据入口和请求模型名，查出应使用的上游协议、模型名覆盖值与网络后端标识。
//
// 调用方须先确定可信的入口：port 对应 Entry.port，hostName 对应 Entry.host_name
// 的完整编译标识，不是公网域名或客户端 Host Header。
// modelName 是调用方从请求体顶层 model 字段提取的原始模型名，尚未应用模型名覆盖；
// 不能用客户端自报的 x-ai-eg-model Header 代替它。
//
// 本方法按精确模型优先、来源创建时间、Namespace、名称、规则与匹配位置寻找首个候选，
// catch-all 匹配任意模型。匹配后交付规则携带的协议、覆盖值与受保护的出口。
// 没有匹配返回 ErrModelRouteNotFound；首个命中为 unavailable 则返回 ErrModelBackendUnavailable，
// 不继续尝试后面的候选，也不跨入口回退。
//
// 例如已有示例中，8080 / gateway/team-a/public/listener/api 入口收到 model=coding，
// 返回 OpenAI 协议、前缀 /v1、覆盖值 code-v2，以及网络后端 team-a/model-network。
// 这里只选择目标，不解析或改写 HTTP 请求、不连接上游，也不重新读取领域资源。
func (c *Config) ResolveModel(port int32, hostName, modelName string) (ModelBackend, error) {
	entry := c.entries[hostKey{port: port, name: hostName}]
	if entry != nil {
		for _, candidate := range entry.models {
			if !candidate.all && candidate.model != modelName {
				continue
			}

			if candidate.unavailable {
				return ModelBackend{}, fmt.Errorf("route %q: %w", candidate.backend.RouteName, ErrModelBackendUnavailable)
			}

			return candidate.backend, nil
		}
	}

	return ModelBackend{}, fmt.Errorf("port %d host %q: %w", port, hostName, ErrModelRouteNotFound)
}

// ResolveMCP 根据可信入口和请求路径，查出应使用的网络后端标识与固定上游路径。
//
// 调用方须先确定可信的入口（port、hostName）与请求路径；path 是 HTTP 请求 URL 的
// 路径部分，不是客户端自报的 Header 或 Host。本方法在入口索引中查找完全相等（含尾斜杠）
// 的路径，构建时已按创建时间、Namespace、name 决定同路径的首个候选。
// 命中后 unavailable 表示后端失效，返回 ErrMCPBackendUnavailable，不回退到后面的
// 候选或普通 HTTP。没有匹配返回 ErrMCPRouteNotFound。
// 这里只选择目标与已默认化的上游路径，不解析或改写 MCP 消息、不连接上游。
func (c *Config) ResolveMCP(port int32, hostName, path string) (MCPBackend, error) {
	entry := c.entries[hostKey{port: port, name: hostName}]
	if entry != nil {
		if candidate, exists := entry.mcp[path]; exists {
			if candidate.unavailable {
				return MCPBackend{}, fmt.Errorf("route %q: %w", candidate.backend.RouteName, ErrMCPBackendUnavailable)
			}

			return candidate.backend, nil
		}
	}

	return MCPBackend{}, fmt.Errorf("port %d host %q path %q: %w", port, hostName, path, ErrMCPRouteNotFound)
}
