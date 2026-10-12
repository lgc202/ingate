package gateway

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestResolveGatewayConflictAndHostSharing(t *testing.T) {
	gateways := []gatewayv1alpha1.Gateway{
		gatewayResource("later", 2, listener("free", 9090, ""), listener("conflict", 8080, "")),
		gatewayResource("first", 1, listener("http", 8080, "a.example.com")),
		gatewayResource("shared", 3, listener("http", 8080, "b.example.com")),
		gatewayResource("available", 4, listener("http", 9090, "")),
	}
	before := (&gatewayv1alpha1.GatewayList{Items: gateways}).DeepCopy().Items
	result := resolve(gateways, nil, nil)
	var names []string
	for _, gateway := range result.gateways {
		names = append(names, gateway.Name)
	}
	if diff := cmp.Diff([]string{"first", "shared", "available"}, names); diff != "" {
		t.Errorf("accepted gateways (-want +got):\n%s", diff)
	}
	if len(result.gatewayConflicts) != 1 || result.gatewayConflicts[0].gateway.Name != "later" {
		t.Errorf("conflicts = %+v; want whole later Gateway rejected", result.gatewayConflicts)
	}
	if diff := cmp.Diff(before, gateways); diff != "" {
		t.Errorf("input mutated:\n%s", diff)
	}
}

func gatewayResource(
	name string,
	created int64,
	listeners ...gatewayv1alpha1.Listener,
) gatewayv1alpha1.Gateway {
	return gatewayv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.NewTime(time.Unix(created, 0))},
		Spec:       gatewayv1alpha1.GatewaySpec{Listeners: listeners},
	}
}

func listener(name string, port int32, hostname string) gatewayv1alpha1.Listener {
	return gatewayv1alpha1.Listener{
		Name:     name,
		Protocol: gatewayv1alpha1.ProtocolHTTP,
		Port:     port,
		Hostname: hostname,
	}
}

func assertGatewayCondition(
	t *testing.T,
	apiClient client.Client,
	name string,
	status metav1.ConditionStatus,
	reason string,
	generation int64,
) {
	t.Helper()
	var gateway gatewayv1alpha1.Gateway
	if err := apiClient.Get(t.Context(), types.NamespacedName{Name: name}, &gateway); err != nil {
		t.Fatalf("Get(Gateway %q) error = %v", name, err)
	}
	assertAcceptedCondition(t, gateway.Status.Conditions, status, reason, generation)
}
