package gateway

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	"github.com/lgc202/ingate/internal/pkg/gatewayconfig"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
)

func validateGateway(gateway *resource.Gateway) field.ErrorList {
	specPath := field.NewPath("spec")
	errs := apiregistry.ValidateResourceID(gateway.Name, field.NewPath("metadata", "name"))

	errs = append(errs, apiregistry.ValidateDisplayName(
		gateway.Spec.DisplayName,
		specPath.Child("displayName"),
	)...)
	listenerCount := len(gateway.Spec.Listeners)
	if listenerCount == 0 {
		errs = append(errs, field.Required(specPath.Child("listeners"), "at least one listener is required"))
		return errs
	}
	if listenerCount > gatewayconfig.MaxListeners {
		errs = append(errs, field.TooMany(
			specPath.Child("listeners"),
			listenerCount,
			gatewayconfig.MaxListeners,
		))
	}

	listenerNames := make(map[string]bool, listenerCount)
	for i, listener := range gateway.Spec.Listeners {
		listenerPath := specPath.Child("listeners").Index(i)
		if listener.Name == "" {
			errs = append(errs, field.Required(listenerPath.Child("name"), "listener name is required"))
		} else if listenerNames[listener.Name] {
			errs = append(errs, field.Duplicate(listenerPath.Child("name"), listener.Name))
		} else if !gatewayconfig.IsValidListenerName(listener.Name) {
			errs = append(errs, field.Invalid(
				listenerPath.Child("name"),
				listener.Name,
				"listener name must be a DNS label",
			))
		}
		listenerNames[listener.Name] = true

		switch listener.Protocol {
		case resource.ProtocolHTTP:
			if listener.CertificateRef != "" {
				errs = append(errs, field.Forbidden(
					listenerPath.Child("certificateRef"),
					"certificateRef is only supported by HTTPS listeners",
				))
			}
		case resource.ProtocolHTTPS:
			if listener.CertificateRef == "" {
				errs = append(errs, field.Required(
					listenerPath.Child("certificateRef"),
					"certificateRef is required for HTTPS listeners",
				))
			} else if !resourceconfig.IsCanonicalID(listener.CertificateRef) {
				errs = append(errs, field.Invalid(
					listenerPath.Child("certificateRef"),
					listener.CertificateRef,
					"certificateRef must be a canonical UUID",
				))
			}
		default:
			errs = append(errs, field.NotSupported(listenerPath.Child("protocol"), listener.Protocol, []string{
				string(resource.ProtocolHTTP),
				string(resource.ProtocolHTTPS),
			}))
		}
		if !gatewayconfig.IsValidListenerPort(listener.Port) {
			errs = append(errs, field.Invalid(
				listenerPath.Child("port"),
				listener.Port,
				"listener port must be between 1 and 65535",
			))
		}

		normalizedHostname, hostnameValid := hostnameutil.Normalize(listener.Hostname)
		hostnameValid = hostnameValid && listener.Hostname != "*"
		if !hostnameValid {
			errs = append(errs, field.Invalid(
				listenerPath.Child("hostname"),
				listener.Hostname,
				"hostname is invalid",
			))
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
			previousValid = previousValid && previous.Hostname != "*"
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
