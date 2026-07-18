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

func TestServiceCreateRejectsOpenAIUpstreamInOrdinaryRoute(t *testing.T) {
	gateway := &resource.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway-1"}}
	upstream := &resource.Upstream{
		ObjectMeta: metav1.ObjectMeta{Name: "model-upstream"},
		Spec: resource.UpstreamSpec{
			DisplayName: "OpenAI",
			Type:        resource.UpstreamTypeModel,
			Protocol:    resource.UpstreamProtocolOpenAI,
		},
	}
	client := clientfake.NewSimpleClientset()
	if _, err := client.GatewayV1().Gateways().Create(context.Background(), gateway, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Gateways.Create(%q) error = %v, want nil", gateway.Name, err)
	}
	if _, err := client.GatewayV1().Upstreams().Create(context.Background(), upstream, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Upstreams.Create(%q) error = %v, want nil", upstream.Name, err)
	}
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

	_, err := service.Create(context.Background(), CreateRouteParams{
		Name:       "普通路由",
		GatewayIDs: []string{gateway.Name},
		Enabled:    true,
		Rules: []RouteRuleParams{
			{
				Name:       "main",
				PathPrefix: "/",
				Methods:    []string{"POST"},
				Targets:    []TargetParams{{UpstreamID: upstream.Name, Weight: 100}},
			},
		},
	})
	var userError *xerrors.UserError
	if !errors.As(err, &userError) {
		t.Fatalf("Service.Create(ordinary route with OpenAI upstream) error = %T, want *xerrors.UserError", err)
	}
	if got, want := userError.Error(), "模型服务 \"OpenAI\" 只能用于模型路由"; got != want {
		t.Errorf("Service.Create(ordinary route with OpenAI upstream) error = %q, want %q", got, want)
	}
	routes, err := client.GatewayV1().Routes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Routes.List() error = %v, want nil", err)
	}
	if len(routes.Items) != 0 {
		t.Errorf("Service.Create(ordinary route with OpenAI upstream) created %d routes, want 0", len(routes.Items))
	}
}

func TestServiceCreateModelRouteUsesSingleUpstream(t *testing.T) {
	gateway := &resource.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway-1"}}
	upstream := &resource.Upstream{
		ObjectMeta: metav1.ObjectMeta{Name: "model-upstream"},
		Spec: resource.UpstreamSpec{
			DisplayName: "OpenAI",
			Type:        resource.UpstreamTypeModel,
			Protocol:    resource.UpstreamProtocolOpenAI,
		},
	}
	client := clientfake.NewSimpleClientset()
	if _, err := client.GatewayV1().Gateways().Create(context.Background(), gateway, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Gateways.Create(%q) error = %v, want nil", gateway.Name, err)
	}
	if _, err := client.GatewayV1().Upstreams().Create(context.Background(), upstream, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Upstreams.Create(%q) error = %v, want nil", upstream.Name, err)
	}
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

	routeID, err := service.Create(context.Background(), CreateRouteParams{
		Name:       "模型路由",
		GatewayIDs: []string{gateway.Name},
		Enabled:    true,
		Rules: []RouteRuleParams{
			{
				Name:       "chat",
				PathPrefix: "/v1/chat/completions",
				Methods:    []string{"POST"},
				ModelRouting: &ModelRoutingParams{
					UpstreamID: upstream.Name,
					Models: []ModelRouteParams{
						{Model: "chat-default", UpstreamModel: "gpt-4o-mini"},
						{Model: "chat-advanced", UpstreamModel: "gpt-4o"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Service.Create(model route) error = %v, want nil", err)
	}
	created, err := client.GatewayV1().Routes().Get(context.Background(), routeID, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Routes.Get(%q) error = %v, want nil", routeID, err)
	}
	if got := len(created.Spec.Rules); got != 1 {
		t.Fatalf("Service.Create(model route) rule count = %d, want 1", got)
	}
	rule := created.Spec.Rules[0]
	if got := len(rule.UpstreamRefs); got != 0 {
		t.Errorf("Service.Create(model route) ordinary upstream ref count = %d, want 0", got)
	}
	if rule.ModelRouting == nil {
		t.Fatal("Service.Create(model route) modelRouting = nil, want configured model routing")
	}
	if got, want := rule.ModelRouting.UpstreamRef, upstream.Name; got != want {
		t.Errorf("Service.Create(model route) upstreamRef = %q, want %q", got, want)
	}
	if got := len(rule.ModelRouting.Models); got != 2 {
		t.Fatalf("Service.Create(model route) model count = %d, want 2", got)
	}
	if got, want := rule.ModelRouting.Models[0], (resource.ModelRoute{Model: "chat-default", UpstreamModel: "gpt-4o-mini"}); got != want {
		t.Errorf("Service.Create(model route) first model = %#v, want %#v", got, want)
	}
	if got, want := rule.ModelRouting.Models[1], (resource.ModelRoute{Model: "chat-advanced", UpstreamModel: "gpt-4o"}); got != want {
		t.Errorf("Service.Create(model route) second model = %#v, want %#v", got, want)
	}
}

func TestServiceCreateRejectsHTTPUpstreamInModelRoute(t *testing.T) {
	gateway := &resource.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway-1"}}
	upstream := &resource.Upstream{
		ObjectMeta: metav1.ObjectMeta{Name: "application-upstream"},
		Spec: resource.UpstreamSpec{
			DisplayName: "应用服务",
			Type:        resource.UpstreamTypeApplication,
			Protocol:    resource.UpstreamProtocolHTTP,
		},
	}
	client := clientfake.NewSimpleClientset()
	if _, err := client.GatewayV1().Gateways().Create(context.Background(), gateway, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Gateways.Create(%q) error = %v, want nil", gateway.Name, err)
	}
	if _, err := client.GatewayV1().Upstreams().Create(context.Background(), upstream, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Upstreams.Create(%q) error = %v, want nil", upstream.Name, err)
	}
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

	_, err := service.Create(context.Background(), CreateRouteParams{
		Name:       "模型路由",
		GatewayIDs: []string{gateway.Name},
		Enabled:    true,
		Rules: []RouteRuleParams{
			{
				Name:       "chat",
				PathPrefix: "/v1/chat/completions",
				Methods:    []string{"POST"},
				ModelRouting: &ModelRoutingParams{
					UpstreamID: upstream.Name,
					Models:     []ModelRouteParams{{Model: "chat-default"}},
				},
			},
		},
	})
	var userError *xerrors.UserError
	if !errors.As(err, &userError) {
		t.Errorf("Service.Create(model route with HTTP upstream) error = %T, want *xerrors.UserError", err)
		return
	}
	if got, want := userError.Error(), "关联服务 \"应用服务\" 不是 OpenAI 兼容模型服务"; got != want {
		t.Errorf("Service.Create(model route with HTTP upstream) error = %q, want %q", got, want)
	}
}
