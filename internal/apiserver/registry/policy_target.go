package registry

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
)

// ValidatePolicyTargetRefs 校验策略目标类型、名称和重复引用
func ValidatePolicyTargetRefs(refs []resource.PolicyTargetRef, path *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	seen := make(map[resource.PolicyTargetRef]bool, len(refs))
	for i, ref := range refs {
		refPath := path.Index(i)
		switch ref.Kind {
		case resource.KindGateway, resource.KindRoute:
		default:
			errs = append(errs, field.NotSupported(refPath.Child("kind"), ref.Kind, []string{
				string(resource.KindGateway),
				string(resource.KindRoute),
			}))
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
