package convert

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func GatewayFromCreateRequest(req dto.CreateGatewayRequest) *gatewayv1alpha1.Gateway {
	return &gatewayv1alpha1.Gateway{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gatewayv1alpha1.SchemeGroupVersion.String(),
			Kind:       "Gateway",
		},
		ObjectMeta: metav1.ObjectMeta{Name: req.Name},
		Spec: gatewayv1alpha1.GatewaySpec{
			Listeners:     gatewayListenersFromDTO(req.Listeners),
			AllowedRoutes: allowedRoutesFromDTO(req.AllowedRouteKinds),
		},
	}
}

func GatewayFromUpdateRequest(name string, req dto.UpdateGatewayRequest) *gatewayv1alpha1.Gateway {
	return &gatewayv1alpha1.Gateway{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gatewayv1alpha1.SchemeGroupVersion.String(),
			Kind:       "Gateway",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: gatewayv1alpha1.GatewaySpec{
			Listeners:     gatewayListenersFromDTO(req.Listeners),
			AllowedRoutes: allowedRoutesFromDTO(req.AllowedRouteKinds),
		},
	}
}

func GatewayToResponse(gateway *gatewayv1alpha1.Gateway) dto.GatewayResponse {
	if gateway == nil {
		return dto.GatewayResponse{}
	}
	return dto.GatewayResponse{
		Metadata: dto.NewObjectMeta(gateway.ObjectMeta),
		Spec: dto.GatewaySpec{
			Listeners:         gatewayListenersToDTO(gateway.Spec.Listeners),
			AllowedRouteKinds: allowedRouteKindsToDTO(gateway.Spec.AllowedRoutes),
		},
		Status: dto.GatewayStatusView{
			ObservedGeneration: gateway.Status.ObservedGeneration,
			Conditions:         dto.NewConditions(gateway.Status.Conditions),
			Listeners:          gatewayListenerStatusesToDTO(gateway.Status.Listeners),
		},
	}
}

func GatewayListToResponse(list *gatewayv1alpha1.GatewayList) dto.GatewayListResponse {
	if list == nil {
		return dto.GatewayListResponse{}
	}
	items := make([]dto.GatewayResponse, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, GatewayToResponse(&list.Items[i]))
	}
	return dto.GatewayListResponse{Items: items}
}

func gatewayListenersFromDTO(listeners []dto.GatewayListener) []gatewayv1alpha1.GatewayListener {
	items := make([]gatewayv1alpha1.GatewayListener, 0, len(listeners))
	for _, listener := range listeners {
		items = append(items, gatewayv1alpha1.GatewayListener{
			Name:      listener.Name,
			Protocol:  listener.Protocol,
			Port:      listener.Port,
			Hostnames: gatewayListenerHostnamesFromDTO(listener),
			TLS:       gatewayTLSFromDTO(listener.TLS),
		})
	}
	return items
}

func gatewayListenersToDTO(listeners []gatewayv1alpha1.GatewayListener) []dto.GatewayListener {
	items := make([]dto.GatewayListener, 0, len(listeners))
	for _, listener := range listeners {
		items = append(items, dto.GatewayListener{
			Name:      listener.Name,
			Protocol:  listener.Protocol,
			Port:      listener.Port,
			Hostnames: gatewayListenerHostnames(listener),
			TLS:       gatewayTLSToDTO(listener.TLS),
		})
	}
	return items
}

func gatewayTLSFromDTO(tls *dto.GatewayTLSConfig) *gatewayv1alpha1.GatewayTLSConfig {
	if tls == nil {
		return nil
	}
	return &gatewayv1alpha1.GatewayTLSConfig{
		Mode:           tls.Mode,
		CertificateRef: localObjectReferenceFromDTO(tls.CertificateRef),
	}
}

func gatewayTLSToDTO(tls *gatewayv1alpha1.GatewayTLSConfig) *dto.GatewayTLSConfig {
	if tls == nil {
		return nil
	}
	return &dto.GatewayTLSConfig{
		Mode:           tls.Mode,
		CertificateRef: localObjectReferenceToDTO(tls.CertificateRef),
	}
}

func localObjectReferenceFromDTO(ref *dto.LocalObjectReference) *gatewayv1alpha1.LocalObjectReference {
	if ref == nil {
		return nil
	}
	return &gatewayv1alpha1.LocalObjectReference{Name: ref.Name}
}

func localObjectReferenceToDTO(ref *gatewayv1alpha1.LocalObjectReference) *dto.LocalObjectReference {
	if ref == nil {
		return nil
	}
	return &dto.LocalObjectReference{Name: ref.Name}
}

func allowedRoutesFromDTO(kinds []string) *gatewayv1alpha1.AllowedRoutesSpec {
	if len(kinds) == 0 {
		return nil
	}
	return &gatewayv1alpha1.AllowedRoutesSpec{Kinds: append([]string(nil), kinds...)}
}

func allowedRouteKindsToDTO(spec *gatewayv1alpha1.AllowedRoutesSpec) []string {
	if spec == nil {
		return nil
	}
	return append([]string(nil), spec.Kinds...)
}

func gatewayListenerStatusesToDTO(listeners []gatewayv1alpha1.GatewayListenerStatus) []dto.GatewayListenerStatus {
	items := make([]dto.GatewayListenerStatus, 0, len(listeners))
	for _, listener := range listeners {
		items = append(items, dto.GatewayListenerStatus{
			Name:           listener.Name,
			AttachedRoutes: listener.AttachedRoutes,
			Conditions:     dto.NewConditions(listener.Conditions),
		})
	}
	return items
}

func gatewayListenerHostnamesFromDTO(listener dto.GatewayListener) []string {
	if len(listener.Hostnames) > 0 {
		return append([]string(nil), listener.Hostnames...)
	}
	if listener.Hostname != "" {
		return []string{listener.Hostname}
	}
	return nil
}

func gatewayListenerHostnames(listener gatewayv1alpha1.GatewayListener) []string {
	if len(listener.Hostnames) > 0 {
		return append([]string(nil), listener.Hostnames...)
	}
	if listener.Hostname != "" {
		return []string{listener.Hostname}
	}
	return nil
}
