package gateway

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	apivalidation "github.com/lgc202/ingate/internal/pkg/apis/gateway/validation"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
)

func validateGateway(gateway *resource.Gateway) field.ErrorList {
	specPath := field.NewPath("spec")
	errs := apiregistry.ValidateResourceID(gateway.Name, field.NewPath("metadata", "name"))

	errs = append(errs, apiregistry.ValidateDisplayName(
		gateway.Spec.DisplayName,
		specPath.Child("displayName"),
	)...)
	listeners := gateway.Spec.Listeners
	listenerCount := len(listeners)
	if listenerCount == 0 {
		errs = append(errs, field.Required(specPath.Child("listeners"), "at least one listener is required"))
		return errs
	}
	if listenerCount > apivalidation.MaxListeners {
		errs = append(errs, field.TooMany(
			specPath.Child("listeners"),
			listenerCount,
			apivalidation.MaxListeners,
		))
		listeners = listeners[:apivalidation.MaxListeners]
	}

	listenerNames := make(map[string]bool, len(listeners))
	for i, listener := range listeners {
		listenerPath := specPath.Child("listeners").Index(i)
		listenerErrs, hostname, hostnameValid := validateListener(listener, listenerPath, listenerNames)
		errs = append(errs, listenerErrs...)
		errs = append(errs, validateListenerConflicts(
			listener,
			listeners[:i],
			listenerPath,
			hostname,
			hostnameValid,
		)...)
	}
	return errs
}

func validateListener(
	listener resource.Listener,
	path *field.Path,
	names map[string]bool,
) (field.ErrorList, string, bool) {
	var errs field.ErrorList
	if listener.Name == "" {
		errs = append(errs, field.Required(path.Child("name"), "listener name is required"))
	} else if names[listener.Name] {
		errs = append(errs, field.Duplicate(path.Child("name"), listener.Name))
	} else if !apivalidation.IsValidListenerName(listener.Name) {
		errs = append(errs, field.Invalid(
			path.Child("name"),
			listener.Name,
			"listener name must be a DNS label",
		))
	}
	names[listener.Name] = true

	errs = append(errs, validateListenerProtocol(listener, path)...)
	if !apivalidation.IsValidListenerPort(listener.Port) {
		errs = append(errs, field.Invalid(
			path.Child("port"),
			listener.Port,
			fmt.Sprintf(
				"listener port must be between %d and %d",
				apivalidation.MinListenerPort,
				apivalidation.MaxListenerPort,
			),
		))
	}

	hostname, valid := hostnameutil.Normalize(listener.Hostname)
	valid = valid && listener.Hostname != "*"
	if !valid {
		errs = append(errs, field.Invalid(
			path.Child("hostname"),
			listener.Hostname,
			"hostname is invalid",
		))
	}
	return errs, hostname, valid
}

func validateListenerProtocol(listener resource.Listener, path *field.Path) field.ErrorList {
	switch listener.Protocol {
	case resource.ProtocolHTTP:
		if listener.CertificateRef != "" {
			return field.ErrorList{field.Forbidden(
				path.Child("certificateRef"),
				"certificateRef is only supported by HTTPS listeners",
			)}
		}
	case resource.ProtocolHTTPS:
		if listener.CertificateRef == "" {
			return field.ErrorList{field.Required(
				path.Child("certificateRef"),
				"certificateRef is required for HTTPS listeners",
			)}
		}
		if !apivalidation.IsCanonicalID(listener.CertificateRef) {
			return field.ErrorList{field.Invalid(
				path.Child("certificateRef"),
				listener.CertificateRef,
				"certificateRef must be a canonical UUID",
			)}
		}
	default:
		return field.ErrorList{field.NotSupported(path.Child("protocol"), listener.Protocol, []string{
			string(resource.ProtocolHTTP),
			string(resource.ProtocolHTTPS),
		})}
	}
	return nil
}

func validateListenerConflicts(
	listener resource.Listener,
	previous []resource.Listener,
	path *field.Path,
	hostname string,
	hostnameValid bool,
) field.ErrorList {
	var errs field.ErrorList
	for _, other := range previous {
		if listener.Port != other.Port {
			continue
		}
		if listener.Protocol != other.Protocol {
			errs = append(errs, field.Invalid(
				path.Child("port"),
				listener.Port,
				"listeners sharing a port must use the same protocol",
			))
			continue
		}
		otherHostname, otherValid := hostnameutil.Normalize(other.Hostname)
		otherValid = otherValid && other.Hostname != "*"
		if hostnameValid && otherValid && hostnameutil.Overlaps(hostname, otherHostname) {
			errs = append(errs, field.Invalid(
				path.Child("hostname"),
				listener.Hostname,
				"hostname overlaps another listener on the same port",
			))
		}
	}
	return errs
}
