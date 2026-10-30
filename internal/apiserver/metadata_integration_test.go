//go:build integration

package apiserver

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/metadata"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	gatewayclient "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// TestMetadataDeleteCollection 验证 Namespace 清理所用原生客户端的真实内容协商和删除范围。
func TestMetadataDeleteCollection(t *testing.T) {
	config := startAPIServer(t)
	coreClient, err := typedcorev1.NewForConfig(config)
	if err != nil {
		t.Fatalf("create core client error = %v", err)
	}

	client, err := gatewayclient.NewForConfig(config)
	if err != nil {
		t.Fatalf("create Gateway client error = %v", err)
	}

	metadataClient, err := metadata.NewForConfig(config)
	if err != nil {
		t.Fatalf("create metadata client error = %v", err)
	}

	ctx := t.Context()
	for _, name := range []string{"team-a", "team-b"} {
		if _, err := coreClient.Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: name},
		}, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Create(Namespace %q) error = %v", name, err)
		}
	}

	for _, tt := range []struct {
		namespace, name, owner string
	}{
		{namespace: "team-a", name: "selected", owner: "cleanup"},
		{namespace: "team-a", name: "retained", owner: "other"},
		{namespace: "team-b", name: "selected", owner: "cleanup"},
	} {
		_, err := client.GatewayV1alpha1().Backends(tt.namespace).Create(ctx, &gatewayv1alpha1.Backend{
			ObjectMeta: metav1.ObjectMeta{Name: tt.name, Labels: map[string]string{"owner": tt.owner}},
			Spec: gatewayv1alpha1.BackendSpec{
				Endpoints: []gatewayv1alpha1.Endpoint{{Address: "127.0.0.1", Port: 8080, Weight: 1}},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Create(Backend %s/%s) error = %v", tt.namespace, tt.name, err)
		}
	}

	resources := metadataClient.Resource(gatewayv1alpha1.SchemeGroupVersion.WithResource("backends")).Namespace("team-a")
	if err := resources.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: "owner=cleanup"}); err != nil {
		t.Fatalf("metadata.DeleteCollection(team-a, owner=cleanup) error = %v, want nil", err)
	}

	if _, err := client.GatewayV1alpha1().Backends("team-a").Get(ctx, "selected", metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Errorf("Get(deleted Backend team-a/selected) error = %v, want NotFound", err)
	}

	for _, tt := range []struct{ namespace, name string }{
		{namespace: "team-a", name: "retained"},
		{namespace: "team-b", name: "selected"},
	} {
		if _, err := client.GatewayV1alpha1().Backends(tt.namespace).Get(ctx, tt.name, metav1.GetOptions{}); err != nil {
			t.Errorf("Get(retained Backend %s/%s) error = %v, want nil", tt.namespace, tt.name, err)
		}
	}

	if err := resources.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{}); err != nil {
		t.Fatalf("metadata.DeleteCollection(team-a, all) error = %v, want nil", err)
	}

	remaining, err := resources.List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("metadata.List(team-a after cleanup) error = %v", err)
	}

	if len(remaining.Items) != 0 {
		t.Errorf("metadata.List(team-a after cleanup) count = %d, want 0", len(remaining.Items))
	}
}
