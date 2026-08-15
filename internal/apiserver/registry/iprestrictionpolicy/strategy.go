package iprestrictionpolicy

import (
	"context"
	"maps"
	"net/netip"
	"slices"
	"strings"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

const gatewayAPIVersion = "gateway.ingate.io/v1"

// strategy 定义 IPRestrictionPolicy 资源在 API Server 存储前后的处理规则
type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// statusStrategy 定义 IPRestrictionPolicy status 子资源更新规则
type statusStrategy struct {
	strategy
}

func newStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{ObjectTyper: typer, NameGenerator: names.SimpleNameGenerator}
}

func (strategy) NamespaceScoped() bool {
	return false
}

func (strategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		fieldpath.APIVersion(gatewayAPIVersion): fieldpath.NewSet(fieldpath.MakePathOrDie("status")),
	}
}

func (strategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	policy := obj.(*resource.IPRestrictionPolicy)
	policy.Status = resource.PolicyStatus{}
	policy.Generation = 1
	canonicalizeSpec(&policy.Spec)
	setUpdatedAt(&policy.ObjectMeta, policy.CreationTimestamp.Time)
}

func (strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return validatePolicy(obj.(*resource.IPRestrictionPolicy))
}

func (strategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

func (strategy) Canonicalize(obj runtime.Object) {
	canonicalizeSpec(&obj.(*resource.IPRestrictionPolicy).Spec)
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newPolicy := obj.(*resource.IPRestrictionPolicy)
	oldPolicy := old.(*resource.IPRestrictionPolicy)

	newPolicy.Status = oldPolicy.Status
	canonicalizeSpec(&newPolicy.Spec)
	newPolicy.Generation = oldPolicy.Generation
	if !apiequality.Semantic.DeepEqual(oldPolicy.Spec, newPolicy.Spec) {
		newPolicy.Generation = oldPolicy.Generation + 1
		setUpdatedAt(&newPolicy.ObjectMeta, time.Now().UTC())
		return
	}
	preserveUpdatedAt(&newPolicy.ObjectMeta, &oldPolicy.ObjectMeta)
}

func (strategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validatePolicy(obj.(*resource.IPRestrictionPolicy))
}

func (strategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}

func (strategy) AllowUnconditionalUpdate() bool {
	return false
}

func newStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: newStrategy(typer)}
}

func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		fieldpath.APIVersion(gatewayAPIVersion): fieldpath.NewSet(fieldpath.MakePathOrDie("spec")),
	}
}

func (statusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newPolicy := obj.(*resource.IPRestrictionPolicy)
	oldPolicy := old.(*resource.IPRestrictionPolicy)

	newPolicy.Spec = oldPolicy.Spec
	metav1.ResetObjectMetaForStatus(&newPolicy.ObjectMeta, &oldPolicy.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return nil
}

func validatePolicy(policy *resource.IPRestrictionPolicy) field.ErrorList {
	specPath := field.NewPath("spec")
	var errs field.ErrorList
	if policy.Spec.DisplayName == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	}
	errs = append(errs, apiregistry.ValidatePolicyTargetRefs(policy.Spec.TargetRefs, specPath.Child("targetRefs"))...)

	hasAllow := len(policy.Spec.Allow) > 0
	hasDeny := len(policy.Spec.Deny) > 0
	if hasAllow == hasDeny {
		errs = append(errs, field.Invalid(specPath, policy.Spec, "exactly one of allow or deny must be configured"))
	}
	errs = append(errs, validateRanges(policy.Spec.Allow, specPath.Child("allow"))...)
	errs = append(errs, validateRanges(policy.Spec.Deny, specPath.Child("deny"))...)
	return errs
}

func validateRanges(values []string, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	for i, value := range values {
		if _, err := netip.ParsePrefix(value); err != nil {
			errs = append(errs, field.Invalid(path.Index(i), value, "value must be an IP address or CIDR prefix"))
		}
	}
	return errs
}

func canonicalizeSpec(spec *resource.IPRestrictionPolicySpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	for i := range spec.TargetRefs {
		spec.TargetRefs[i].Name = strings.TrimSpace(spec.TargetRefs[i].Name)
	}
	spec.Allow = canonicalizeRanges(spec.Allow)
	spec.Deny = canonicalizeRanges(spec.Deny)
}

func canonicalizeRanges(values []string) []string {
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if address, err := netip.ParseAddr(value); err == nil {
			value = netip.PrefixFrom(address, address.BitLen()).String()
		} else if prefix, err := netip.ParsePrefix(value); err == nil {
			value = prefix.Masked().String()
		}
		unique[value] = struct{}{}
	}
	return slices.Sorted(maps.Keys(unique))
}

func setUpdatedAt(metadata *metav1.ObjectMeta, updatedAt time.Time) {
	metadata.Annotations = maps.Clone(metadata.Annotations)
	if metadata.Annotations == nil {
		metadata.Annotations = make(map[string]string, 1)
	}
	metadata.Annotations[resource.AnnotationUpdatedAt] = updatedAt.Format(time.RFC3339Nano)
}

func preserveUpdatedAt(metadata, oldMetadata *metav1.ObjectMeta) {
	metadata.Annotations = maps.Clone(metadata.Annotations)
	delete(metadata.Annotations, resource.AnnotationUpdatedAt)
	updatedAt := oldMetadata.Annotations[resource.AnnotationUpdatedAt]
	if updatedAt == "" {
		return
	}
	if metadata.Annotations == nil {
		metadata.Annotations = make(map[string]string, 1)
	}
	metadata.Annotations[resource.AnnotationUpdatedAt] = updatedAt
}
