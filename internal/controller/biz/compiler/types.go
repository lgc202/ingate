// Package compiler 将声明式资源直接编译为 Envoy xDS 配置
package compiler

import (
	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"k8s.io/apimachinery/pkg/types"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
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
type Reason = gatewayv1.ConditionReason

const (
	// ReasonInvalidSpec 表示资源字段不满足编译要求
	ReasonInvalidSpec = gatewayv1.ReasonInvalidSpec
	// ReasonReferenceNotFound 表示资源引用的目标不存在
	ReasonReferenceNotFound = gatewayv1.ReasonReferenceNotFound
	// ReasonInvalidReference 表示资源引用的目标存在但不可用
	ReasonInvalidReference = gatewayv1.ReasonInvalidReference
	// ReasonConflict 表示资源与同一配置域内的其他资源冲突
	ReasonConflict = gatewayv1.ReasonConflict
	// ReasonUnsupported 表示当前 Envoy 配置链路尚不支持该能力
	ReasonUnsupported = gatewayv1.ReasonUnsupported
	// ReasonCompileFailed 表示资源通过校验后仍无法生成一致的 Envoy 配置
	ReasonCompileFailed = gatewayv1.ReasonCompileFailed
)

// ResourceGeneration 标识一次编译所观察到的资源身份和 spec 版本
//
// UID 用于隔离删除后同名重建的资源，Generation 用于避免旧配置结果覆盖新 spec 状态
type ResourceGeneration struct {
	Kind       gatewayv1.Kind
	Name       string
	UID        types.UID
	Generation int64
}

// CompiledPolicyTarget 标识一条策略实际展开到 Envoy 配置中的作用目标
type CompiledPolicyTarget struct {
	Policy ResourceGeneration
	Target ResourceGeneration
}

// Resources 表示一次全量编译使用的声明式资源
//
// 使用 gateway/v1 指针可以直接接收 informer 和 generated client 的结果，
// 同时避免复制带有 ObjectMeta 和切片字段的 Kubernetes 资源值
type Resources struct {
	Gateways              []*gatewayv1.Gateway
	Certificates          []*gatewayv1.Certificate
	Routes                []*gatewayv1.Route
	Upstreams             []*gatewayv1.Upstream
	RateLimitPolicies     []*gatewayv1.RateLimitPolicy
	IPRestrictionPolicies []*gatewayv1.IPRestrictionPolicy
}

// Generations 返回当前资源集合中所有非 nil 资源的身份和 spec 版本
func (r Resources) Generations() []ResourceGeneration {
	result := make([]ResourceGeneration, 0,
		len(r.Gateways)+len(r.Certificates)+len(r.Routes)+len(r.Upstreams)+
			len(r.RateLimitPolicies)+len(r.IPRestrictionPolicies),
	)
	for _, resource := range r.Gateways {
		if resource != nil {
			result = append(result, newResourceGeneration(gatewayv1.KindGateway, resource.Name, resource.UID, resource.Generation))
		}
	}
	for _, resource := range r.Certificates {
		if resource != nil {
			result = append(result, newResourceGeneration(gatewayv1.KindCertificate, resource.Name, resource.UID, resource.Generation))
		}
	}
	for _, resource := range r.Routes {
		if resource != nil {
			result = append(result, newResourceGeneration(gatewayv1.KindRoute, resource.Name, resource.UID, resource.Generation))
		}
	}
	for _, resource := range r.Upstreams {
		if resource != nil {
			result = append(result, newResourceGeneration(gatewayv1.KindUpstream, resource.Name, resource.UID, resource.Generation))
		}
	}
	for _, resource := range r.RateLimitPolicies {
		if resource != nil {
			result = append(result, newResourceGeneration(gatewayv1.KindRateLimitPolicy, resource.Name, resource.UID, resource.Generation))
		}
	}
	for _, resource := range r.IPRestrictionPolicies {
		if resource != nil {
			result = append(result, newResourceGeneration(gatewayv1.KindIPRestrictionPolicy, resource.Name, resource.UID, resource.Generation))
		}
	}
	return result
}

// Diagnostic 描述一个资源在当前配置域中的编译结果
type Diagnostic struct {
	Severity Severity
	Kind     gatewayv1.Kind
	ID       string
	Reason   Reason
	Message  string
}

// EnvoyConfig 保存编译完成的四类 Envoy 动态资源
type EnvoyConfig struct {
	Listeners []*listenerv3.Listener
	Routes    []*routev3.RouteConfiguration
	Clusters  []*clusterv3.Cluster
	Endpoints []*endpointv3.ClusterLoadAssignment
}

// Result 表示一次全量编译的结果
//
// 任意 Error diagnostic 都会使 Version、Config 和 PolicyTargets 为空，ResourceGenerations 仍保留本次输入来源
type Result struct {
	Version             string
	Config              EnvoyConfig
	ResourceGenerations []ResourceGeneration
	PolicyTargets       []CompiledPolicyTarget
	Diagnostics         []Diagnostic
}

// HasErrors 返回结果是否包含阻塞发布的诊断
func (r Result) HasErrors() bool {
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Severity == SeverityError {
			return true
		}
	}
	return false
}

func newResourceGeneration(kind gatewayv1.Kind, name string, uid types.UID, generation int64) ResourceGeneration {
	return ResourceGeneration{
		Kind:       kind,
		Name:       name,
		UID:        uid,
		Generation: generation,
	}
}
