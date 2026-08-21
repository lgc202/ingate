package gateway

import (
	"strings"

	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
)

func validateGateway(gateway *resource.Gateway) field.ErrorList {
	specPath := field.NewPath("spec")
	var errs field.ErrorList

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
