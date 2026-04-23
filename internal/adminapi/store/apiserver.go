package store

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"

	adminconfig "github.com/lgc202/ingate/internal/adminapi/config"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

const (
	defaultUserAgent = "ingate-admin-api"
	defaultQPS       = 20
	defaultBurst     = 40
)

type APIServerStore struct {
	client clientset.Interface
}

func NewAPIServerStore(cfg adminconfig.Config) (*APIServerStore, error) {
	client, err := newClientset(cfg)
	if err != nil {
		return nil, err
	}
	return &APIServerStore{client: client}, nil
}

func newClientset(cfg adminconfig.Config) (*clientset.Clientset, error) {
	restConfig := &rest.Config{
		Host:        cfg.APIServerAddress,
		BearerToken: cfg.APIServerToken,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: cfg.APIServerInsecureSkipTLSVerify,
		},
		UserAgent: defaultUserAgent,
		QPS:       defaultQPS,
		Burst:     defaultBurst,
	}

	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create ingate apiserver clientset: %w", err)
	}
	return client, nil
}

func (s *APIServerStore) CreateGateway(ctx context.Context, gateway *gatewayv1alpha1.Gateway) (*gatewayv1alpha1.Gateway, error) {
	return s.client.GatewayV1alpha1().Gateways().Create(ctx, gateway, createOptions)
}

func (s *APIServerStore) UpdateGateway(ctx context.Context, gateway *gatewayv1alpha1.Gateway) (*gatewayv1alpha1.Gateway, error) {
	return s.client.GatewayV1alpha1().Gateways().Update(ctx, gateway, updateOptions)
}

func (s *APIServerStore) DeleteGateway(ctx context.Context, name string) error {
	return s.client.GatewayV1alpha1().Gateways().Delete(ctx, name, deleteOptions)
}

func (s *APIServerStore) GetGateway(ctx context.Context, name string) (*gatewayv1alpha1.Gateway, error) {
	return s.client.GatewayV1alpha1().Gateways().Get(ctx, name, getOptions)
}

func (s *APIServerStore) ListGateways(ctx context.Context) (*gatewayv1alpha1.GatewayList, error) {
	return s.client.GatewayV1alpha1().Gateways().List(ctx, listOptions)
}

func (s *APIServerStore) CreateBackend(ctx context.Context, backend *gatewayv1alpha1.Backend) (*gatewayv1alpha1.Backend, error) {
	return s.client.GatewayV1alpha1().Backends().Create(ctx, backend, createOptions)
}

func (s *APIServerStore) UpdateBackend(ctx context.Context, backend *gatewayv1alpha1.Backend) (*gatewayv1alpha1.Backend, error) {
	return s.client.GatewayV1alpha1().Backends().Update(ctx, backend, updateOptions)
}

func (s *APIServerStore) DeleteBackend(ctx context.Context, name string) error {
	return s.client.GatewayV1alpha1().Backends().Delete(ctx, name, deleteOptions)
}

func (s *APIServerStore) GetBackend(ctx context.Context, name string) (*gatewayv1alpha1.Backend, error) {
	return s.client.GatewayV1alpha1().Backends().Get(ctx, name, getOptions)
}

func (s *APIServerStore) ListBackends(ctx context.Context) (*gatewayv1alpha1.BackendList, error) {
	return s.client.GatewayV1alpha1().Backends().List(ctx, listOptions)
}

func (s *APIServerStore) CreateCertificate(ctx context.Context, certificate *gatewayv1alpha1.Certificate) (*gatewayv1alpha1.Certificate, error) {
	return s.client.GatewayV1alpha1().Certificates().Create(ctx, certificate, createOptions)
}

func (s *APIServerStore) UpdateCertificate(ctx context.Context, certificate *gatewayv1alpha1.Certificate) (*gatewayv1alpha1.Certificate, error) {
	return s.client.GatewayV1alpha1().Certificates().Update(ctx, certificate, updateOptions)
}

func (s *APIServerStore) DeleteCertificate(ctx context.Context, name string) error {
	return s.client.GatewayV1alpha1().Certificates().Delete(ctx, name, deleteOptions)
}

func (s *APIServerStore) GetCertificate(ctx context.Context, name string) (*gatewayv1alpha1.Certificate, error) {
	return s.client.GatewayV1alpha1().Certificates().Get(ctx, name, getOptions)
}

func (s *APIServerStore) ListCertificates(ctx context.Context) (*gatewayv1alpha1.CertificateList, error) {
	return s.client.GatewayV1alpha1().Certificates().List(ctx, listOptions)
}

func (s *APIServerStore) CreateSecret(ctx context.Context, secret *gatewayv1alpha1.Secret) (*gatewayv1alpha1.Secret, error) {
	return s.client.GatewayV1alpha1().Secrets().Create(ctx, secret, createOptions)
}

