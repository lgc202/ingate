package gateway

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

const gatewayAPIVersion = "gateway.ingate.io/v1"

// strategy 定义 Gateway 资源在 apiserver 存储前后的处理规则
type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// statusStrategy 定义 Gateway status 子资源更新规则
type statusStrategy struct {
	strategy
}

func newStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{
		ObjectTyper:   typer,
		NameGenerator: names.SimpleNameGenerator,
	}
}

func (strategy) NamespaceScoped() bool {
	return false
}

func (strategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		fieldpath.APIVersion(gatewayAPIVersion): fieldpath.NewSet(
			fieldpath.MakePathOrDie("status"),
		),
	}
}

func (strategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	gateway := obj.(*resource.Gateway)
	gateway.Status = resource.ResourceStatus{}
	gateway.Generation = 1
}

func (strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return validateGateway(obj.(*resource.Gateway))
}

func (strategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

func (strategy) Canonicalize(obj runtime.Object) {
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newGateway := obj.(*resource.Gateway)
	oldGateway := old.(*resource.Gateway)

	newGateway.Status = oldGateway.Status
	newGateway.Generation = oldGateway.Generation
	if !apiequality.Semantic.DeepEqual(oldGateway.Spec, newGateway.Spec) {
		newGateway.Generation = oldGateway.Generation + 1
	}
}

func (strategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validateGateway(obj.(*resource.Gateway))
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
		fieldpath.APIVersion(gatewayAPIVersion): fieldpath.NewSet(
			fieldpath.MakePathOrDie("spec"),
		),
	}
}

func (statusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newGateway := obj.(*resource.Gateway)
	oldGateway := old.(*resource.Gateway)

	newGateway.Spec = oldGateway.Spec
	metav1.ResetObjectMetaForStatus(&newGateway.ObjectMeta, &oldGateway.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return nil
}

func validateGateway(gateway *resource.Gateway) field.ErrorList {
	specPath := field.NewPath("spec")
	errs := field.ErrorList{}

	if gateway.Spec.DisplayName == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	}
	if len(gateway.Spec.Listeners) == 0 {
		errs = append(errs, field.Required(specPath.Child("listeners"), "at least one listener is required"))
		return errs
	}

	listenerNames := make(map[string]struct{}, len(gateway.Spec.Listeners))
	ports := map[int]*field.Path{}
	for i, listener := range gateway.Spec.Listeners {
		listenerPath := specPath.Child("listeners").Index(i)
		if listener.Name == "" {
			errs = append(errs, field.Required(listenerPath.Child("name"), "listener name is required"))
		} else if _, ok := listenerNames[listener.Name]; ok {
			errs = append(errs, field.Duplicate(listenerPath.Child("name"), listener.Name))
		}
		switch listener.Protocol {
		case resource.ProtocolHTTP:
			if listener.CertificateRef != "" {
				errs = append(errs, field.Forbidden(listenerPath.Child("certificateRef"), "certificateRef is only supported by HTTPS listeners"))
			}
		case resource.ProtocolHTTPS:
			if listener.CertificateRef == "" {
				errs = append(errs, field.Required(listenerPath.Child("certificateRef"), "certificateRef is required for HTTPS listeners"))
			}
		default:
			errs = append(errs, field.NotSupported(listenerPath.Child("protocol"), listener.Protocol, []string{
				string(resource.ProtocolHTTP),
				string(resource.ProtocolHTTPS),
			}))
		}
		if listener.Port < 1 || listener.Port > 65535 {
			errs = append(errs, field.Invalid(listenerPath.Child("port"), listener.Port, "listener port must be between 1 and 65535"))
		} else if firstPath, ok := ports[listener.Port]; ok {
			errs = append(errs, field.Duplicate(listenerPath.Child("port"), listener.Port))
			errs = append(errs, field.Duplicate(firstPath, listener.Port))
		}

		listenerNames[listener.Name] = struct{}{}
		ports[listener.Port] = listenerPath.Child("port")
	}

	hostnamesByListener := make(map[string][]string, len(gateway.Spec.Listeners))
	for i, binding := range gateway.Spec.HostBindings {
		bindingPath := specPath.Child("hostBindings").Index(i)
		normalizedHostname, hostnameValid := hostnameutil.Normalize(binding.Hostname)
		hostnameValid = hostnameValid && binding.Hostname != "*"
		if !hostnameValid {
			errs = append(errs, field.Invalid(bindingPath.Child("hostname"), binding.Hostname, "hostname is invalid"))
		}

		if len(binding.ListenerRefs) == 0 {
			errs = append(errs, field.Required(bindingPath.Child("listenerRefs"), "listenerRefs is required"))
		}
		seenListenerRefs := make(map[string]struct{}, len(binding.ListenerRefs))
		for j, listenerRef := range binding.ListenerRefs {
			listenerRefPath := bindingPath.Child("listenerRefs").Index(j)
			if listenerRef == "" {
				errs = append(errs, field.Required(listenerRefPath, "listenerRef cannot be empty"))
				continue
			}
			if _, exists := seenListenerRefs[listenerRef]; exists {
				errs = append(errs, field.Duplicate(listenerRefPath, listenerRef))
				continue
			}
			seenListenerRefs[listenerRef] = struct{}{}
			if _, ok := listenerNames[listenerRef]; !ok {
				errs = append(errs, field.Invalid(listenerRefPath, listenerRef, "listenerRef references unknown listener"))
				continue
			}
			if !hostnameValid {
				continue
			}

			for _, existingHostname := range hostnamesByListener[listenerRef] {
				if hostnameutil.Overlaps(normalizedHostname, existingHostname) {
					errs = append(errs, field.Invalid(
						bindingPath.Child("hostname"),
						binding.Hostname,
						"hostname overlaps another binding for the same listener",
					))
					break
				}
			}
			hostnamesByListener[listenerRef] = append(hostnamesByListener[listenerRef], normalizedHostname)
		}
	}
	return errs
}
