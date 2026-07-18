package upstream

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	upstreamstore "github.com/lgc202/ingate/internal/adminapi/store/upstream"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
)

func TestServiceUpdateRejectsModelProtocolWhileOrdinaryRouteReferencesUpstream(t *testing.T) {
	ctx := context.Background()
	upstream := testUpstream("application", resource.UpstreamTypeApplication, resource.UpstreamProtocolHTTP)
	route := &resource.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route-1"},
		Spec: resource.RouteSpec{
			DisplayName: "订单接口",
			Rules: []resource.RouteRule{{
				Name:         "main",
				UpstreamRefs: []resource.UpstreamRef{{Name: upstream.Name, Weight: 100}},
			}},
		},
	}
	service := newTestService(t, ctx, upstream, route)

	err := service.Update(ctx, upstream.Name, UpdateUpstreamParams{
		Version: upstream.ResourceVersion,
		UpstreamParams: UpstreamParams{
			Name:     upstream.Spec.DisplayName,
			Type:     resource.UpstreamTypeModel,
			Protocol: resource.UpstreamProtocolOpenAI,
		},
	})
	var userError *xerrors.UserError
	if !errors.As(err, &userError) {
		t.Fatalf("Service.Update(ordinary route reference) error = %T, want *xerrors.UserError", err)
	}
	if got, want := userError.Error(), "服务仍被普通路由 \"订单接口\" 引用，不能改为 OpenAI 模型服务"; got != want {
		t.Errorf("Service.Update(ordinary route reference) error = %q, want %q", got, want)
	}
}

func TestServiceUpdateRejectsHTTPProtocolWhileModelRouteReferencesUpstream(t *testing.T) {
	ctx := context.Background()
	upstream := testUpstream("model", resource.UpstreamTypeModel, resource.UpstreamProtocolOpenAI)
	route := &resource.Route{
		ObjectMeta: metav1.ObjectMeta{Name: "route-1"},
		Spec: resource.RouteSpec{
			DisplayName: "模型对话",
			Rules: []resource.RouteRule{{
				Name: "chat",
				ModelRouting: &resource.ModelRouting{
					UpstreamRef: upstream.Name,
					Models:      []resource.ModelRoute{{Model: "assistant"}},
				},
			}},
		},
	}
	service := newTestService(t, ctx, upstream, route)

	err := service.Update(ctx, upstream.Name, UpdateUpstreamParams{
		Version: upstream.ResourceVersion,
		UpstreamParams: UpstreamParams{
			Name:     upstream.Spec.DisplayName,
			Type:     resource.UpstreamTypeApplication,
			Protocol: resource.UpstreamProtocolHTTP,
		},
	})
	var userError *xerrors.UserError
	if !errors.As(err, &userError) {
		t.Fatalf("Service.Update(model route reference) error = %T, want *xerrors.UserError", err)
	}
	if got, want := userError.Error(), "服务仍被模型路由 \"模型对话\" 引用，必须保持为 OpenAI 兼容大模型服务"; got != want {
		t.Errorf("Service.Update(model route reference) error = %q, want %q", got, want)
	}
}

