package registry

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
)

// ValidatePolicyTargetRefs 校验具体策略允许的目标类型、名称和重复引用
func ValidatePolicyTargetRefs(
	refs []resource.PolicyTargetRef,
	path *field.Path,
	allowedKinds ...resource.Kind,
) field.ErrorList {
	errs := field.ErrorList{}
	seen := make(map[resource.PolicyTargetRef]bool, len(refs))
	for i, ref := range refs {
		refPath := path.Index(i)
		if !kindAllowed(ref.Kind, allowedKinds) {
			supported := make([]string, 0, len(allowedKinds))
			for _, kind := range allowedKinds {
				supported = append(supported, string(kind))
			}
			errs = append(errs, field.NotSupported(refPath.Child("kind"), ref.Kind, supported))
		}
		if ref.Name == "" {
			errs = append(errs, field.Required(refPath.Child("name"), "target name is required"))
		}
		if seen[ref] {
			errs = append(errs, field.Duplicate(refPath, ref))
			continue
		}
		seen[ref] = true
	}
	return errs
}

func kindAllowed(kind resource.Kind, allowed []resource.Kind) bool {
	for _, candidate := range allowed {
		if kind == candidate {
			return true
		}
	}
	return false
}
