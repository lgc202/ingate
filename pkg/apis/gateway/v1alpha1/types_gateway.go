package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Gateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GatewaySpec   `json:"spec,omitempty"`
	Status GatewayStatus `json:"status,omitempty"`
}

type GatewaySpec struct {
	// +listType=map
	// +listMapKey=name
	Listeners     []GatewayListener  `json:"listeners,omitempty"`
	AllowedRoutes *AllowedRoutesSpec `json:"allowedRoutes,omitempty"`
}

type GatewayListener struct {
	Name     string `json:"name,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Port     int32  `json:"port,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	// +listType=set
	Hostnames []string          `json:"hostnames,omitempty"`
	TLS       *GatewayTLSConfig `json:"tls,omitempty"`
}

type GatewayTLSConfig struct {
	Mode           string                `json:"mode,omitempty"`
	CertificateRef *LocalObjectReference `json:"certificateRef,omitempty"`
}

type AllowedRoutesSpec struct {
	// +listType=set
	Kinds []string `json:"kinds,omitempty"`
}

type GatewayStatus struct {
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +listType=map
	// +listMapKey=name
	Listeners []GatewayListenerStatus `json:"listeners,omitempty"`
}

type GatewayListenerStatus struct {
	Name           string `json:"name,omitempty"`
	AttachedRoutes int32  `json:"attachedRoutes,omitempty"`
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type LocalObjectReference struct {
	Name string `json:"name,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type GatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Gateway `json:"items"`
}
