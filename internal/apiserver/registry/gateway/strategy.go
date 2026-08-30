package gateway

import (
	"cmp"
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
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
)

// strategy 定义 Gateway 资源在 API Server 存储前后的处理规则。
type strategy struct {
	apiregistry.Strategy
}

// statusStrategy 定义 Gateway status 子资源更新规则。
type statusStrategy struct {
	strategy
}

func (strategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	gateway := obj.(*resource.Gateway)
	gateway.Status = resource.ResourceStatus{}
	canonicalizeGatewaySpec(&gateway.Spec)
	apiregistry.PrepareObjectMetaForCreate(&gateway.ObjectMeta)
}

func (strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return validateGateway(obj.(*resource.Gateway))
}

func (strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newGateway := obj.(*resource.Gateway)
	oldGateway := old.(*resource.Gateway)

	newGateway.Status = oldGateway.Status
	canonicalizeGatewaySpec(&newGateway.Spec)
	specChanged := !apiequality.Semantic.DeepEqual(oldGateway.Spec, newGateway.Spec)
	apiregistry.PrepareObjectMetaForUpdate(&newGateway.ObjectMeta, &oldGateway.ObjectMeta, specChanged)
}

func (strategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	return validateGateway(obj.(*resource.Gateway))
}

func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return apiregistry.SpecResetFields()
}

func (statusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newGateway := obj.(*resource.Gateway)
	oldGateway := old.(*resource.Gateway)

	newGateway.Spec = oldGateway.Spec
	metav1.ResetObjectMetaForStatus(&newGateway.ObjectMeta, &oldGateway.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	gateway := obj.(*resource.Gateway)
	return apiregistry.ValidateResourceStatus(gateway.Status, gateway.Generation)
}

func newStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{Strategy: apiregistry.NewStrategy(typer)}
}

func newStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: newStrategy(typer)}
}

func canonicalizeGatewaySpec(spec *resource.GatewaySpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	for i := range spec.Listeners {
		spec.Listeners[i].Name = strings.TrimSpace(spec.Listeners[i].Name)
		if certificateID, valid := resourceconfig.NormalizeID(spec.Listeners[i].CertificateRef); valid {
			spec.Listeners[i].CertificateRef = certificateID
		}
		hostname, ok := hostnameutil.Normalize(spec.Listeners[i].Hostname)
		if !ok {
			continue
		}
		if hostname == "*" {
			hostname = ""
		}
		spec.Listeners[i].Hostname = hostname
	}
	slices.SortFunc(spec.Listeners, func(left, right resource.Listener) int {
		return cmp.Or(
			strings.Compare(left.Name, right.Name),
			cmp.Compare(left.Port, right.Port),
			cmp.Compare(left.Protocol, right.Protocol),
			strings.Compare(left.Hostname, right.Hostname),
			strings.Compare(left.CertificateRef, right.CertificateRef),
		)
	})
}
