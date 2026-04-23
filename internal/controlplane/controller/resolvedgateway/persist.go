package resolvedgateway

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

type Persister struct {
	client clientset.Interface
}

func NewPersister(client clientset.Interface) *Persister {
	return &Persister{client: client}
}

func (p *Persister) Upsert(ctx context.Context, resolvedGateway *gatewayv1alpha1.ResolvedGateway) (*gatewayv1alpha1.ResolvedGateway, error) {
	if p == nil || p.client == nil {
		return nil, fmt.Errorf("resolvedgateway persister is not initialized")
	}
	if resolvedGateway == nil {
		return nil, fmt.Errorf("resolvedgateway must not be nil")
	}

	current, err := p.client.GatewayV1alpha1().ResolvedGateways().Get(ctx, resolvedGateway.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return p.client.GatewayV1alpha1().ResolvedGateways().Create(ctx, resolvedGateway, metav1.CreateOptions{})
	}
	if err != nil {
		return nil, err
	}

	updated := resolvedGateway.DeepCopy()
	updated.ResourceVersion = current.ResourceVersion
	return p.client.GatewayV1alpha1().ResolvedGateways().Update(ctx, updated, metav1.UpdateOptions{})
}
