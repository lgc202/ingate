package repository

import (
	"context"
	"errors"
	"testing"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
)

func TestGatewayCreate(t *testing.T) {
	client := clientfake.NewSimpleClientset()
	gateways := NewGateway(client.GatewayV1alpha1().Gateways())

	created, err := gateways.Create(context.Background(), &gatewayv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "public-gateway"},
	})
	if err != nil {
		t.Fatalf("Gateway.Create(public-gateway) error = %v, want nil", err)
	}
	if created.Name != "public-gateway" {
		t.Errorf("Gateway.Create(public-gateway) name = %q, want public-gateway", created.Name)
	}
}

func TestGatewayCreateAlreadyExists(t *testing.T) {
	client := clientfake.NewSimpleClientset()
	resource := gatewayv1alpha1.SchemeGroupVersion.WithResource("gateways").GroupResource()
	storageErr := apierrors.NewAlreadyExists(resource, "public-gateway")
	client.PrependReactor("create", "gateways", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, storageErr
	})
	gateways := NewGateway(client.GatewayV1alpha1().Gateways())

	_, err := gateways.Create(context.Background(), &gatewayv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "public-gateway"},
	})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Errorf("Gateway.Create(public-gateway) error = %v, want ErrAlreadyExists", err)
	}
}