func TestServiceUpdateAPIKey(t *testing.T) {
	tests := []struct {
		name       string
		params     UpdateUpstreamParams
		wantAPIKey string
	}{
		{
			name: "omitted API key preserves current value",
			params: UpdateUpstreamParams{
				UpstreamParams: modelUpstreamParams(),
			},
			wantAPIKey: "existing-secret",
		},
		{
			name: "provided API key replaces current value",
			params: UpdateUpstreamParams{
				UpstreamParams: modelUpstreamParams(),
				APIKey:         &APIKeyParams{Value: "rotated-secret"},
			},
			wantAPIKey: "rotated-secret",
		},
		{
			name: "remove API key clears authentication",
			params: UpdateUpstreamParams{
				UpstreamParams: modelUpstreamParams(),
				RemoveAPIKey:   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			upstream := testUpstream("model", resource.UpstreamTypeModel, resource.UpstreamProtocolOpenAI)
			upstream.Spec.TLS = &resource.UpstreamTLS{ServerName: "api.example.com"}
			upstream.Spec.Authentication = &resource.UpstreamAuthentication{
				APIKey: &resource.APIKeyAuthentication{Value: "existing-secret"},
			}
			service := newTestService(t, ctx, upstream, nil)
			tt.params.Version = upstream.ResourceVersion

			if err := service.Update(ctx, upstream.Name, tt.params); err != nil {
				t.Fatalf("Service.Update(%q) error = %v, want nil", tt.name, err)
			}
			updated, err := service.store.Get(ctx, upstream.Name)
			if err != nil {
				t.Fatalf("Store.Get(%q) error = %v, want nil", upstream.Name, err)
			}
			if tt.wantAPIKey == "" {
				if updated.Spec.Authentication != nil {
					t.Errorf("Service.Update(%q) authentication = %#v, want nil", tt.name, updated.Spec.Authentication)
				}
				return
			}
			if updated.Spec.Authentication == nil || updated.Spec.Authentication.APIKey == nil {
				t.Fatalf("Service.Update(%q) API key = nil, want %q", tt.name, tt.wantAPIKey)
			}
			if got := updated.Spec.Authentication.APIKey.Value; got != tt.wantAPIKey {
				t.Errorf("Service.Update(%q) API key = %q, want %q", tt.name, got, tt.wantAPIKey)
			}
		})
	}
}

func TestServiceUpdateRejectsPreservedAPIKeyWithoutTLS(t *testing.T) {
	ctx := context.Background()
	upstream := testUpstream("model", resource.UpstreamTypeModel, resource.UpstreamProtocolOpenAI)
	upstream.Spec.TLS = &resource.UpstreamTLS{ServerName: "api.example.com"}
	upstream.Spec.Authentication = &resource.UpstreamAuthentication{
		APIKey: &resource.APIKeyAuthentication{Value: "existing-secret"},
	}
	service := newTestService(t, ctx, upstream, nil)

	err := service.Update(ctx, upstream.Name, UpdateUpstreamParams{
		Version:        upstream.ResourceVersion,
		UpstreamParams: UpstreamParams{Name: upstream.Spec.DisplayName, Type: upstream.Spec.Type, Protocol: upstream.Spec.Protocol},
	})
	var userError *xerrors.UserError
	if !errors.As(err, &userError) {
		t.Fatalf("Service.Update(preserved API key without TLS) error = %T, want *xerrors.UserError", err)
	}
}

func newTestService(t *testing.T, ctx context.Context, upstream *resource.Upstream, route *resource.Route) *Service {
	t.Helper()
	client := clientfake.NewSimpleClientset()
	if _, err := client.GatewayV1().Upstreams().Create(ctx, upstream, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Upstreams.Create(%q) error = %v, want nil", upstream.Name, err)
	}
	if route != nil {
		if _, err := client.GatewayV1().Routes().Create(ctx, route, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Routes.Create(%q) error = %v, want nil", route.Name, err)
		}
	}
	return New(upstreamstore.New(client), routestore.New(client))
}

func modelUpstreamParams() UpstreamParams {
	return UpstreamParams{
		Name:     "model",
		Type:     resource.UpstreamTypeModel,
		Protocol: resource.UpstreamProtocolOpenAI,
		TLS:      &TLSParams{ServerName: "api.example.com"},
	}
}

func testUpstream(name string, upstreamType resource.UpstreamType, protocol resource.UpstreamProtocol) *resource.Upstream {
	return &resource.Upstream{
		ObjectMeta: metav1.ObjectMeta{Name: name, ResourceVersion: "1"},
		Spec: resource.UpstreamSpec{
			DisplayName: name,
			Type:        upstreamType,
			Protocol:    protocol,
		},
	}
}
