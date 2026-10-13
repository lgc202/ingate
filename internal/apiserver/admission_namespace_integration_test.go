//go:build integration

package apiserver

import (
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
	genericapiserver "k8s.io/apiserver/pkg/server"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// namespaceLifecycleCapture 捕获框架实际装配的插件，供测试调用，不替换生产准入链。
type namespaceLifecycleCapture struct {
	plugin *lifecycle.Lifecycle
}

func TestNamespaceLifecycleLoopback(t *testing.T) {
	// 用另一个隔离 Ingate Server 模拟宿主 CoreAPI，验证两套 Namespace 数据不会混用。
	hostConfig := startAPIServer(t)
	hostClient, err := typedcorev1.NewForConfig(hostConfig)
	if err != nil {
		t.Fatalf("create host core client error = %v", err)
	}
	ctx := t.Context()
	for _, name := range []string{"host-only", "shared"} {
		namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
		if _, err := hostClient.Namespaces().Create(ctx, namespace, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Create(host Namespace %q) error = %v", name, err)
		}
	}

	kubeconfig := clientcmdapi.NewConfig()
	kubeconfig.Clusters["host"] = &clientcmdapi.Cluster{
		Server:                   hostConfig.Host,
		CertificateAuthorityData: hostConfig.CAData,
	}
	kubeconfig.AuthInfos["host"] = &clientcmdapi.AuthInfo{}
	kubeconfig.Contexts["host"] = &clientcmdapi.Context{Cluster: "host", AuthInfo: "host"}
	kubeconfig.CurrentContext = "host"
	path := filepath.Join(t.TempDir(), "host.kubeconfig")
	if err := clientcmd.WriteToFile(*kubeconfig, path); err != nil {
		t.Fatalf("write isolated host kubeconfig error = %v", err)
	}

	options, err := NewOptions()
	if err != nil {
		t.Fatalf("NewOptions() error = %v", err)
	}
	options.RecommendedOptions.CoreAPI.CoreAPIKubeconfigPath = path
	options.RecommendedOptions.Authentication = nil
	options.RecommendedOptions.Authorization = nil
	options.RecommendedOptions.Features.EnablePriorityAndFairness = false
	// 只验证 Lifecycle 的真实启动和数据源；不把此用例当作完整默认准入链验证。
	options.RecommendedOptions.Admission.Plugins = admission.NewPlugins()
	lifecycle.Register(options.RecommendedOptions.Admission.Plugins)
	options.RecommendedOptions.Admission.RecommendedPluginOrder = []string{lifecycle.PluginName}
	capture := &namespaceLifecycleCapture{}
	options.RecommendedOptions.ExtraAdmissionInitializers = func(
		config *genericapiserver.RecommendedConfig,
	) ([]admission.PluginInitializer, error) {
		if config.ClientConfig == nil || config.ClientConfig.Host != hostConfig.Host {
			t.Error("admission host ClientConfig changed, want original host core API")
		}
		return []admission.PluginInitializer{capture}, nil
	}
	localConfig := startAPIServerWithOptions(t, options)
	if capture.plugin == nil {
		t.Fatal("ExtraAdmissionInitializers did not capture NamespaceLifecycle, want preserved initializer")
	}
	localClient, err := typedcorev1.NewForConfig(localConfig)
	if err != nil {
		t.Fatalf("create local core client error = %v", err)
	}
	for _, name := range []string{"local-only", "shared"} {
		namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
		if _, err := localClient.Namespaces().Create(ctx, namespace, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Create(local Namespace %q) error = %v", name, err)
		}
	}
	if err := localClient.Namespaces().Delete(ctx, "shared", metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Delete(local Namespace shared) error = %v", err)
	}

	tests := []struct {
		namespace  string
		wantReason metav1.StatusReason
	}{
		{namespace: "local-only"},
		{namespace: "host-only", wantReason: metav1.StatusReasonNotFound},
		{namespace: "shared", wantReason: metav1.StatusReasonForbidden},
	}
	for _, tt := range tests {
		// List/Watch 和实时查询走真实 HTTPS，业务属性则直接交给捕获的官方插件；
		// 当前 Gateway 仍是集群级，此处不能通过其 HTTP 入口验证 namespaced 创建。
		attributes := admission.NewAttributesRecord(
			&gatewayv1alpha1.Gateway{},
			nil,
			gatewayv1alpha1.SchemeGroupVersion.WithKind("Gateway"),
			tt.namespace,
			"edge",
			gatewayv1alpha1.SchemeGroupVersion.WithResource("gateways"),
			"",
			admission.Create,
			nil,
			false,
			nil,
		)
		err := capture.plugin.Admit(ctx, attributes, nil)
		if tt.wantReason == "" {
			if err != nil {
				t.Errorf("Admit(%q) error = %v, want nil", tt.namespace, err)
			}
			continue
		}
		if got := apierrors.ReasonForError(err); got != tt.wantReason {
			t.Errorf("Admit(%q) reason = %q, want %q (error = %v)", tt.namespace, got, tt.wantReason, err)
		}
	}
}

func (c *namespaceLifecycleCapture) Initialize(plugin admission.Interface) {
	if plugin, ok := plugin.(*lifecycle.Lifecycle); ok {
		c.plugin = plugin
	}
}
