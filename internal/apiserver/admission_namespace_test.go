package apiserver

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/component-base/featuregate"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// externalClientProbe 模拟另一个需要宿主依赖的插件，验证本地初始化器不会修改其接线。
type externalClientProbe struct {
	*admission.Handler
	client    kubernetes.Interface
	informers informers.SharedInformerFactory
}

func TestNamespaceLifecycleDataSource(t *testing.T) {
	// 故意构造两套不同空间和同名阶段冲突；若误用宿主数据源，后面的拒绝断言会失败。
	local := fake.NewClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "local-only"}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "shared"}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceTerminating}},
	)
	host := fake.NewClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "host-only"}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "shared"}, Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive}},
	)
	localFactory := informers.NewSharedInformerFactory(local, 0)
	hostFactory := informers.NewSharedInformerFactory(host, 0)
	localInitializer := &namespaceLifecycleInitializer{client: local, informers: localFactory}
	immortalNamespaces := sets.New[string](metav1.NamespaceDefault, metav1.NamespaceSystem, metav1.NamespacePublic)
	plugin, err := lifecycle.NewLifecycle(immortalNamespaces)
	if err != nil {
		t.Fatalf("create NamespaceLifecycle error = %v", err)
	}
	hostInitializer := initializer.New(host, nil, hostFactory, nil, featuregate.NewFeatureGate(), nil, nil, nil)
	chain := admission.PluginInitializers{hostInitializer, localInitializer}
	chain.Initialize(plugin)
	if err := admission.ValidateInitialization(plugin); err != nil {
		t.Fatalf("ValidateInitialization(NamespaceLifecycle) error = %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(func() {
		cancel()
		localFactory.Shutdown()
		hostFactory.Shutdown()
	})
	localFactory.StartWithContext(ctx)
	if err := localFactory.WaitForCacheSyncWithContext(ctx).AsError(); err != nil {
		t.Fatalf("WaitForCacheSync(local Namespace) error = %v", err)
	}

	tests := []struct {
		name, namespace string
		operation       admission.Operation
		wantReason      metav1.StatusReason
	}{
		{name: "local_active", namespace: "local-only", operation: admission.Create},
		{name: "host_only", namespace: "host-only", operation: admission.Create, wantReason: metav1.StatusReasonNotFound},
		{name: "local_terminating_host_active", namespace: "shared", operation: admission.Create, wantReason: metav1.StatusReasonForbidden},
		{name: "update_missing", namespace: "missing", operation: admission.Update},
		{name: "delete_missing", namespace: "missing", operation: admission.Delete},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 直接构造 namespaced 请求属性，只检查插件行为；业务 HTTP 作用域尚未切换。
			attributes := admission.NewAttributesRecord(
				&gatewayv1alpha1.Gateway{},
				nil,
				gatewayv1alpha1.SchemeGroupVersion.WithKind("Gateway"),
				tt.namespace,
				"edge",
				gatewayv1alpha1.SchemeGroupVersion.WithResource("gateways"),
				"",
				tt.operation,
				nil,
				false,
				nil,
			)
			err := plugin.Admit(ctx, attributes, nil)
			if tt.wantReason == "" {
				if err != nil {
					t.Errorf("Admit(%s, %q) error = %v, want nil", tt.operation, tt.namespace, err)
				}
				return
			}
			if got := apierrors.ReasonForError(err); got != tt.wantReason {
				t.Errorf("Admit(%s, %q) reason = %q, want %q (error = %v)",
					tt.operation, tt.namespace, got, tt.wantReason, err)
			}
		})
	}
	if actions := host.Actions(); len(actions) != 0 {
		t.Errorf("NamespaceLifecycle host actions = %v, want none", actions)
	}

	probe := &externalClientProbe{Handler: admission.NewHandler(admission.Create)}
	chain.Initialize(probe)
	if probe.client != host || probe.informers != hostFactory {
		t.Error("non-NamespaceLifecycle plugin received local dependencies, want unchanged host dependencies")
	}
}

func TestNamespaceLifecycleInitializerRequiresLoopback(t *testing.T) {
	_, config := storageTestOptions(t)
	if _, err := newNamespaceLifecycleInitializer(config); err == nil {
		t.Error("newNamespaceLifecycleInitializer(nil loopback) error = nil, want missing loopback")
	}
	config.LoopbackClientConfig = &rest.Config{Host: "://invalid"}
	if _, err := newNamespaceLifecycleInitializer(config); err == nil {
		t.Error("newNamespaceLifecycleInitializer(invalid loopback) error = nil, want invalid client config")
	}
}

func TestAdmissionDisabledDoesNotInitializeNamespace(t *testing.T) {
	options, config := storageTestOptions(t)
	options.RecommendedOptions.Etcd = nil
	options.RecommendedOptions.SecureServing = nil
	options.RecommendedOptions.Authentication = nil
	options.RecommendedOptions.Authorization = nil
	options.RecommendedOptions.CoreAPI = nil
	options.RecommendedOptions.Admission = nil
	options.RecommendedOptions.Features.EnablePriorityAndFairness = false
	called := false
	options.RecommendedOptions.ExtraAdmissionInitializers = func(
		*genericapiserver.RecommendedConfig,
	) ([]admission.PluginInitializer, error) {
		called = true
		return nil, nil
	}

	namespaceInitializer, err := options.applyRecommendedOptions(config)
	if err != nil {
		t.Fatalf("applyRecommendedOptions(disabled Admission) error = %v", err)
	}
	if namespaceInitializer != nil || !called {
		t.Errorf("applyRecommendedOptions(disabled Admission) initializer/called = %v/%t, want nil/true",
			namespaceInitializer, called)
	}
	if _, ok := config.PostStartHooks[namespaceInformerHook]; ok {
		t.Error("disabled Admission registers Namespace informer hook, want none")
	}
}

func TestAdmissionExtraInitializerError(t *testing.T) {
	options, config := storageTestOptions(t)
	options.RecommendedOptions.Etcd = nil
	options.RecommendedOptions.SecureServing = nil
	options.RecommendedOptions.Authentication = nil
	options.RecommendedOptions.Authorization = nil
	options.RecommendedOptions.CoreAPI = nil
	options.RecommendedOptions.Features.EnablePriorityAndFairness = false
	wantErr := errors.New("extra initializer unavailable")
	options.RecommendedOptions.ExtraAdmissionInitializers = func(
		*genericapiserver.RecommendedConfig,
	) ([]admission.PluginInitializer, error) {
		return nil, wantErr
	}

	namespaceInitializer, err := options.applyRecommendedOptions(config)
	if !errors.Is(err, wantErr) || namespaceInitializer != nil {
		t.Errorf("applyRecommendedOptions(extra initializer failure) = %v/%v, want nil and wrapped %v",
			namespaceInitializer, err, wantErr)
	}
	if _, ok := config.PostStartHooks[namespaceInformerHook]; ok {
		t.Error("failed extra initializer registers Namespace informer hook, want none")
	}
	if _, err := options.RecommendedOptions.ExtraAdmissionInitializers(config); !errors.Is(err, wantErr) {
		t.Errorf("original ExtraAdmissionInitializers error = %v, want %v", err, wantErr)
	}
}

func (p *externalClientProbe) SetExternalKubeClientSet(client kubernetes.Interface) {
	p.client = client
}

func (p *externalClientProbe) SetExternalKubeInformerFactory(factory informers.SharedInformerFactory) {
	p.informers = factory
}

func (p *externalClientProbe) ValidateInitialization() error {
	return nil
}
