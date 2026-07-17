// Package config 将声明式 Gateway 资源直接编译为 Envoy xDS 配置
package config

import (
	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Severity 表示编译诊断的严重程度
type Severity string

const (
	// SeverityError 表示配置不可发布
	SeverityError Severity = "Error"
	// SeverityWarning 表示配置仍可发布，但存在需要处理的问题
	SeverityWarning Severity = "Warning"
)

// Reason 表示资源编译状态的稳定原因
type Reason string

const (
	// ReasonAccepted 表示资源已被编译器接受
	ReasonAccepted Reason = "Accepted"
	// ReasonInvalidSpec 表示资源字段不满足编译要求
	ReasonInvalidSpec Reason = "InvalidSpec"
	// ReasonReferenceNotFound 表示资源引用的目标不存在
	ReasonReferenceNotFound Reason = "ReferenceNotFound"
	// ReasonConflict 表示资源与同一配置域内的其他资源冲突
	ReasonConflict Reason = "Conflict"
	// ReasonUnsupported 表示当前 Envoy 配置链路尚不支持该能力
	ReasonUnsupported Reason = "Unsupported"
	// ReasonCompileFailed 表示资源通过校验后仍无法生成一致的 Envoy 配置
	ReasonCompileFailed Reason = "CompileFailed"
)

// ResourceSet 表示一次编译使用的完整声明式资源集合
//
// 使用 gateway/v1 指针可以直接接收 informer 和 generated client 的结果，
// 同时避免复制带有 ObjectMeta 和切片字段的 Kubernetes 资源值
type ResourceSet struct {
	Gateways              []*gatewayv1.Gateway
	Routes                []*gatewayv1.Route
	Upstreams             []*gatewayv1.Upstream
	RateLimitPolicies     []*gatewayv1.RateLimitPolicy
	AccessControlPolicies []*gatewayv1.AccessControlPolicy
	PolicyBindings        []*gatewayv1.PolicyBinding
}

// Diagnostic 描述一个资源在当前配置域中的编译结果
type Diagnostic struct {
	Severity Severity
	Kind     gatewayv1.Kind
	ID       string
	Reason   Reason
	Message  string
}

// Config 保存可以直接交给 xDS Snapshot Cache 的四类 Envoy 资源
type Config struct {
	Listeners []*listenerv3.Listener
	Routes    []*routev3.RouteConfiguration
	Clusters  []*clusterv3.Cluster
	Endpoints []*endpointv3.ClusterLoadAssignment
}

// CompileResult 表示一次全量编译的结果
//
// 任意 Error diagnostic 都会使 Version 为空并清空 Config，调用方不得发布该结果
type CompileResult struct {
	Version     string
	Config      Config
	Diagnostics []Diagnostic
}

// Compiler 将完整资源集合直接编译为 Envoy protobuf
type Compiler struct{}

// HasErrors 返回结果是否包含阻塞发布的诊断
func (r CompileResult) HasErrors() bool {
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Severity == SeverityError {
			return true
		}
	}
	return false
}
