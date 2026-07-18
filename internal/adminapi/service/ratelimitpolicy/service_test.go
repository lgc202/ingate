package ratelimitpolicy

import (
	"context"
	"errors"
	"sync"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/pkg/xerrors"
	gatewaystore "github.com/lgc202/ingate/internal/adminapi/store/gateway"
	ratelimitpolicystore "github.com/lgc202/ingate/internal/adminapi/store/ratelimitpolicy"
	routestore "github.com/lgc202/ingate/internal/adminapi/store/route"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
)

func TestServiceCreateStoresValidatedTargets(t *testing.T) {
	ctx := context.Background()
	client := clientfake.NewSimpleClientset()
	gateway := &resource.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gateway-1"},
		Spec:       resource.GatewaySpec{DisplayName: "生产网关"},
	}
	if _, err := client.GatewayV1().Gateways().Create(ctx, gateway, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Gateways.Create(%q) error: %v", gateway.Name, err)
	}
	service := New(
		ratelimitpolicystore.New(client),
		gatewaystore.New(client),
		routestore.New(client),
	)

	policyID, err := service.Create(ctx, CreatePolicyParams{PolicyParams: testPolicyParams([]TargetParams{{Kind: resource.KindGateway, ID: gateway.Name}})})
	if err != nil {
		t.Fatalf("Service.Create() error: %v", err)
	}
	policy, err := client.GatewayV1().RateLimitPolicies().Get(ctx, policyID, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("RateLimitPolicies.Get(%q) error: %v", policyID, err)
	}
	want := resource.PolicyTargetRef{Kind: resource.KindGateway, Name: gateway.Name}
	if len(policy.Spec.TargetRefs) != 1 || policy.Spec.TargetRefs[0] != want {
		t.Errorf("stored target refs = %v, want [%v]", policy.Spec.TargetRefs, want)
	}
}

func TestServiceCreateRejectsMissingTarget(t *testing.T) {
	client := clientfake.NewSimpleClientset()
	service := New(
		ratelimitpolicystore.New(client),
		gatewaystore.New(client),
		routestore.New(client),
	)

	_, err := service.Create(context.Background(), CreatePolicyParams{PolicyParams: testPolicyParams([]TargetParams{{Kind: resource.KindRoute, ID: "missing-route"}})})
	var userError *xerrors.UserError
	if !errors.As(err, &userError) {
		t.Fatalf("Service.Create(missing route) error = %T, want *xerrors.UserError", err)
	}
}

func TestServiceCreateSerializesDisplayNameValidation(t *testing.T) {
	client := clientfake.NewSimpleClientset()
	service := New(
		ratelimitpolicystore.New(client),
		gatewaystore.New(client),
		routestore.New(client),
	)

	const attempts = 12
	start := make(chan struct{})
	results := make(chan error, attempts)
	var waitGroup sync.WaitGroup
	for range attempts {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			_, err := service.Create(context.Background(), CreatePolicyParams{PolicyParams: testPolicyParams(nil)})
			results <- err
		}()
	}
	close(start)
	waitGroup.Wait()
	close(results)

	successes := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		var userError *xerrors.UserError
		if !errors.As(err, &userError) {
			t.Errorf("Service.Create(concurrent duplicate) error = %T, want *xerrors.UserError", err)
		}
	}
	if successes != 1 {
		t.Errorf("Service.Create(concurrent duplicate) successes = %d, want 1", successes)
	}
}

func testPolicyParams(targets []TargetParams) PolicyParams {
	return PolicyParams{
		Name:    "公共限流",
		Enabled: true,
		Targets: targets,
		Rules: []resource.RateLimitRule{
			{
				Name: "default",
				Key:  resource.RateLimitKey{Parts: []resource.RateLimitKeyPart{{Type: resource.RateLimitKeyTypeIP}}},
				Limit: resource.RateLimitQuota{
					Requests:      100,
					WindowSeconds: 60,
				},
			},
		},
	}
}
