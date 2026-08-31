package compiler

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/gatewayconfig"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
)

type listenerGroup struct {
	key              listenerKey
	hostnameClaims   []hostnameClaim
	gatewayListeners []gatewayListener
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

		listeners := c.buildGatewayListeners(gateway)
		for _, listener := range listeners {
			group := groups[listener.key]
			if group == nil {
				group = &listenerGroup{key: listener.key}
				groups[listener.key] = group
			}
			group.hostnameClaims = append(group.hostnameClaims, hostnameClaim{
				gatewayID: gatewayID,
				hostname:  listener.hostname,
			})
			group.gatewayListeners = append(group.gatewayListeners, listener)
			listenersByGateway[gatewayID] = append(
				listenersByGateway[gatewayID],
				listener,
			)
		}
	}

	for _, key := range sortedListenerKeys(groups) {
		c.validateHostnameClaims(groups[key])
	}
	c.validateListenerPortOwnership(groups)
	return groups, listenersByGateway
}

func (c *compilation) buildGatewayListeners(gateway *gatewayv1.Gateway) []gatewayListener {
	listenerCount := len(gateway.Spec.Listeners)
	if listenerCount == 0 {
		c.addResourceError(
			gatewayv1.KindGateway,
			gateway.Name,
			ReasonInvalidSpec,
			fmt.Sprintf("gateway %q must declare at least one listener", gateway.Name),
		)
		return nil
	}
	if listenerCount > gatewayconfig.MaxListeners {
		c.addResourceError(
			gatewayv1.KindGateway,
			gateway.Name,
			ReasonInvalidSpec,
			fmt.Sprintf(
				"gateway %q declares %d listeners; the maximum is %d",
				gateway.Name,
				listenerCount,
				gatewayconfig.MaxListeners,
			),
		)
		return nil
	}

	listeners := make([]gatewayListener, 0, listenerCount)
	seenNames := make(map[string]bool, listenerCount)
	for _, listener := range gateway.Spec.Listeners {
		compiled, ok := c.buildGatewayListener(gateway.Name, listener, seenNames)
		if ok {
			listeners = append(listeners, compiled)
		}
	}
	return listeners
}

func (c *compilation) buildGatewayListener(
	gatewayID string,
	listener gatewayv1.Listener,
	seenNames map[string]bool,
) (gatewayListener, bool) {
	if !gatewayconfig.IsValidListenerName(listener.Name) {
		c.addResourceError(
			gatewayv1.KindGateway,
			gatewayID,
			ReasonInvalidSpec,
			fmt.Sprintf(
				"gateway %q listener name %q is not a valid DNS label",
				gatewayID,
				listener.Name,
			),
		)
		return gatewayListener{}, false
	}
	if seenNames[listener.Name] {
		c.addResourceError(
			gatewayv1.KindGateway,
			gatewayID,
			ReasonInvalidSpec,
			fmt.Sprintf(
				"gateway %q declares listener name %q more than once",
				gatewayID,
				listener.Name,
			),
		)
		return gatewayListener{}, false
	}
	seenNames[listener.Name] = true

	if !gatewayconfig.IsValidListenerPort(listener.Port) {
		c.addResourceError(
			gatewayv1.KindGateway,
			gatewayID,
			ReasonInvalidSpec,
			fmt.Sprintf(
				"gateway %q listener %q has invalid port %d",
				gatewayID,
				listener.Name,
				listener.Port,
			),
		)
		return gatewayListener{}, false
	}
	if listener.Protocol != gatewayv1.ProtocolHTTP && listener.Protocol != gatewayv1.ProtocolHTTPS {
		c.addResourceError(
			gatewayv1.KindGateway,
			gatewayID,
			ReasonUnsupported,
			fmt.Sprintf(
				"gateway %q listener %q uses unsupported protocol %q",
				gatewayID,
				listener.Name,
				listener.Protocol,
			),
		)
		return gatewayListener{}, false
	}

	hostname, ok := hostnameutil.Normalize(listener.Hostname)
	if !ok || listener.Hostname == "*" {
		c.addResourceError(
			gatewayv1.KindGateway,
			gatewayID,
			ReasonInvalidSpec,
			fmt.Sprintf(
				"gateway %q listener %q has invalid hostname %q",
				gatewayID,
				listener.Name,
				listener.Hostname,
			),
		)
		return gatewayListener{}, false
	}
	if listener.Protocol == gatewayv1.ProtocolHTTP && listener.CertificateRef != "" {
		c.addResourceError(
			gatewayv1.KindGateway,
			gatewayID,
			ReasonInvalidSpec,
			fmt.Sprintf(
				"gateway %q HTTP listener %q must not reference a certificate",
				gatewayID,
				listener.Name,
			),
		)
		return gatewayListener{}, false
	}
	if listener.Protocol == gatewayv1.ProtocolHTTPS &&
		!c.validListenerCertificate(gatewayID, listener, hostname) {
		return gatewayListener{}, false
	}

	return gatewayListener{
		key: listenerKey{
			address:  defaultBindAddress,
			port:     listener.Port,
			protocol: listener.Protocol,
		},
		gatewayID:      gatewayID,
		hostname:       hostname,
		certificateRef: listener.CertificateRef,
	}, true
}

