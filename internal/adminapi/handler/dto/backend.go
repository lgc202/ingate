package dto

type CreateBackendRequest struct {
	Name        string             `json:"name" binding:"required"`
	Type        string             `json:"type" binding:"required"`
	Protocol    string             `json:"protocol,omitempty"`
	DefaultPort int32              `json:"defaultPort" binding:"required,gte=1,lte=65535"`
	Static      *StaticBackendSpec `json:"static,omitempty"`
	DNS         *DNSBackendSpec    `json:"dns,omitempty"`
	LoadBalance *LoadBalanceSpec   `json:"loadBalance,omitempty"`
}

type UpdateBackendRequest struct {
	Type        string             `json:"type" binding:"required"`
	Protocol    string             `json:"protocol" binding:"required"`
	DefaultPort int32              `json:"defaultPort" binding:"required,gte=1,lte=65535"`
	Static      *StaticBackendSpec `json:"static,omitempty"`
	DNS         *DNSBackendSpec    `json:"dns,omitempty"`
	LoadBalance *LoadBalanceSpec   `json:"loadBalance,omitempty"`
}

type StaticBackendSpec struct {
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

type BackendResponse struct {
	Metadata ObjectMeta        `json:"metadata"`
	Spec     BackendSpec       `json:"spec"`
	Status   BackendStatusView `json:"status,omitempty"`
}

type BackendSpec struct {
	Type        string             `json:"type,omitempty"`
	Protocol    string             `json:"protocol,omitempty"`
	DefaultPort int32              `json:"defaultPort,omitempty"`
	Static      *StaticBackendSpec `json:"static,omitempty"`
	DNS         *DNSBackendSpec    `json:"dns,omitempty"`
	LoadBalance *LoadBalanceSpec   `json:"loadBalance,omitempty"`
}

type BackendStatusView struct {
	ObservedGeneration int64             `json:"observedGeneration,omitempty"`
	Conditions         []Condition       `json:"conditions,omitempty"`
	Endpoints          []BackendEndpoint `json:"endpoints,omitempty"`
}

type BackendListResponse struct {
	Items []BackendResponse `json:"items"`
}
