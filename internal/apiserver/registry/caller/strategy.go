package caller

import (
	"context"
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

type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

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
		fieldpath.APIVersion(resource.SchemeGroupVersion.String()): fieldpath.NewSet(
			fieldpath.MakePathOrDie("status"),
		),
	}
}

func (strategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	caller := obj.(*resource.Caller)
	caller.Status = resource.ResourceStatus{}
	caller.Generation = 1
	canonicalizeCallerSpec(&caller.Spec)
	apiregistry.SetUpdatedAt(&caller.ObjectMeta, caller.CreationTimestamp.Time)
}

func (strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return validateCaller(obj.(*resource.Caller))
}

func (strategy) WarningsOnCreate(context.Context, runtime.Object) []string {
	return nil
}

func (strategy) Canonicalize(obj runtime.Object) {
	canonicalizeCallerSpec(&obj.(*resource.Caller).Spec)
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newCaller := obj.(*resource.Caller)
	oldCaller := old.(*resource.Caller)

	newCaller.Status = oldCaller.Status
	canonicalizeCallerSpec(&newCaller.Spec)
	newCaller.Generation = oldCaller.Generation
	if !apiequality.Semantic.DeepEqual(oldCaller.Spec, newCaller.Spec) {
		newCaller.Generation = oldCaller.Generation + 1
		apiregistry.SetUpdatedAt(&newCaller.ObjectMeta, time.Now().UTC())
		return
	}
	apiregistry.PreserveUpdatedAt(&newCaller.ObjectMeta, &oldCaller.ObjectMeta)
}

func (strategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	return validateCaller(obj.(*resource.Caller))
}

func (strategy) WarningsOnUpdate(context.Context, runtime.Object, runtime.Object) []string {
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
		fieldpath.APIVersion(resource.SchemeGroupVersion.String()): fieldpath.NewSet(
			fieldpath.MakePathOrDie("spec"),
		),
	}
}

func (statusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newCaller := obj.(*resource.Caller)
	oldCaller := old.(*resource.Caller)
	newCaller.Spec = oldCaller.Spec
	metav1.ResetObjectMetaForStatus(&newCaller.ObjectMeta, &oldCaller.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(context.Context, runtime.Object, runtime.Object) field.ErrorList {
	return nil
}

func canonicalizeCallerSpec(spec *resource.CallerSpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	for i := range spec.RouteRefs {
		spec.RouteRefs[i] = strings.TrimSpace(spec.RouteRefs[i])
	}
	slices.Sort(spec.RouteRefs)
	for i := range spec.AccessKeys {
		spec.AccessKeys[i].DisplayName = strings.TrimSpace(spec.AccessKeys[i].DisplayName)
		spec.AccessKeys[i].SecretDigest = strings.ToLower(strings.TrimSpace(spec.AccessKeys[i].SecretDigest))
	}
	slices.SortFunc(spec.AccessKeys, func(a, b resource.AccessKey) int {
		return strings.Compare(a.ID, b.ID)
	})
}
