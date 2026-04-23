package store

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
)

type Store interface {
	CreateGateway(ctx context.Context, gateway *gatewayv1alpha1.Gateway) (*gatewayv1alpha1.Gateway, error)
	UpdateGateway(ctx context.Context, gateway *gatewayv1alpha1.Gateway) (*gatewayv1alpha1.Gateway, error)
	DeleteGateway(ctx context.Context, name string) error
	GetGateway(ctx context.Context, name string) (*gatewayv1alpha1.Gateway, error)
	ListGateways(ctx context.Context) (*gatewayv1alpha1.GatewayList, error)

	CreateBackend(ctx context.Context, backend *gatewayv1alpha1.Backend) (*gatewayv1alpha1.Backend, error)
	UpdateBackend(ctx context.Context, backend *gatewayv1alpha1.Backend) (*gatewayv1alpha1.Backend, error)
	DeleteBackend(ctx context.Context, name string) error
	GetBackend(ctx context.Context, name string) (*gatewayv1alpha1.Backend, error)
	ListBackends(ctx context.Context) (*gatewayv1alpha1.BackendList, error)

	CreateCertificate(ctx context.Context, certificate *gatewayv1alpha1.Certificate) (*gatewayv1alpha1.Certificate, error)
	UpdateCertificate(ctx context.Context, certificate *gatewayv1alpha1.Certificate) (*gatewayv1alpha1.Certificate, error)
	DeleteCertificate(ctx context.Context, name string) error
	GetCertificate(ctx context.Context, name string) (*gatewayv1alpha1.Certificate, error)
	ListCertificates(ctx context.Context) (*gatewayv1alpha1.CertificateList, error)

	CreateSecret(ctx context.Context, secret *gatewayv1alpha1.Secret) (*gatewayv1alpha1.Secret, error)
	UpdateSecret(ctx context.Context, secret *gatewayv1alpha1.Secret) (*gatewayv1alpha1.Secret, error)
	DeleteSecret(ctx context.Context, name string) error
	GetSecret(ctx context.Context, name string) (*gatewayv1alpha1.Secret, error)
	ListSecrets(ctx context.Context) (*gatewayv1alpha1.SecretList, error)

	CreateRoute(ctx context.Context, route *gatewayv1alpha1.Route) (*gatewayv1alpha1.Route, error)
	UpdateRoute(ctx context.Context, route *gatewayv1alpha1.Route) (*gatewayv1alpha1.Route, error)
	DeleteRoute(ctx context.Context, name string) error
	GetRoute(ctx context.Context, name string) (*gatewayv1alpha1.Route, error)
	ListRoutes(ctx context.Context) (*gatewayv1alpha1.RouteList, error)

	CreateAuthPolicy(ctx context.Context, policy *policyv1alpha1.AuthPolicy) (*policyv1alpha1.AuthPolicy, error)
	UpdateAuthPolicy(ctx context.Context, policy *policyv1alpha1.AuthPolicy) (*policyv1alpha1.AuthPolicy, error)
	DeleteAuthPolicy(ctx context.Context, name string) error
	GetAuthPolicy(ctx context.Context, name string) (*policyv1alpha1.AuthPolicy, error)
	ListAuthPolicies(ctx context.Context) (*policyv1alpha1.AuthPolicyList, error)

	CreateTrafficPolicy(ctx context.Context, policy *policyv1alpha1.TrafficPolicy) (*policyv1alpha1.TrafficPolicy, error)
	UpdateTrafficPolicy(ctx context.Context, policy *policyv1alpha1.TrafficPolicy) (*policyv1alpha1.TrafficPolicy, error)
	DeleteTrafficPolicy(ctx context.Context, name string) error
	GetTrafficPolicy(ctx context.Context, name string) (*policyv1alpha1.TrafficPolicy, error)
	ListTrafficPolicies(ctx context.Context) (*policyv1alpha1.TrafficPolicyList, error)
}

var (
	createOptions = metav1.CreateOptions{}
	updateOptions = metav1.UpdateOptions{}
	deleteOptions = metav1.DeleteOptions{}
	getOptions    = metav1.GetOptions{}
	listOptions   = metav1.ListOptions{}
)
