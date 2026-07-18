package accesscontrolpolicy

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	accesscontrolpolicystore "github.com/lgc202/ingate/internal/adminapi/store/accesscontrolpolicy"
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
)

func TestServiceCreateStoresValidatedTargets(t *testing.T) {
	ctx := context.Background()
	client := clientfake.NewSimpleClientset()
	route := &resource.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route-1"},
		Spec:       resource.RouteSpec{DisplayName: "模型路由"},
	}
	if _, err := client.GatewayV1().Routes().Create(ctx, route, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Routes.Create(%q) error: %v", route.Name, err)
	}
	service := New(
		accesscontrolpolicystore.New(client),
		gatewaystore.New(client),
		routestore.New(client),
	)

	policyID, err := service.Create(ctx, CreatePolicyParams{PolicyParams: testPolicyParams([]TargetParams{{Kind: resource.KindRoute, ID: route.Name}})})
	if err != nil {
		t.Fatalf("Service.Create() error: %v", err)
	}
	policy, err := client.GatewayV1().AccessControlPolicies().Get(ctx, policyID, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("AccessControlPolicies.Get(%q) error: %v", policyID, err)
	}
	want := resource.PolicyTargetRef{Kind: resource.KindRoute, Name: route.Name}
	if len(policy.Spec.TargetRefs) != 1 || policy.Spec.TargetRefs[0] != want {
		t.Errorf("stored target refs = %v, want [%v]", policy.Spec.TargetRefs, want)
	}
}

func TestServiceCreateRejectsMissingTarget(t *testing.T) {
	client := clientfake.NewSimpleClientset()
	service := New(
		accesscontrolpolicystore.New(client),
		gatewaystore.New(client),
		routestore.New(client),
	)

	_, err := service.Create(context.Background(), CreatePolicyParams{PolicyParams: testPolicyParams([]TargetParams{{Kind: resource.KindGateway, ID: "missing-gateway"}})})
	var userError *xerrors.UserError
	if !errors.As(err, &userError) {
		t.Fatalf("Service.Create(missing gateway) error = %T, want *xerrors.UserError", err)
	}
}

func testPolicyParams(targets []TargetParams) PolicyParams {
	return PolicyParams{
		Name:          "内部访问",
		Enabled:       true,
		Targets:       targets,
		DefaultAction: resource.AccessControlActionDeny,
	}
}
