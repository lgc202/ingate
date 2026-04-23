package convert

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func BackendFromCreateRequest(req dto.CreateBackendRequest) *gatewayv1alpha1.Backend {
	return &gatewayv1alpha1.Backend{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gatewayv1alpha1.SchemeGroupVersion.String(),
			Kind:       "Backend",
		},
		ObjectMeta: metav1.ObjectMeta{Name: req.Name},
		Spec: gatewayv1alpha1.BackendSpec{
			Type:        req.Type,
			Protocol:    req.Protocol,
			DefaultPort: req.DefaultPort,
			Static:      staticBackendFromDTO(req.Static),
			DNS:         dnsBackendFromDTO(req.DNS),
			LoadBalance: loadBalanceFromDTO(req.LoadBalance),
		},
	}
}

func BackendFromUpdateRequest(name string, req dto.UpdateBackendRequest) *gatewayv1alpha1.Backend {
	return &gatewayv1alpha1.Backend{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gatewayv1alpha1.SchemeGroupVersion.String(),
			Kind:       "Backend",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: gatewayv1alpha1.BackendSpec{
			Type:        req.Type,
			Protocol:    req.Protocol,
			DefaultPort: req.DefaultPort,
			Static:      staticBackendFromDTO(req.Static),
			DNS:         dnsBackendFromDTO(req.DNS),
			LoadBalance: loadBalanceFromDTO(req.LoadBalance),
		},
	}
}

func BackendToResponse(backend *gatewayv1alpha1.Backend) dto.BackendResponse {
	if backend == nil {
		return dto.BackendResponse{}
	}
	return dto.BackendResponse{
		Metadata: dto.NewObjectMeta(backend.ObjectMeta),
		Spec: dto.BackendSpec{
			Type:        backend.Spec.Type,
			Protocol:    normalizeBackendProtocol(backend.Spec.Protocol),
			DefaultPort: backend.Spec.DefaultPort,
			Static:      staticBackendToDTO(backend.Spec.Static),
			DNS:         dnsBackendToDTO(backend.Spec.DNS),
			LoadBalance: loadBalanceToDTO(backend.Spec.LoadBalance),
		},
		Status: dto.BackendStatusView{
			ObservedGeneration: backend.Status.ObservedGeneration,
			Conditions:         dto.NewConditions(backend.Status.Conditions),
			Endpoints:          backendEndpointsToDTO(backend.Status.Endpoints),
		},
	}
}

func BackendListToResponse(list *gatewayv1alpha1.BackendList) dto.BackendListResponse {
	if list == nil {
		return dto.BackendListResponse{}
	}
	items := make([]dto.BackendResponse, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, BackendToResponse(&list.Items[i]))
	}
	return dto.BackendListResponse{Items: items}
}

func staticBackendFromDTO(spec *dto.StaticBackendSpec) *gatewayv1alpha1.StaticBackendSpec {
	if spec == nil {
		return nil
	}
	return &gatewayv1alpha1.StaticBackendSpec{Endpoints: backendEndpointsFromDTO(spec.Endpoints)}
}

func staticBackendToDTO(spec *gatewayv1alpha1.StaticBackendSpec) *dto.StaticBackendSpec {
	if spec == nil {
		return nil
	}
	return &dto.StaticBackendSpec{Endpoints: backendEndpointsToDTO(spec.Endpoints)}
}

func dnsBackendFromDTO(spec *dto.DNSBackendSpec) *gatewayv1alpha1.DNSBackendSpec {
	if spec == nil {
		return nil
	}
	return &gatewayv1alpha1.DNSBackendSpec{Host: spec.Host, Port: spec.Port}
}

func dnsBackendToDTO(spec *gatewayv1alpha1.DNSBackendSpec) *dto.DNSBackendSpec {
	if spec == nil {
		return nil
	}
	return &dto.DNSBackendSpec{Host: spec.Host, Port: spec.Port}
}

func loadBalanceFromDTO(spec *dto.LoadBalanceSpec) *gatewayv1alpha1.LoadBalanceSpec {
	if spec == nil {
		return nil
	}
	return &gatewayv1alpha1.LoadBalanceSpec{Policy: spec.Policy}
}

func loadBalanceToDTO(spec *gatewayv1alpha1.LoadBalanceSpec) *dto.LoadBalanceSpec {
	if spec == nil {
		return nil
	}
	return &dto.LoadBalanceSpec{Policy: spec.Policy}
}

func backendEndpointsFromDTO(endpoints []dto.BackendEndpoint) []gatewayv1alpha1.BackendEndpoint {
	items := make([]gatewayv1alpha1.BackendEndpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		items = append(items, gatewayv1alpha1.BackendEndpoint{
			Address: endpoint.Address,
			Port:    endpoint.Port,
			Weight:  endpoint.Weight,
			Healthy: endpoint.Healthy,
		})
	}
	return items
}

func backendEndpointsToDTO(endpoints []gatewayv1alpha1.BackendEndpoint) []dto.BackendEndpoint {
	items := make([]dto.BackendEndpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		items = append(items, dto.BackendEndpoint{
			Address: endpoint.Address,
			Port:    endpoint.Port,
			Weight:  endpoint.Weight,
			Healthy: endpoint.Healthy,
		})
	}
	return items
}

func normalizeBackendProtocol(protocol string) string {
	if protocol == "" {
		return gatewayv1alpha1.BackendProtocolHTTP
	}
	return protocol
}
