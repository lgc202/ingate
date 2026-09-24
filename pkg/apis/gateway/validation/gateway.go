// Package validation 校验代理网关资源的内部类型。
package validation

import (
	"strings"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

const (
	minPort int32 = 1
	maxPort int32 = 65_535
)

// ValidateGateway 校验新建 Gateway 的资源身份和期望状态。
func ValidateGateway(gateway *gateway.Gateway) field.ErrorList {
	errs := apivalidation.ValidateObjectMeta(
		&gateway.ObjectMeta,
		false,
		apivalidation.NameIsDNSSubdomain,
		field.NewPath("metadata"),
	)
	return append(errs, validateSpec(&gateway.Spec, field.NewPath("spec"))...)
}

// ValidateGatewayUpdate 校验更新后的 Gateway 资源。
func ValidateGatewayUpdate(next, previous *gateway.Gateway) field.ErrorList {
	errs := ValidateGateway(next)
	return append(errs, apivalidation.ValidateObjectMetaUpdate(
		&next.ObjectMeta,
		&previous.ObjectMeta,
		field.NewPath("metadata"),
	)...)
}

// ValidateGatewayStatusUpdate 校验 Gateway Status 子资源更新。
func ValidateGatewayStatusUpdate(next, previous *gateway.Gateway) field.ErrorList {
	errs := apivalidation.ValidateObjectMetaUpdate(
		&next.ObjectMeta,
		&previous.ObjectMeta,
		field.NewPath("metadata"),
	)
	return append(errs, validateConditions(next.Status.Conditions, next.Generation)...)
}

func validateSpec(spec *gateway.GatewaySpec, path *field.Path) field.ErrorList {
	listenersPath := path.Child("listeners")
	if len(spec.Listeners) == 0 {
		return field.ErrorList{field.Required(listenersPath, "must contain at least one listener")}
	}

	var errs field.ErrorList
	names := make(map[string]bool, len(spec.Listeners))
	validListeners := make([]gateway.Listener, 0, len(spec.Listeners))
	for i, listener := range spec.Listeners {
		listenerPath := listenersPath.Index(i)
		listenerErrs := validateListener(listener, listenerPath)
		errs = append(errs, listenerErrs...)

		if listener.Name != "" {
			if names[listener.Name] {
				errs = append(errs, field.Duplicate(listenerPath.Child("name"), listener.Name))
				continue
			}
			names[listener.Name] = true
		}

		if len(listenerErrs) > 0 {
			continue
		}
		for _, existing := range validListeners {
			if existing.Port == listener.Port &&
				(existing.Hostname == "" || listener.Hostname == "" || existing.Hostname == listener.Hostname) {
				errs = append(errs, field.Invalid(
					listenerPath,
					listener,
					"overlaps with another listener on the same port",
				))
				break
			}
		}
		validListeners = append(validListeners, listener)
	}

	return errs
}

func validateListener(listener gateway.Listener, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if listener.Name == "" {
		errs = append(errs, field.Required(path.Child("name"), "is required"))
	} else if messages := validation.IsDNS1123Label(listener.Name); len(messages) > 0 {
		errs = append(errs, field.Invalid(path.Child("name"), listener.Name, strings.Join(messages, "; ")))
	}

	if listener.Protocol != gateway.ProtocolHTTP {
		errs = append(errs, field.NotSupported(
			path.Child("protocol"), listener.Protocol, []string{string(gateway.ProtocolHTTP)},
		))
	}

	if listener.Port < minPort || listener.Port > maxPort {
		errs = append(errs, field.Invalid(path.Child("port"), listener.Port, "must be between 1 and 65535"))
	}

	if listener.Hostname != "" {
		if messages := validation.IsDNS1123Subdomain(listener.Hostname); len(messages) > 0 {
			errs = append(errs, field.Invalid(
				path.Child("hostname"), listener.Hostname, strings.Join(messages, "; "),
			))
		}
	}

	return errs
}
