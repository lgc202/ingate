package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	BackendProtocolHTTP  = "HTTP"
	BackendProtocolHTTPS = "HTTPS"
	BackendProtocolGRPC  = "gRPC"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Backend struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackendSpec   `json:"spec,omitempty"`
	Status BackendStatus `json:"status,omitempty"`
}

type BackendSpec struct {
	Type        string             `json:"type,omitempty"`
	Protocol    string             `json:"protocol,omitempty"`
	DefaultPort int32              `json:"defaultPort,omitempty"`
	Static      *StaticBackendSpec `json:"static,omitempty"`
	DNS         *DNSBackendSpec    `json:"dns,omitempty"`
	LoadBalance *LoadBalanceSpec   `json:"loadBalance,omitempty"`
}

type StaticBackendSpec struct {
	// +listType=atomic
	Endpoints []BackendEndpoint `json:"endpoints,omitempty"`
}

type DNSBackendSpec struct {
	Host string `json:"host,omitempty"`
	Port int32  `json:"port,omitempty"`
}

type LoadBalanceSpec struct {
	Policy string `json:"policy,omitempty"`
}

type BackendEndpoint struct {
	Address string `json:"address,omitempty"`
	Port    int32  `json:"port,omitempty"`
	Weight  int32  `json:"weight,omitempty"`
	Healthy bool   `json:"healthy,omitempty"`
}

type BackendStatus struct {
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +listType=atomic
	Endpoints []BackendEndpoint `json:"endpoints,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type BackendList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Backend `json:"items"`
}