func (s *APIServerStore) UpdateSecret(ctx context.Context, secret *gatewayv1alpha1.Secret) (*gatewayv1alpha1.Secret, error) {
	return s.client.GatewayV1alpha1().Secrets().Update(ctx, secret, updateOptions)
}

func (s *APIServerStore) DeleteSecret(ctx context.Context, name string) error {
	return s.client.GatewayV1alpha1().Secrets().Delete(ctx, name, deleteOptions)
}

func (s *APIServerStore) GetSecret(ctx context.Context, name string) (*gatewayv1alpha1.Secret, error) {
	return s.client.GatewayV1alpha1().Secrets().Get(ctx, name, getOptions)
}

func (s *APIServerStore) ListSecrets(ctx context.Context) (*gatewayv1alpha1.SecretList, error) {
	return s.client.GatewayV1alpha1().Secrets().List(ctx, listOptions)
}

func (s *APIServerStore) CreateRoute(ctx context.Context, route *gatewayv1alpha1.Route) (*gatewayv1alpha1.Route, error) {
	return s.client.GatewayV1alpha1().Routes().Create(ctx, route, createOptions)
}

func (s *APIServerStore) UpdateRoute(ctx context.Context, route *gatewayv1alpha1.Route) (*gatewayv1alpha1.Route, error) {
	return s.client.GatewayV1alpha1().Routes().Update(ctx, route, updateOptions)
}

func (s *APIServerStore) DeleteRoute(ctx context.Context, name string) error {
	return s.client.GatewayV1alpha1().Routes().Delete(ctx, name, deleteOptions)
}

func (s *APIServerStore) GetRoute(ctx context.Context, name string) (*gatewayv1alpha1.Route, error) {
	return s.client.GatewayV1alpha1().Routes().Get(ctx, name, getOptions)
}

func (s *APIServerStore) ListRoutes(ctx context.Context) (*gatewayv1alpha1.RouteList, error) {
	return s.client.GatewayV1alpha1().Routes().List(ctx, listOptions)
}

func (s *APIServerStore) CreateAuthPolicy(ctx context.Context, policy *policyv1alpha1.AuthPolicy) (*policyv1alpha1.AuthPolicy, error) {
	return s.client.PolicyV1alpha1().AuthPolicies().Create(ctx, policy, createOptions)
}

func (s *APIServerStore) UpdateAuthPolicy(ctx context.Context, policy *policyv1alpha1.AuthPolicy) (*policyv1alpha1.AuthPolicy, error) {
	return s.client.PolicyV1alpha1().AuthPolicies().Update(ctx, policy, updateOptions)
}

func (s *APIServerStore) DeleteAuthPolicy(ctx context.Context, name string) error {
	return s.client.PolicyV1alpha1().AuthPolicies().Delete(ctx, name, deleteOptions)
}

func (s *APIServerStore) GetAuthPolicy(ctx context.Context, name string) (*policyv1alpha1.AuthPolicy, error) {
	return s.client.PolicyV1alpha1().AuthPolicies().Get(ctx, name, getOptions)
}

func (s *APIServerStore) ListAuthPolicies(ctx context.Context) (*policyv1alpha1.AuthPolicyList, error) {
	return s.client.PolicyV1alpha1().AuthPolicies().List(ctx, listOptions)
}

func (s *APIServerStore) CreateTrafficPolicy(ctx context.Context, policy *policyv1alpha1.TrafficPolicy) (*policyv1alpha1.TrafficPolicy, error) {
	return s.client.PolicyV1alpha1().TrafficPolicies().Create(ctx, policy, createOptions)
}

func (s *APIServerStore) UpdateTrafficPolicy(ctx context.Context, policy *policyv1alpha1.TrafficPolicy) (*policyv1alpha1.TrafficPolicy, error) {
	return s.client.PolicyV1alpha1().TrafficPolicies().Update(ctx, policy, updateOptions)
}

func (s *APIServerStore) DeleteTrafficPolicy(ctx context.Context, name string) error {
	return s.client.PolicyV1alpha1().TrafficPolicies().Delete(ctx, name, deleteOptions)
}

func (s *APIServerStore) GetTrafficPolicy(ctx context.Context, name string) (*policyv1alpha1.TrafficPolicy, error) {
	return s.client.PolicyV1alpha1().TrafficPolicies().Get(ctx, name, getOptions)
}

func (s *APIServerStore) ListTrafficPolicies(ctx context.Context) (*policyv1alpha1.TrafficPolicyList, error) {
	return s.client.PolicyV1alpha1().TrafficPolicies().List(ctx, listOptions)
}
