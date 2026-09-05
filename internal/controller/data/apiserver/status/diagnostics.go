package status

import (
	"cmp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/controller/biz/compiler"
	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

type diagnosticIndex struct {
	specific map[resourceKey][]compiler.Diagnostic
	kinds    map[gatewayv1.Kind][]compiler.Diagnostic
	global   []compiler.Diagnostic
}

// newDiagnosticIndex 按资源、资源类型和全局三个作用域关联编译诊断。
func newDiagnosticIndex(
	resources []compiler.ResourceGeneration,
	diagnostics []compiler.Diagnostic,
) diagnosticIndex {
	knownResources := make(map[resourceKey]bool, len(resources))
	knownKinds := make(map[gatewayv1.Kind]bool)
	for _, resource := range resources {
		knownResources[resourceKey{kind: resource.Kind, name: resource.Name}] = true
		knownKinds[resource.Kind] = true
	}

	index := diagnosticIndex{
		specific: make(map[resourceKey][]compiler.Diagnostic),
		kinds:    make(map[gatewayv1.Kind][]compiler.Diagnostic),
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity != compiler.SeverityError && diagnostic.Severity != compiler.SeverityWarning {
			continue
		}
		key := resourceKey{kind: diagnostic.Kind, name: diagnostic.ResourceID}
		switch {
		case diagnostic.Kind == "" || !knownKinds[diagnostic.Kind]:
			index.global = append(index.global, diagnostic)
		case diagnostic.ResourceID == "" || !knownResources[key]:
			index.kinds[diagnostic.Kind] = append(index.kinds[diagnostic.Kind], diagnostic)
		default:
			index.specific[key] = append(index.specific[key], diagnostic)
		}
	}
	return index
}

func (i diagnosticIndex) forResource(kind gatewayv1.Kind, name string) compileDecision {
	diagnostics := make([]compiler.Diagnostic, 0,
		len(i.global)+len(i.kinds[kind])+len(i.specific[resourceKey{kind: kind, name: name}]),
	)
	diagnostics = append(diagnostics, i.global...)
	diagnostics = append(diagnostics, i.kinds[kind]...)
	diagnostics = append(diagnostics, i.specific[resourceKey{kind: kind, name: name}]...)

	decision := compileDecision{
		accepted: conditionDecision{
			status:  metav1.ConditionTrue,
			reason:  gatewayv1.ReasonAccepted,
			message: messageAccepted,
		},
	}
	if kindHasReferences(kind) {
		resolvedRefs := conditionDecision{
			status:  metav1.ConditionTrue,
			reason:  gatewayv1.ReasonResolvedRefs,
			message: messageResolvedRefs,
		}
		decision.resolvedRefs = &resolvedRefs
	}

	for _, diagnostic := range diagnostics {
		if decision.resolvedRefs != nil && isReferenceReason(diagnostic.Reason) {
			if decision.resolvedRefs.status == metav1.ConditionTrue {
				*decision.resolvedRefs = decisionFromDiagnostic(diagnostic)
			}
			continue
		}
		if diagnostic.Severity == compiler.SeverityWarning {
			if decision.accepted.message == messageAccepted && diagnostic.Message != "" {
				decision.accepted.message = diagnostic.Message
			}
			continue
		}
		if decision.accepted.status == metav1.ConditionTrue {
			decision.accepted = decisionFromDiagnostic(diagnostic)
		}
	}
	return decision
}

// forWasmPlugin 不继承 Envoy 配置的全局诊断，
// 插件安装只由制品自身的拉取、校验和包冲突决定。
func (i diagnosticIndex) forWasmPlugin(name string) compileDecision {
	diagnostics := make([]compiler.Diagnostic, 0,
		len(i.kinds[gatewayv1.KindWasmPlugin])+len(i.specific[resourceKey{kind: gatewayv1.KindWasmPlugin, name: name}]),
	)
	diagnostics = append(diagnostics, i.kinds[gatewayv1.KindWasmPlugin]...)
	diagnostics = append(diagnostics, i.specific[resourceKey{kind: gatewayv1.KindWasmPlugin, name: name}]...)

	decision := compileDecision{accepted: conditionDecision{
		status:  metav1.ConditionTrue,
		reason:  gatewayv1.ReasonAccepted,
		message: messageAccepted,
	}}
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == compiler.SeverityWarning {
			if decision.accepted.message == messageAccepted && diagnostic.Message != "" {
				decision.accepted.message = diagnostic.Message
			}
			continue
		}
		if decision.accepted.status == metav1.ConditionTrue {
			decision.accepted = decisionFromDiagnostic(diagnostic)
		}
	}
	return decision
}

func decisionFromDiagnostic(diagnostic compiler.Diagnostic) conditionDecision {
	reason := cmp.Or(diagnostic.Reason, gatewayv1.ReasonCompileFailed)
	message := diagnostic.Message
	if message == "" || diagnostic.Kind == "" {
		message = messageCompileFailed
	}
	return conditionDecision{
		status:  metav1.ConditionFalse,
		reason:  reason,
		message: message,
	}
}

func isReferenceReason(reason compiler.Reason) bool {
	return reason == compiler.ReasonReferenceNotFound ||
		reason == compiler.ReasonPluginNotInstalled ||
		reason == compiler.ReasonInvalidReference ||
		reason == compiler.ReasonArtifactUnavailable
}
