package table

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	commonregistry "github.com/lgc202/ingate/internal/controlplane/apiserver/registry/common"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

var (
	gatewayColumns = []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: "Gateway name."},
		{Name: "Listeners", Type: "integer", Description: "Number of listeners."},
		{Name: "Hostnames", Type: "string", Description: "Configured listener hostnames."},
		{Name: "Age", Type: "string", Description: "Time since creation."},
	}
	routeColumns = []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: "Route name."},
		{Name: "Parents", Type: "integer", Description: "Number of parentRefs."},
		{Name: "Rules", Type: "integer", Description: "Number of rules."},
		{Name: "Hostnames", Type: "string", Description: "Configured route hostnames."},
		{Name: "Age", Type: "string", Description: "Time since creation."},
	}
	backendColumns = []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: "Backend name."},
		{Name: "Type", Type: "string", Description: "Backend discovery type."},
		{Name: "Port", Type: "integer", Description: "Default backend port."},
		{Name: "Endpoints", Type: "integer", Description: "Resolved endpoint count."},
		{Name: "Age", Type: "string", Description: "Time since creation."},
	}
	secretColumns = []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: "Secret name."},
		{Name: "Type", Type: "string", Description: "Secret type."},
		{Name: "Keys", Type: "integer", Description: "Number of stored keys."},
		{Name: "Age", Type: "string", Description: "Time since creation."},
	}
	certificateColumns = []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: "Certificate name."},
		{Name: "Secret", Type: "string", Description: "Referenced secret name."},
		{Name: "Domains", Type: "string", Description: "Certificate domains."},
		{Name: "Age", Type: "string", Description: "Time since creation."},
	}
	resolvedGatewayColumns = []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: "ResolvedGateway name."},
		{Name: "Gateway", Type: "string", Description: "Referenced gateway name."},
		{Name: "Listeners", Type: "integer", Description: "Resolved listener count."},
		{Name: "Routes", Type: "integer", Description: "Resolved route count."},
		{Name: "Backends", Type: "integer", Description: "Resolved backend count."},
		{Name: "Age", Type: "string", Description: "Time since creation."},
	}
)

func GatewayCells(obj runtime.Object) ([]interface{}, error) {
	gateway, ok := obj.(*gatewayv1alpha1.Gateway)
	if !ok {
		return nil, fmt.Errorf("expected Gateway, got %T", obj)
	}
	return []interface{}{
		gateway.Name,
		len(gateway.Spec.Listeners),
		joinGatewayHostnames(gateway.Spec.Listeners),
		commonregistry.FormatTimestampAge(gateway.CreationTimestamp),
	}, nil
}

func RouteCells(obj runtime.Object) ([]interface{}, error) {
	route, ok := obj.(*gatewayv1alpha1.Route)
	if !ok {
		return nil, fmt.Errorf("expected Route, got %T", obj)
	}
	return []interface{}{
		route.Name,
		len(route.Spec.ParentRefs),
		len(route.Spec.Rules),
		joinStrings(route.Spec.Hostnames),
		commonregistry.FormatTimestampAge(route.CreationTimestamp),
	}, nil
}

func BackendCells(obj runtime.Object) ([]interface{}, error) {
	backend, ok := obj.(*gatewayv1alpha1.Backend)
	if !ok {
		return nil, fmt.Errorf("expected Backend, got %T", obj)
	}
	return []interface{}{
		backend.Name,
		backend.Spec.Type,
		backend.Spec.DefaultPort,
		len(backend.Status.Endpoints),
		commonregistry.FormatTimestampAge(backend.CreationTimestamp),
	}, nil
}

func SecretCells(obj runtime.Object) ([]interface{}, error) {
	secret, ok := obj.(*gatewayv1alpha1.Secret)
	if !ok {
		return nil, fmt.Errorf("expected Secret, got %T", obj)
	}
	return []interface{}{
		secret.Name,
		secret.Spec.Type,
		len(secret.Spec.StringData),
		commonregistry.FormatTimestampAge(secret.CreationTimestamp),
	}, nil
}

func CertificateCells(obj runtime.Object) ([]interface{}, error) {
	certificate, ok := obj.(*gatewayv1alpha1.Certificate)
	if !ok {
		return nil, fmt.Errorf("expected Certificate, got %T", obj)
	}
	return []interface{}{
		certificate.Name,
		certificate.Spec.SecretRef.Name,
		joinStrings(certificate.Spec.Domains),
		commonregistry.FormatTimestampAge(certificate.CreationTimestamp),
	}, nil
}

func ResolvedGatewayCells(obj runtime.Object) ([]interface{}, error) {
	resolvedGateway, ok := obj.(*gatewayv1alpha1.ResolvedGateway)
	if !ok {
		return nil, fmt.Errorf("expected ResolvedGateway, got %T", obj)
	}
	return []interface{}{
		resolvedGateway.Name,
		resolvedGateway.Spec.GatewayRef.Name,
		len(resolvedGateway.Spec.Listeners),
		len(resolvedGateway.Spec.Routes),
		len(resolvedGateway.Spec.Backends),
		commonregistry.FormatTimestampAge(resolvedGateway.CreationTimestamp),
	}, nil
}

func GatewayColumns() []metav1.TableColumnDefinition { return gatewayColumns }
func RouteColumns() []metav1.TableColumnDefinition   { return routeColumns }
func BackendColumns() []metav1.TableColumnDefinition { return backendColumns }
func SecretColumns() []metav1.TableColumnDefinition  { return secretColumns }
func CertificateColumns() []metav1.TableColumnDefinition {
	return certificateColumns
}
func ResolvedGatewayColumns() []metav1.TableColumnDefinition { return resolvedGatewayColumns }

func joinGatewayHostnames(listeners []gatewayv1alpha1.GatewayListener) string {
	hostnames := make([]string, 0, len(listeners))
	for _, listener := range listeners {
		if len(listener.Hostnames) > 0 {
			hostnames = append(hostnames, listener.Hostnames...)
			continue
		}
		if listener.Hostname != "" {
			hostnames = append(hostnames, listener.Hostname)
		}
	}
	return joinStrings(hostnames)
}

func joinStrings(items []string) string {
	if len(items) == 0 {
		return "*"
	}
	return strings.Join(items, ",")
}
