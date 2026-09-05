// Package compiler 将声明式资源直接编译为 Envoy xDS 配置。
package compiler

import (
	"slices"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

const (
	// ReasonInvalidSpec 表示资源字段不满足编译要求。
	ReasonInvalidSpec = gatewayv1.ReasonInvalidSpec
	// ReasonReferenceNotFound 表示资源引用的目标不存在。
	ReasonReferenceNotFound = gatewayv1.ReasonReferenceNotFound
	// ReasonPluginNotInstalled 表示策略依赖的数据面插件尚未安装。
	ReasonPluginNotInstalled = gatewayv1.ReasonPluginNotInstalled
	// ReasonInvalidReference 表示资源引用的目标存在但不可用。
	ReasonInvalidReference = gatewayv1.ReasonInvalidReference
	// ReasonConflict 表示资源与同一配置域内的其他资源冲突。
	ReasonConflict = gatewayv1.ReasonConflict
	// ReasonUnsupported 表示当前 Envoy 配置链路尚不支持该能力。
	ReasonUnsupported = gatewayv1.ReasonUnsupported
	// ReasonCompileFailed 表示资源通过校验后仍无法生成一致的 Envoy 配置。
	ReasonCompileFailed = gatewayv1.ReasonCompileFailed
	// ReasonArtifactUnavailable 表示插件制品无法下载或校验。
	ReasonArtifactUnavailable = gatewayv1.ReasonArtifactUnavailable
)

// Severity 表示编译诊断的严重程度。
type Severity string

const (
	// SeverityError 表示配置不可发布。
	SeverityError Severity = "Error"
	// SeverityWarning 表示配置仍可发布，但存在需要处理的问题。
	SeverityWarning Severity = "Warning"
)

// Reason 表示资源编译状态的稳定原因。
type Reason = gatewayv1.ConditionReason

// ResourceGeneration 标识一次编译所观察到的资源身份和 spec 版本。
//
// UID 用于隔离删除后同名重建的资源，Generation 用于避免旧配置结果覆盖新 spec 状态。
type ResourceGeneration struct {
	Kind       gatewayv1.Kind
	Name       string
	UID        types.UID
	Generation int64
}

// CompiledPolicyTarget 标识一条策略实际展开到 Envoy 配置中的作用目标。
type CompiledPolicyTarget struct {
	Policy ResourceGeneration
	Target ResourceGeneration
}

// WasmModule 描述 Controller 已下载并校验、可供 Envoy 直接读取的内容寻址文件。
type WasmModule struct {
	Path   string
	SHA256 string
}

// Resources 表示一次全量编译使用的声明式资源。
//
// 使用 gateway/v1 指针可以直接接收 informer 和 generated client 的结果，
// 同时避免复制带有 ObjectMeta 和切片字段的 Kubernetes 资源值。
type Resources struct {
	Gateways                     []*gatewayv1.Gateway
	Certificates                 []*gatewayv1.Certificate
	Routes                       []*gatewayv1.Route
	Upstreams                    []*gatewayv1.Upstream
	RateLimitPolicies            []*gatewayv1.RateLimitPolicy
	IPRestrictionPolicies        []*gatewayv1.IPRestrictionPolicy
	HeaderTransformationPolicies []*gatewayv1.HeaderTransformationPolicy
	MockResponsePolicies         []*gatewayv1.MockResponsePolicy
	WasmPlugins                  []*gatewayv1.WasmPlugin
}

// Diagnostic 描述一个资源在当前配置域中的编译结果。
// Kind 为空时诊断作用于整个配置域；ResourceID 为空时诊断作用于该 Kind 的全部资源。
type Diagnostic struct {
	Severity   Severity
	Kind       gatewayv1.Kind
	ResourceID string
	Reason     Reason
	Message    string
}

// EnvoyConfig 保存编译完成的四类 Envoy 动态资源。
type EnvoyConfig struct {
	Listeners []*listenerv3.Listener
	Routes    []*routev3.RouteConfiguration
	Clusters  []*clusterv3.Cluster
	Endpoints []*endpointv3.ClusterLoadAssignment
}

// Result 表示一次全量编译的结果。
//
// 任意 Error diagnostic 都会使 Version、Config 和 PolicyTargets 为空，
// ResourceGenerations 仍保留本次输入来源。
type Result struct {
	Version             string
	Config              EnvoyConfig
	ResourceGenerations []ResourceGeneration
	PolicyTargets       []CompiledPolicyTarget
	Diagnostics         []Diagnostic
}

// Generations 返回当前资源集合中所有非 nil 资源的身份和 spec 版本。
func (r Resources) Generations() []ResourceGeneration {
	result := make([]ResourceGeneration, 0,
		len(r.Gateways)+len(r.Certificates)+len(r.Routes)+len(r.Upstreams)+
			len(r.RateLimitPolicies)+len(r.IPRestrictionPolicies)+len(r.HeaderTransformationPolicies)+
			len(r.MockResponsePolicies)+len(r.WasmPlugins),
	)
	for _, resource := range r.Gateways {
		if resource != nil {
			result = append(result, newResourceGeneration(gatewayv1.KindGateway, resource))
		}
	}
	for _, resource := range r.Certificates {
		if resource != nil {
			result = append(result, newResourceGeneration(gatewayv1.KindCertificate, resource))
		}
	}
	for _, resource := range r.Routes {
		if resource != nil {
			result = append(result, newResourceGeneration(gatewayv1.KindRoute, resource))
		}
	}
	for _, resource := range r.Upstreams {
		if resource != nil {
			result = append(result, newResourceGeneration(gatewayv1.KindUpstream, resource))
		}
	}
	for _, resource := range r.RateLimitPolicies {
		if resource != nil {
			result = append(result, newResourceGeneration(gatewayv1.KindRateLimitPolicy, resource))
		}
	}
	for _, resource := range r.IPRestrictionPolicies {
		if resource != nil {
			result = append(result, newResourceGeneration(gatewayv1.KindIPRestrictionPolicy, resource))
		}
	}
	for _, resource := range r.HeaderTransformationPolicies {
		if resource != nil {
			result = append(result, newResourceGeneration(
				gatewayv1.KindHeaderTransformationPolicy,
				resource,
			))
		}
	}
	for _, resource := range r.MockResponsePolicies {
		if resource != nil {
			result = append(result, newResourceGeneration(gatewayv1.KindMockResponsePolicy, resource))
		}
	}
	for _, resource := range r.WasmPlugins {
		if resource != nil {
			result = append(result, newResourceGeneration(gatewayv1.KindWasmPlugin, resource))
		}
	}
	slices.SortFunc(result, compareResourceGeneration)
	return result
}

// HasErrors 返回结果是否包含阻塞发布的诊断。
func (r Result) HasErrors() bool {
	return slices.ContainsFunc(r.Diagnostics, func(diagnostic Diagnostic) bool {
		return diagnostic.Severity == SeverityError
	})
}

func newResourceGeneration(kind gatewayv1.Kind, resource metav1.Object) ResourceGeneration {
	return ResourceGeneration{
		Kind:       kind,
		Name:       resource.GetName(),
		UID:        resource.GetUID(),
		Generation: resource.GetGeneration(),
	}
}
