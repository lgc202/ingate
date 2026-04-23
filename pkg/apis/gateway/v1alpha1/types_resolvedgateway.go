package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ResolvedGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResolvedGatewaySpec   `json:"spec,omitempty"`
	Status ResolvedGatewayStatus `json:"status,omitempty"`
}

type ResolvedGatewaySpec struct {
	GatewayRef LocalObjectReference `json:"gatewayRef,omitempty"`
	Version    string               `json:"version,omitempty"`

	GatewayAuthSummary    *ResolvedGatewayAuthSummary    `json:"gatewayAuthSummary,omitempty"`
	GatewayTrafficSummary *ResolvedGatewayTrafficSummary `json:"gatewayTrafficSummary,omitempty"`

	// +listType=map
	// +listMapKey=name
	Listeners []ResolvedGatewayListener `json:"listeners,omitempty"`
	// +listType=map
	// +listMapKey=name
	Routes []ResolvedGatewayRoute `json:"routes,omitempty"`
	// +listType=map
	// +listMapKey=name
	Backends []ResolvedGatewayBackend `json:"backends,omitempty"`
	// +listType=map
	// +listMapKey=name
	Extensions []ResolvedGatewayExtension `json:"extensions,omitempty"`
}

type ResolvedGatewayListener struct {
	Name     string `json:"name,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Port     int32  `json:"port,omitempty"`
	// +listType=set
	Hostnames []string                    `json:"hostnames,omitempty"`
	TLS       *ResolvedGatewayListenerTLS `json:"tls,omitempty"`
}

type ResolvedGatewayListenerTLS struct {
	Mode           string                `json:"mode,omitempty"`
	CertificateRef *LocalObjectReference `json:"certificateRef,omitempty"`
	SecretRef      *LocalObjectReference `json:"secretRef,omitempty"`
	// +listType=set
	Domains []string `json:"domains,omitempty"`
}

type ResolvedGatewayRoute struct {
	Name string `json:"name,omitempty"`
	// +listType=set
	Hostnames []string `json:"hostnames,omitempty"`
	// +listType=atomic
	Rules          []ResolvedGatewayRouteRule     `json:"rules,omitempty"`
	AuthSummary    *ResolvedGatewayAuthSummary    `json:"authSummary,omitempty"`
	TrafficSummary *ResolvedGatewayTrafficSummary `json:"trafficSummary,omitempty"`
}

type ResolvedGatewayRouteRule struct {
	// +listType=atomic
	Matches []HTTPRouteMatch `json:"matches,omitempty"`
	// +listType=atomic
	BackendRefs []BackendRef `json:"backendRefs,omitempty"`
	// +listType=atomic
	Filters []HTTPRouteFilter `json:"filters,omitempty"`
}

type ResolvedGatewayBackend struct {
	Name        string           `json:"name,omitempty"`
	Protocol    string           `json:"protocol,omitempty"`
	DefaultPort int32            `json:"defaultPort,omitempty"`
	LoadBalance *LoadBalanceSpec `json:"loadBalance,omitempty"`
	// +listType=atomic
	Endpoints      []BackendEndpoint              `json:"endpoints,omitempty"`
	AuthSummary    *ResolvedGatewayAuthSummary    `json:"authSummary,omitempty"`
	TrafficSummary *ResolvedGatewayTrafficSummary `json:"trafficSummary,omitempty"`
}

type ResolvedGatewayAuthSummary struct {
	// +listType=map
	// +listMapKey=name
	Policies []ResolvedPolicyRef `json:"policies,omitempty"`
}

type ResolvedGatewayTrafficSummary struct {
	// +listType=map
	// +listMapKey=name
	Policies []ResolvedTrafficPolicyRef `json:"policies,omitempty"`
}

type ResolvedPolicyRef struct {
	Kind string `json:"kind,omitempty"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

type ResolvedTrafficPolicyRef struct {
	Kind string `json:"kind,omitempty"`
	Name string `json:"name,omitempty"`

	TimeoutDuration string `json:"timeoutDuration,omitempty"`
	RetryAttempts   int32  `json:"retryAttempts,omitempty"`
	// +listType=set
	RetryConditions []string `json:"retryConditions,omitempty"`

	RateLimitRequests int32  `json:"rateLimitRequests,omitempty"`
	RateLimitUnit     string `json:"rateLimitUnit,omitempty"`
	RateLimitScope    string `json:"rateLimitScope,omitempty"`
}

type ResolvedGatewayExtension struct {
	Name string `json:"name,omitempty"`
}

type ResolvedGatewayStatus struct {
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ResolvedGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResolvedGateway `json:"items"`
}
