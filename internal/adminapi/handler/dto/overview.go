package dto

type OverviewResponse struct {
	Summary OverviewSummary `json:"summary"`
	Chains  []OverviewChain `json:"chains,omitempty"`
}

type OverviewSummary struct {
	GatewayCount       int32  `json:"gatewayCount"`
	RouteCount         int32  `json:"routeCount"`
	BackendCount       int32  `json:"backendCount"`
	PolicyCount        int32  `json:"policyCount"`
	UnresolvedRefCount int32  `json:"unresolvedRefCount"`
	ControlPlaneStatus string `json:"controlPlaneStatus"`
}

type OverviewChain struct {
	GatewayName string `json:"gatewayName"`
	RouteName   string `json:"routeName"`
	RouteHost   string `json:"routeHost"`
	RoutePath   string `json:"routePath"`
	BackendName string `json:"backendName"`
	Status      string `json:"status"`
}
