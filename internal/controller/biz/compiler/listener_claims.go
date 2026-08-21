package compiler

import (
	"fmt"
	"maps"
	"slices"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
)

type listenerGroup struct {
	key      listenerKey
	claims   []hostnameClaim
	gateways []gatewayListener
}

type gatewayListener struct {
	key            listenerKey
	gatewayID      string
	hostname       string
	certificateRef string
}

type hostnameClaim struct {
	gatewayID string
	hostname  string
}

func (c *compilation) buildListenerGroups() (map[listenerKey]*listenerGroup, map[string][]gatewayListener) {
	groups := make(map[listenerKey]*listenerGroup)
	listenersByGateway := make(map[string][]gatewayListener)
	for _, gatewayID := range slices.Sorted(maps.Keys(c.gateways)) {
		gateway := c.gateways[gatewayID]
		if !gateway.Spec.Enabled {
			continue
		}
		if len(gateway.Spec.Listeners) == 0 {
			c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gatewayID, ReasonInvalidSpec, fmt.Sprintf("gateway %q must declare at least one listener", gatewayID))
			continue
		}

		names := make(map[string]bool, len(gateway.Spec.Listeners))
		for _, listener := range gateway.Spec.Listeners {
			if listener.Name == "" || names[listener.Name] {
				c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gatewayID, ReasonConflict, fmt.Sprintf("gateway %q has an empty or duplicate listener name %q", gatewayID, listener.Name))
				continue
			}
			names[listener.Name] = true
			if listener.Port < 1 || listener.Port > 65535 {
				c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gatewayID, ReasonInvalidSpec, fmt.Sprintf("gateway %q listener %q has an invalid port", gatewayID, listener.Name))
				continue
			}
			if listener.Protocol != gatewayv1.ProtocolHTTP && listener.Protocol != gatewayv1.ProtocolHTTPS {
				c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gatewayID, ReasonUnsupported, fmt.Sprintf("gateway %q listener %q uses unsupported protocol %q", gatewayID, listener.Name, listener.Protocol))
				continue
			}

			hostname, ok := hostnameutil.Normalize(listener.Hostname)
			if !ok {
				c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gatewayID, ReasonInvalidSpec, fmt.Sprintf("gateway %q listener %q has invalid hostname %q", gatewayID, listener.Name, listener.Hostname))
				continue
			}
			if listener.Protocol == gatewayv1.ProtocolHTTP && listener.CertificateRef != "" {
				c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gatewayID, ReasonInvalidSpec, fmt.Sprintf("gateway %q HTTP listener %q must not reference a certificate", gatewayID, listener.Name))
				continue
			}
			if listener.Protocol == gatewayv1.ProtocolHTTPS && !c.validListenerCertificate(gatewayID, listener) {
				continue
			}

			key := listenerKey{address: defaultBindAddress, port: listener.Port, protocol: listener.Protocol}
			group := groups[key]
			if group == nil {
				group = &listenerGroup{key: key}
				groups[key] = group
			}
			gatewayListener := gatewayListener{
				key:            key,
				gatewayID:      gatewayID,
				hostname:       hostname,
				certificateRef: listener.CertificateRef,
			}
			group.claims = append(group.claims, hostnameClaim{gatewayID: gatewayID, hostname: hostname})
			group.gateways = append(group.gateways, gatewayListener)
			listenersByGateway[gatewayID] = append(listenersByGateway[gatewayID], gatewayListener)
		}
	}

	for _, key := range sortedListenerKeys(groups) {
		c.validateHostnameClaims(groups[key])
	}
	c.validateListenerPortOwnership(groups)
	return groups, listenersByGateway
}

func (c *compilation) validListenerCertificate(gatewayID string, listener gatewayv1.Listener) bool {
	if listener.CertificateRef == "" {
		c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gatewayID, ReasonInvalidSpec, fmt.Sprintf("gateway %q HTTPS listener %q must reference a certificate", gatewayID, listener.Name))
		return false
	}
	if _, exists := c.certificates[listener.CertificateRef]; !exists {
		c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gatewayID, ReasonReferenceNotFound, fmt.Sprintf("gateway %q HTTPS listener %q references missing certificate %q", gatewayID, listener.Name, listener.CertificateRef))
		return false
	}
	if !c.validCertificates[listener.CertificateRef] {
		c.addDiagnostic(SeverityError, gatewayv1.KindGateway, gatewayID, ReasonInvalidReference, fmt.Sprintf("gateway %q HTTPS listener %q references invalid certificate %q", gatewayID, listener.Name, listener.CertificateRef))
		return false
	}
	return true
}

func (c *compilation) validateHostnameClaims(group *listenerGroup) {
	for i, first := range group.claims {
		for _, second := range group.claims[i+1:] {
			if !hostnameutil.Overlaps(first.hostname, second.hostname) {
				continue
			}
			message := fmt.Sprintf("listener %s hostname ranges %q and %q overlap", listenerName(group.key), first.hostname, second.hostname)
			c.addDiagnostic(SeverityError, gatewayv1.KindGateway, first.gatewayID, ReasonConflict, message)
			c.addDiagnostic(SeverityError, gatewayv1.KindGateway, second.gatewayID, ReasonConflict, message)
		}
	}
}

func (c *compilation) validateListenerPortOwnership(groups map[listenerKey]*listenerGroup) {
	keys := sortedListenerKeys(groups)
	for i, first := range keys {
		for _, second := range keys[i+1:] {
			if first.address != second.address || first.port != second.port || first.protocol == second.protocol {
				continue
			}
			message := fmt.Sprintf("port %d is configured for both %s and %s gateways", first.port, first.protocol, second.protocol)
			for _, listener := range groups[first].gateways {
				c.addDiagnostic(SeverityError, gatewayv1.KindGateway, listener.gatewayID, ReasonConflict, message)
			}
			for _, listener := range groups[second].gateways {
				c.addDiagnostic(SeverityError, gatewayv1.KindGateway, listener.gatewayID, ReasonConflict, message)
			}
		}
	}
}
