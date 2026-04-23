package dto

type CreateGatewayRequest struct {
	Name              string            `json:"name" binding:"required"`
	Listeners         []GatewayListener `json:"listeners" binding:"required,min=1,dive"`
	AllowedRouteKinds []string          `json:"allowedRouteKinds,omitempty"`
}

type UpdateGatewayRequest struct {
	Listeners         []GatewayListener `json:"listeners" binding:"required,min=1,dive"`
	AllowedRouteKinds []string          `json:"allowedRouteKinds,omitempty"`
}

type GatewayListener struct {
	Name      string            `json:"name" binding:"required"`
	Protocol  string            `json:"protocol" binding:"required"`
	Port      int32             `json:"port" binding:"required,gte=1,lte=65535"`
	Hostname  string            `json:"hostname,omitempty"`
	Hostnames []string          `json:"hostnames,omitempty"`
	TLS       *GatewayTLSConfig `json:"tls,omitempty"`
}

type GatewayTLSConfig struct {
	Mode           string                `json:"mode,omitempty"`
	CertificateRef *LocalObjectReference `json:"certificateRef,omitempty"`
}

type LocalObjectReference struct {
	Name string `json:"name,omitempty"`
}

type GatewayResponse struct {
	Metadata ObjectMeta        `json:"metadata"`
	Spec     GatewaySpec       `json:"spec"`
	Status   GatewayStatusView `json:"status,omitempty"`
}

type GatewaySpec struct {
	Listeners         []GatewayListener `json:"listeners,omitempty"`
	AllowedRouteKinds []string          `json:"allowedRouteKinds,omitempty"`
}

type GatewayStatusView struct {
	ObservedGeneration int64                   `json:"observedGeneration,omitempty"`
	Conditions         []Condition             `json:"conditions,omitempty"`
	Listeners          []GatewayListenerStatus `json:"listeners,omitempty"`
}

type GatewayListenerStatus struct {
	Name           string      `json:"name"`
	AttachedRoutes int32       `json:"attachedRoutes"`
	Conditions     []Condition `json:"conditions,omitempty"`
}

type GatewayListResponse struct {
	Items []GatewayResponse `json:"items"`
}
