package registry

import (
	"cmp"
	"slices"

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/validation/field"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	"github.com/lgc202/ingate/internal/pkg/policyconfig"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
)

// ValidatePolicyTargetRefs 校验具体策略允许的目标类型、名称和重复引用。
func ValidatePolicyTargetRefs(
	refs []resource.PolicyTargetRef,
	path *field.Path,
	allowedKinds ...resource.Kind,
) field.ErrorList {
	errs := field.ErrorList{}
	if len(refs) > policyconfig.MaxTargets {
		errs = append(errs, field.TooMany(path, len(refs), policyconfig.MaxTargets))
		refs = refs[:policyconfig.MaxTargets]
	}

	supportedKinds := lo.Map(allowedKinds, func(kind resource.Kind, _ int) string {
		return string(kind)
	})

	seen := make(map[resource.PolicyTargetRef]bool, len(refs))
	for i, ref := range refs {
		refPath := path.Index(i)
		if !kindAllowed(ref.Kind, allowedKinds) {
			errs = append(errs, field.NotSupported(refPath.Child("kind"), ref.Kind, supportedKinds))
		}
		if !resourceconfig.IsCanonicalID(ref.Name) {
			errs = append(errs, field.Invalid(refPath.Child("name"), ref.Name, "target name must be a canonical UUID"))
		}
		if seen[ref] {
			errs = append(errs, field.Duplicate(refPath, ref))
			continue
		}
		seen[ref] = true
	}
	return errs
}

// CanonicalizePolicyTargetRefs 规范化目标 ID，并按稳定身份排序。
func CanonicalizePolicyTargetRefs(refs []resource.PolicyTargetRef) {
	for i := range refs {
		if targetID, valid := resourceconfig.NormalizeID(refs[i].Name); valid {
			refs[i].Name = targetID
		}
	}
	slices.SortFunc(refs, func(left, right resource.PolicyTargetRef) int {
		return cmp.Or(
			cmp.Compare(left.Kind, right.Kind),
			cmp.Compare(left.Name, right.Name),
		)
	})
}

func kindAllowed(kind resource.Kind, allowed []resource.Kind) bool {
	return slices.Contains(allowed, kind)
}
