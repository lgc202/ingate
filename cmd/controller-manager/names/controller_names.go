package names

const (
	ControllerManagerName          = "ingate-controller-manager"
	DefaultLeaderElectionNamespace = "ingate-system"

	GatewayControllerName         = "gateway"
	RouteControllerName           = "route"
	BackendControllerName         = "backend"
	CertificateControllerName     = "certificate"
	AuthPolicyControllerName      = "authpolicy"
	TrafficPolicyControllerName   = "trafficpolicy"
	ResolvedGatewayControllerName = "resolvedgateway"
)

var TriggerControllerNames = []string{
	GatewayControllerName,
	RouteControllerName,
	BackendControllerName,
	CertificateControllerName,
	AuthPolicyControllerName,
	TrafficPolicyControllerName,
}
