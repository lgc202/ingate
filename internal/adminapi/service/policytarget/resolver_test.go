package policytarget

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
)

func TestResolverValidatesAndResolvesTargets(t *testing.T) {
	ctx := context.Background()
	client := clientfake.NewSimpleClientset()
	gateway := &resource.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gateway-1"},
		Spec:       resource.GatewaySpec{DisplayName: "生产网关"},
	}
	route := &resource.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route-1"},
		Spec:       resource.RouteSpec{DisplayName: "模型路由"},
	}
	if _, err := client.GatewayV1().Gateways().Create(ctx, gateway, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Gateways.Create(%q) error: %v", gateway.Name, err)
	}
	if _, err := client.GatewayV1().Routes().Create(ctx, route, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Routes.Create(%q) error: %v", route.Name, err)
	}

	resolver := New(gatewaystore.New(client), routestore.New(client))
	refs := []resource.PolicyTargetRef{
		{Kind: resource.KindGateway, Name: gateway.Name},
		{Kind: resource.KindRoute, Name: route.Name},
	}
	if err := resolver.Validate(ctx, refs); err != nil {
		t.Fatalf("Resolver.Validate(%v) error: %v", refs, err)
	}
	names, err := resolver.DisplayNames(ctx, refs)
	if err != nil {
		t.Fatalf("Resolver.DisplayNames(%v) error: %v", refs, err)
	}
	if got := names.Name(refs[0]); got != gateway.Spec.DisplayName {
		t.Errorf("gateway display name = %q, want %q", got, gateway.Spec.DisplayName)
	}
	if got := names.Name(refs[1]); got != route.Spec.DisplayName {
		t.Errorf("route display name = %q, want %q", got, route.Spec.DisplayName)
	}
}

func TestResolverRejectsMissingTarget(t *testing.T) {
	client := clientfake.NewSimpleClientset()
	resolver := New(gatewaystore.New(client), routestore.New(client))

	err := resolver.Validate(context.Background(), []resource.PolicyTargetRef{{Kind: resource.KindRoute, Name: "missing-route"}})
	var userError *xerrors.UserError
	if !errors.As(err, &userError) {
		t.Fatalf("Resolver.Validate(missing route) error = %T, want *xerrors.UserError", err)
	}
}