func (c *compilation) validListenerCertificate(
	gatewayID string,
	listener gatewayv1.Listener,
	hostname string,
) bool {
	if listener.CertificateRef == "" {
		c.addResourceError(
			gatewayv1.KindGateway,
			gatewayID,
			ReasonInvalidSpec,
			fmt.Sprintf(
				"gateway %q HTTPS listener %q must reference a certificate",
				gatewayID,
				listener.Name,
			),
		)
		return false
	}
	if _, exists := c.certificates[listener.CertificateRef]; !exists {
		c.addResourceError(
			gatewayv1.KindGateway,
			gatewayID,
			ReasonReferenceNotFound,
			fmt.Sprintf(
				"gateway %q HTTPS listener %q references missing certificate %q",
				gatewayID,
				listener.Name,
				listener.CertificateRef,
			),
		)
		return false
	}
	leaf := c.certificateLeaves[listener.CertificateRef]
	if leaf == nil {
		c.addResourceError(
			gatewayv1.KindGateway,
			gatewayID,
			ReasonInvalidReference,
			fmt.Sprintf(
				"gateway %q HTTPS listener %q references invalid certificate %q",
				gatewayID,
				listener.Name,
				listener.CertificateRef,
			),
		)
		return false
	}
	if c.observedAt.Before(leaf.NotBefore) || !c.observedAt.Before(leaf.NotAfter) {
		c.addResourceError(
			gatewayv1.KindGateway,
			gatewayID,
			ReasonInvalidReference,
			fmt.Sprintf(
				"gateway %q HTTPS listener %q references certificate %q outside its validity period",
				gatewayID,
				listener.Name,
				listener.CertificateRef,
			),
		)
		return false
	}
	if hostname == "*" {
		return true
	}
	if strings.HasPrefix(hostname, "*.") {
		if slices.ContainsFunc(leaf.DNSNames, func(name string) bool {
			return strings.EqualFold(name, hostname)
		}) {
			return true
		}
	} else if leaf.VerifyHostname(hostname) == nil {
		return true
	}
	c.addResourceError(
		gatewayv1.KindGateway,
		gatewayID,
		ReasonInvalidReference,
		fmt.Sprintf(
			"gateway %q HTTPS listener %q hostname %q is not covered by certificate %q",
			gatewayID,
			listener.Name,
			hostname,
			listener.CertificateRef,
		),
	)
	return false
}

func (c *compilation) validateHostnameClaims(group *listenerGroup) {
	for i, first := range group.hostnameClaims {
		for _, second := range group.hostnameClaims[i+1:] {
			if !hostnameutil.Overlaps(first.hostname, second.hostname) {
				continue
			}
			message := fmt.Sprintf(
				"listener %s hostname ranges %q and %q overlap",
				listenerName(group.key),
				first.hostname,
				second.hostname,
			)
			c.addResourceError(
				gatewayv1.KindGateway,
				first.gatewayID,
				ReasonConflict,
				message,
			)
			c.addResourceError(
				gatewayv1.KindGateway,
				second.gatewayID,
				ReasonConflict,
				message,
			)
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
			message := fmt.Sprintf(
				"port %d is configured for both %s and %s gateways",
				first.port,
				first.protocol,
				second.protocol,
			)
			for _, listener := range groups[first].gatewayListeners {
				c.addResourceError(
					gatewayv1.KindGateway,
					listener.gatewayID,
					ReasonConflict,
					message,
				)
			}
			for _, listener := range groups[second].gatewayListeners {
				c.addResourceError(
					gatewayv1.KindGateway,
					listener.gatewayID,
					ReasonConflict,
					message,
				)
			}
		}
	}
}
