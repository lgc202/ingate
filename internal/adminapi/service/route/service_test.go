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
			Model: &resource.ModelSpec{
				Provider:    resource.ModelProviderOpenAI,
				APIBasePath: "/v1",
				Models: []resource.ModelCatalogItem{
					{Name: "gpt-4o-mini", DisplayName: "GPT-4o mini", Enabled: true},
				},
			},
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

	_, err := service.Create(context.Background(), resource.RouteSpec{
		DisplayName: "普通路由",
		ParentRefs:  []resource.ParentRef{{Name: gateway.Name}},
		Enabled:     true,
		Rules: []resource.RouteRule{
			{
				Name:         "main",
				PathPrefix:   "/",
				Methods:      []string{"POST"},
				UpstreamRefs: []resource.UpstreamRef{{Name: upstream.Name, Weight: 100}},
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

func TestServiceCreateModelRouteUsesPerModelUpstreams(t *testing.T) {
	gateway := &resource.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway-1"}}
	openAIUpstream := &resource.Upstream{
		ObjectMeta: metav1.ObjectMeta{Name: "openai-upstream"},
		Spec: resource.UpstreamSpec{
			DisplayName: "OpenAI",
			Type:        resource.UpstreamTypeModel,
			Protocol:    resource.UpstreamProtocolOpenAI,
			Model: &resource.ModelSpec{
				Provider:    resource.ModelProviderOpenAI,
				APIBasePath: "/v1",
				Models: []resource.ModelCatalogItem{
					{Name: "gpt-4o-mini", DisplayName: "GPT-4o mini", Enabled: true},
				},
			},
		},
	}
	deepSeekUpstream := &resource.Upstream{
		ObjectMeta: metav1.ObjectMeta{Name: "deepseek-upstream"},
		Spec: resource.UpstreamSpec{
			DisplayName: "DeepSeek",
			Type:        resource.UpstreamTypeModel,
			Protocol:    resource.UpstreamProtocolOpenAI,
			Model: &resource.ModelSpec{
				Provider:    resource.ModelProviderDeepSeek,
				APIBasePath: "/v1",
				Models: []resource.ModelCatalogItem{
					{Name: "deepseek-reasoner", DisplayName: "DeepSeek Reasoner", Enabled: true},
				},
			},
		},
	}
	client := clientfake.NewSimpleClientset()
	if _, err := client.GatewayV1().Gateways().Create(context.Background(), gateway, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Gateways.Create(%q) error = %v, want nil", gateway.Name, err)
	}
	if _, err := client.GatewayV1().Upstreams().Create(context.Background(), openAIUpstream, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Upstreams.Create(%q) error = %v, want nil", openAIUpstream.Name, err)
	}
	if _, err := client.GatewayV1().Upstreams().Create(context.Background(), deepSeekUpstream, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Upstreams.Create(%q) error = %v, want nil", deepSeekUpstream.Name, err)
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

	routeID, err := service.Create(context.Background(), resource.RouteSpec{
		DisplayName: "模型路由",
		ParentRefs:  []resource.ParentRef{{Name: gateway.Name}},
		Enabled:     true,
		Rules: []resource.RouteRule{
			{
				Name:       "chat",
				PathPrefix: "/v1/chat/completions",
				Methods:    []string{"POST"},
				ModelRouting: &resource.ModelRouting{
					Models: []resource.ModelRoute{
						{Model: "chat-default", UpstreamRef: openAIUpstream.Name, UpstreamModel: "gpt-4o-mini"},
						{Model: "reasoning", UpstreamRef: deepSeekUpstream.Name, UpstreamModel: "deepseek-reasoner"},
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
	if got := len(rule.ModelRouting.Models); got != 2 {
		t.Fatalf("Service.Create(model route) model count = %d, want 2", got)
	}
	if got, want := rule.ModelRouting.Models[0], (resource.ModelRoute{Model: "chat-default", UpstreamRef: openAIUpstream.Name, UpstreamModel: "gpt-4o-mini"}); got != want {
		t.Errorf("Service.Create(model route) first model = %#v, want %#v", got, want)
	}
	if got, want := rule.ModelRouting.Models[1], (resource.ModelRoute{Model: "reasoning", UpstreamRef: deepSeekUpstream.Name, UpstreamModel: "deepseek-reasoner"}); got != want {
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

	_, err := service.Create(context.Background(), resource.RouteSpec{
		DisplayName: "模型路由",
		ParentRefs:  []resource.ParentRef{{Name: gateway.Name}},
		Enabled:     true,
		Rules: []resource.RouteRule{
			{
				Name:       "chat",
				PathPrefix: "/v1/chat/completions",
				Methods:    []string{"POST"},
				ModelRouting: &resource.ModelRouting{
					Models: []resource.ModelRoute{{Model: "chat-default", UpstreamRef: upstream.Name}},
				},
			},
		},
	})
	var userError *xerrors.UserError
	if !errors.As(err, &userError) {
		t.Errorf("Service.Create(model route with HTTP upstream) error = %T, want *xerrors.UserError", err)
		return
	}
	if got, want := userError.Error(), "关联服务 \"应用服务\" 不是有效的大模型服务"; got != want {
		t.Errorf("Service.Create(model route with HTTP upstream) error = %q, want %q", got, want)
	}
}

func TestServiceCreateRejectsDisabledUpstreamModel(t *testing.T) {
	gateway := &resource.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway-1"}}
	upstream := &resource.Upstream{
		ObjectMeta: metav1.ObjectMeta{Name: "anthropic-upstream"},
		Spec: resource.UpstreamSpec{
			DisplayName: "Anthropic",
			Type:        resource.UpstreamTypeModel,
			Protocol:    resource.UpstreamProtocolAnthropic,
			Model: &resource.ModelSpec{
				Provider:    resource.ModelProviderAnthropic,
				APIBasePath: "/v1",
				Models: []resource.ModelCatalogItem{
					{Name: "claude-sonnet", DisplayName: "Claude Sonnet", Enabled: false},
					{Name: "claude-haiku", DisplayName: "Claude Haiku", Enabled: true},
				},
			},
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

	_, err := service.Create(context.Background(), resource.RouteSpec{
		DisplayName: "模型路由",
		ParentRefs:  []resource.ParentRef{{Name: gateway.Name}},
		Enabled:     true,
		Rules: []resource.RouteRule{{
			Name:       "chat",
			PathPrefix: "/v1/chat/completions",
			Methods:    []string{"POST"},
			ModelRouting: &resource.ModelRouting{Models: []resource.ModelRoute{
				{Model: "assistant", UpstreamRef: upstream.Name, UpstreamModel: "claude-sonnet"},
			}},
		}},
	})
	var userError *xerrors.UserError
	if !errors.As(err, &userError) {
		t.Fatalf("Service.Create(disabled upstream model) error = %T, want *xerrors.UserError", err)
	}
	if got, want := userError.Error(), "模型服务 \"Anthropic\" 未启用厂商模型 \"claude-sonnet\""; got != want {
		t.Errorf("Service.Create(disabled upstream model) error = %q, want %q", got, want)
	}
}

func TestValidModelUpstream(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*resource.Upstream)
		want   bool
	}{
		{name: "valid", want: true},
		{
			name: "provider protocol mismatch",
			mutate: func(upstream *resource.Upstream) {
				upstream.Spec.Protocol = resource.UpstreamProtocolGemini
			},
		},
		{
			name: "invalid API base path",
			mutate: func(upstream *resource.Upstream) {
				upstream.Spec.Model.APIBasePath = "/v1/"
			},
		},
		{
			name: "duplicate model name",
			mutate: func(upstream *resource.Upstream) {
				upstream.Spec.Model.Models = append(upstream.Spec.Model.Models, upstream.Spec.Model.Models[0])
			},
		},
		{
			name: "no enabled model",
			mutate: func(upstream *resource.Upstream) {
				upstream.Spec.Model.Models[0].Enabled = false
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &resource.Upstream{Spec: resource.UpstreamSpec{
				Type:     resource.UpstreamTypeModel,
				Protocol: resource.UpstreamProtocolAnthropic,
				Model: &resource.ModelSpec{
					Provider:    resource.ModelProviderAnthropic,
					APIBasePath: "/v1",
					Models: []resource.ModelCatalogItem{
						{Name: "claude-sonnet", DisplayName: "Claude Sonnet", Enabled: true},
					},
				},
			}}
			if tt.mutate != nil {
				tt.mutate(upstream)
			}

			if got := validModelUpstream(upstream); got != tt.want {
				t.Errorf("validModelUpstream(%q) = %t, want %t", tt.name, got, tt.want)
			}
		})
	}
}
