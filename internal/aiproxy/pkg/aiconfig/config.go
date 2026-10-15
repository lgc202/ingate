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
)

// Config 是一份已校验且不可修改的运行配置，可由多个请求共同持有。
// 只能通过 newConfig 创建；更新时替换整个实例，不修改已发布实例。
//
// Go 没有只读对象限定符，这里依靠封装和使用约定：字段不导出，查询方法只读，
// 结果不携带内部可变引用。调用方不得通过返回的指针覆盖整个 Config 值；
// 本包包括 Loader 在内的所有实现均不得在发布后修改字段、写入 map，或在查询中执行懒初始化。
// Loader 只保护实例指针的交接，已捕获实例的并发读取依赖这些约束。
type Config struct {
	config   *aidataplane.Config
	hosts    map[hostKey]aidataplane.Host
	backends map[string]ModelBackend
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

type hostKey struct {
	port int32
	name string
}

// newConfig 解码完整编译配置，并构建请求使用的查找表。
// 返回值独立于 data；失败不返回部分结果，也不记录日志。
func newConfig(data []byte) (*Config, error) {
	config, err := aidataplane.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("decode AI runtime configuration: %w", err)
	}

	// Decode 创建本次构建独有的配置；索引可以引用其字段，因为后续不再修改或向外暴露它们。
	// 全部索引构建完成才交给 Loader 发布，不能把仍在填充的 map 交给请求读取。
	hosts := make(map[hostKey]aidataplane.Host)
	for _, listener := range config.Listeners {
		for _, host := range listener.Hosts {
			hosts[hostKey{port: listener.Port, name: host.Name}] = host
		}
	}
	backends := make(map[string]ModelBackend, len(config.Backends))
	networks := make(map[string]aidataplane.NetworkBackend, len(config.NetworkBackends))
	for _, network := range config.NetworkBackends {
		networks[network.Name] = network
	}
	for _, backend := range config.Backends {
		target, err := aidataplane.ModelTargetName(backend, networks[backend.NetworkBackendName])
		if err != nil {
			return nil, fmt.Errorf("build model target %q: %w", backend.Name, err)
		}

		backends[backend.Name] = ModelBackend{
			Name:               backend.Name,
			Schema:             backend.Schema,
			ModelNameOverride:  *backend.ModelNameOverride,
			NetworkBackendName: backend.NetworkBackendName,
			TargetName:         target,
		}
	}

	return &Config{config: config, hosts: hosts, backends: backends}, nil
}

// Revision 返回本实例已校验的内容版本，不表示 Envoy 已加载同一版本。
func (c *Config) Revision() string {
	return c.config.Revision
}

// ResolveModel 根据入口和请求模型名，查出应使用的上游协议、模型名覆盖值与网络后端标识。
//
// 调用方须先确定可信的入口：port 对应 listeners[].port，hostName 对应该端口下
// hosts[].name 的完整编译标识，不是公网域名或客户端 Host Header。
// modelName 是调用方从请求体顶层 model 字段提取的原始模型名，尚未应用模型名覆盖；
// 不能用客户端自报的 x-ai-eg-model Header 代替它。
//
// 本方法在该入口的 modelRoutes 中按编译顺序寻找首个匹配：有条件时精确比较模型名，
// 空条件匹配任意模型。命中后按 backendName 查协议后端，返回其 schema、覆盖值与网络引用。
// 没有匹配返回 ErrModelRouteNotFound；首个命中的后端为空则返回 ErrModelBackendUnavailable，
// 不继续尝试后面的候选，也不跨入口回退。
//
// 例如已有示例中，8080 / gateway/team-a/public/listener/api 入口收到 model=coding，
// 返回 OpenAI 协议、前缀 /v1、覆盖值 code-v2，以及网络后端 team-a/model-network。
// 这里只选择目标，不解析或改写 HTTP 请求、不连接上游，也不重新读取领域资源。
func (c *Config) ResolveModel(port int32, hostName, modelName string) (ModelBackend, error) {
	for _, route := range c.hosts[hostKey{port: port, name: hostName}].ModelRoutes {
		if len(route.Headers) != 0 && route.Headers[0].Value != modelName {
			continue
		}
		if *route.BackendName == "" {
			return ModelBackend{}, fmt.Errorf("route %q: %w", route.Name, ErrModelBackendUnavailable)
		}

		backend := c.backends[*route.BackendName]
		backend.RouteName = route.Name

		return backend, nil
	}

	return ModelBackend{}, fmt.Errorf("port %d host %q: %w", port, hostName, ErrModelRouteNotFound)
}
