package route

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	"github.com/lgc202/ingate/internal/adminapi/service/policytarget"
	accesscontrolpolicystore "github.com/lgc202/ingate/internal/adminapi/store/accesscontrolpolicy"
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	ratelimitpolicystore "github.com/lgc202/ingate/internal/adminapi/store/ratelimitpolicy"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	upstreamstore "github.com/lgc202/ingate/internal/adminapi/store/upstream"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
)

func TestServiceDeleteRejectsRouteUsedByPolicy(t *testing.T) {
	route := &resource.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route-1"},
		Spec:       resource.RouteSpec{DisplayName: "模型路由"},
	}
	policy := &resource.AccessControlPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "access-control-1"},
		Spec: resource.AccessControlPolicySpec{
			DisplayName: "内网访问控制",
			TargetRefs:  []resource.PolicyTargetRef{{Kind: resource.KindRoute, Name: route.Name}},
		},
	}
	client := clientfake.NewSimpleClientset(route, policy)
	policyUsage := policytarget.NewUsageFinder(
		ratelimitpolicystore.New(client),
		accesscontrolpolicystore.New(client),
	)
	service := New(
		routestore.New(client),
		gatewaystore.New(client),
		upstreamstore.New(client),
		policyUsage,
	)

	err := service.Delete(context.Background(), route.Name)
	var userError *xerrors.UserError
	if !errors.As(err, &userError) {
		t.Errorf("Service.Delete(%q) error = %T, want *xerrors.UserError", route.Name, err)
	}
	if _, err := client.GatewayV1().Routes().Get(context.Background(), route.Name, metav1.GetOptions{}); err != nil {
		t.Errorf("Service.Delete(%q) removed referenced route: %v", route.Name, err)
	}
}
