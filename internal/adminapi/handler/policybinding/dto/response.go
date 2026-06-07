package dto

import (
	"time"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	policybindingservice "github.com/lgc202/ingate/internal/adminapi/service/policybinding"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// NewListPolicyBindingsResp 转换策略绑定列表用例结果为控制台响应
func NewListPolicyBindingsResp(result *policybindingservice.ListResult) ListPolicyBindingsResp {
	return ListPolicyBindingsResp{
		Bindings: lo.Map(result.Bindings, func(binding resource.PolicyBinding, _ int) PolicyBinding {
			return bindingFromResource(&binding)
		}),
	}
}

// NewGetPolicyBindingResp 转换单个策略绑定用例结果为控制台响应
func NewGetPolicyBindingResp(result *policybindingservice.BindingResult) PolicyBinding {
	return bindingFromResource(result.Binding)
}

func bindingFromResource(binding *resource.PolicyBinding) PolicyBinding {
	return PolicyBinding{
		ID:      binding.Name,
		Version: binding.ResourceVersion,
		PolicyBindingConfig: PolicyBindingConfig{
			Name:        binding.Spec.DisplayName,
			Description: binding.Spec.Description,
			Enabled:     binding.Spec.Enabled,
			TargetRef:   binding.Spec.TargetRef,
			Policies:    binding.Spec.Policies,
		},
		CreatedAt: createdAt(binding.ObjectMeta),
	}
}

func createdAt(metadata metav1.ObjectMeta) string {
	if metadata.CreationTimestamp.IsZero() {
		return ""
	}
	return metadata.CreationTimestamp.UTC().Format(time.RFC3339)
}
