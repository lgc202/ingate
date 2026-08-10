package gateway

import (
	"context"
	"maps"
	"strings"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
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
	canonicalizeGatewaySpec(&gateway.Spec)
	setUpdatedAt(&gateway.ObjectMeta, gateway.CreationTimestamp.Time)
}

func (strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return validateGateway(obj.(*resource.Gateway))
}

func (strategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

func (strategy) Canonicalize(obj runtime.Object) {
	gateway := obj.(*resource.Gateway)
	canonicalizeGatewaySpec(&gateway.Spec)
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newGateway := obj.(*resource.Gateway)
	oldGateway := old.(*resource.Gateway)

	newGateway.Status = oldGateway.Status
	canonicalizeGatewaySpec(&newGateway.Spec)
	newGateway.Generation = oldGateway.Generation
	if !apiequality.Semantic.DeepEqual(oldGateway.Spec, newGateway.Spec) {
		newGateway.Generation = oldGateway.Generation + 1
		setUpdatedAt(&newGateway.ObjectMeta, time.Now().UTC())
		return
	}
	preserveUpdatedAt(&newGateway.ObjectMeta, &oldGateway.ObjectMeta)
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

	if strings.TrimSpace(gateway.Spec.DisplayName) == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	}
	if len(gateway.Spec.Listeners) == 0 {
		errs = append(errs, field.Required(specPath.Child("listeners"), "at least one listener is required"))
		return errs
	}

	listenerNames := make(map[string]struct{}, len(gateway.Spec.Listeners))
	for i, listener := range gateway.Spec.Listeners {
		listenerPath := specPath.Child("listeners").Index(i)
		if listener.Name == "" {
			errs = append(errs, field.Required(listenerPath.Child("name"), "listener name is required"))
		} else if _, ok := listenerNames[listener.Name]; ok {
			errs = append(errs, field.Duplicate(listenerPath.Child("name"), listener.Name))
		} else if messages := utilvalidation.IsDNS1123Label(listener.Name); len(messages) > 0 {
			errs = append(errs, field.Invalid(listenerPath.Child("name"), listener.Name, strings.Join(messages, "; ")))
		}
		listenerNames[listener.Name] = struct{}{}

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
		}

		normalizedHostname, hostnameValid := hostnameutil.Normalize(listener.Hostname)
		hostnameValid = hostnameValid && listener.Hostname != "*"
		if !hostnameValid {
			errs = append(errs, field.Invalid(listenerPath.Child("hostname"), listener.Hostname, "hostname is invalid"))
		}

		for j := range i {
			previous := gateway.Spec.Listeners[j]
			if listener.Port != previous.Port {
				continue
			}
			if listener.Protocol != previous.Protocol {
				errs = append(errs, field.Invalid(
					listenerPath.Child("port"),
					listener.Port,
					"listeners sharing a port must use the same protocol",
				))
				continue
			}
			previousHostname, previousValid := hostnameutil.Normalize(previous.Hostname)
			if hostnameValid && previousValid && hostnameutil.Overlaps(normalizedHostname, previousHostname) {
				errs = append(errs, field.Invalid(
					listenerPath.Child("hostname"),
					listener.Hostname,
					"hostname overlaps another listener on the same port",
				))
			}
		}
	}
	return errs
}

func canonicalizeGatewaySpec(spec *resource.GatewaySpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	for i := range spec.Listeners {
		spec.Listeners[i].Name = strings.TrimSpace(spec.Listeners[i].Name)
		spec.Listeners[i].CertificateRef = strings.TrimSpace(spec.Listeners[i].CertificateRef)
		hostname, ok := hostnameutil.Normalize(spec.Listeners[i].Hostname)
		if !ok {
			continue
		}
		if hostname == "*" {
			hostname = ""
		}
		spec.Listeners[i].Hostname = hostname
	}
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
