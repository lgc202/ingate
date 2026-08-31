package caller

import (
	"context"
	"slices"
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
)

type strategy struct {
	apiregistry.Strategy
}

type statusStrategy struct {
	strategy
}

func (strategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	caller := obj.(*resource.Caller)
	caller.Status = resource.ResourceStatus{}
	canonicalizeCallerSpec(&caller.Spec)
	apiregistry.PrepareObjectMetaForCreate(&caller.ObjectMeta)
}

func (strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return validateCaller(obj.(*resource.Caller))
}

func (strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newCaller := obj.(*resource.Caller)
	oldCaller := old.(*resource.Caller)

	newCaller.Status = oldCaller.Status
	canonicalizeCallerSpec(&newCaller.Spec)
	specChanged := !apiequality.Semantic.DeepEqual(oldCaller.Spec, newCaller.Spec)
	apiregistry.PrepareObjectMetaForUpdate(&newCaller.ObjectMeta, &oldCaller.ObjectMeta, specChanged)
}

func (strategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	return validateCaller(obj.(*resource.Caller))
}

func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return apiregistry.SpecResetFields()
}

func (statusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newCaller := obj.(*resource.Caller)
	oldCaller := old.(*resource.Caller)
	newCaller.Spec = oldCaller.Spec
	metav1.ResetObjectMetaForStatus(&newCaller.ObjectMeta, &oldCaller.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	caller := obj.(*resource.Caller)
	return apiregistry.ValidateResourceStatus(caller.Status, caller.Generation)
}

func newStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{Strategy: apiregistry.NewStrategy(typer)}
}

func newStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: newStrategy(typer)}
}

func canonicalizeCallerSpec(spec *resource.CallerSpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	for i := range spec.RouteRefs {
		if routeID, valid := resourceconfig.NormalizeID(spec.RouteRefs[i]); valid {
			spec.RouteRefs[i] = routeID
		}
	}
	slices.Sort(spec.RouteRefs)
	for i := range spec.AccessKeys {
		if accessKeyID, valid := resourceconfig.NormalizeID(spec.AccessKeys[i].ID); valid {
			spec.AccessKeys[i].ID = accessKeyID
		}
		spec.AccessKeys[i].DisplayName = strings.TrimSpace(spec.AccessKeys[i].DisplayName)
		spec.AccessKeys[i].SecretDigest = strings.ToLower(strings.TrimSpace(spec.AccessKeys[i].SecretDigest))
	}
	slices.SortFunc(spec.AccessKeys, func(a, b resource.AccessKey) int {
		return strings.Compare(a.ID, b.ID)
	})
}
