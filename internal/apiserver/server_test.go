package apiserver

import (
	"errors"
	"net"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/registry/generic"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

type unavailableRESTOptions struct {
	err error
}

func TestConfigFailureClosesPreparedListener(t *testing.T) {
	options, listener := servingTestOptions(t)
	cause := errors.New("admission initializer unavailable")
	options.RecommendedOptions.ExtraAdmissionInitializers = func(
		*genericapiserver.RecommendedConfig,
	) ([]admission.PluginInitializer, error) {
		return nil, cause
	}

	config, err := options.Config()

	if config != nil || !errors.Is(err, cause) {
		t.Fatalf("Config(failed initializer) = %v/%v, want nil and original cause", config, err)
	}

	if err := listener.SetDeadline(time.Now()); !errors.Is(err, net.ErrClosed) {
		t.Errorf("Config(failed initializer) listener error = %v, want closed listener", err)
	}
}

func TestStorageInstallationFailureClosesPreparedListener(t *testing.T) {
	options, listener := servingTestOptions(t)
	config, err := options.Config()
	if err != nil {
		t.Fatalf("Config(test serving) error = %v, want nil", err)
	}

	cause := errors.New("storage options unavailable")
	config.GenericConfig.RESTOptionsGetter = unavailableRESTOptions{err: cause}

	server, err := config.Complete().New()

	if server != nil || !errors.Is(err, cause) {
		t.Fatalf("New(failed storage) = %v/%v, want nil and original cause", server, err)
	}

	if err := listener.SetDeadline(time.Now()); !errors.Is(err, net.ErrClosed) {
		t.Errorf("New(failed storage) listener error = %v, want closed listener", err)
	}
}

func servingTestOptions(t *testing.T) (*Options, *net.TCPListener) {
	t.Helper()
	options, err := NewOptions()
	if err != nil {
		t.Fatalf("NewOptions() error = %v", err)
	}

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatalf("ListenTCP(test serving) error = %v", err)
	}

	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("Close(test serving) error = %v", err)
		}
	})
	options.RecommendedOptions.SecureServing.Listener = listener
	options.RecommendedOptions.SecureServing.ServerCert.CertDirectory = ""
	// 只测试原生组装失败的清理，不创建 etcd 连接或依赖宿主 Kubernetes。
	options.RecommendedOptions.Etcd = nil
	options.RecommendedOptions.Authentication = nil
	options.RecommendedOptions.Authorization = nil
	options.RecommendedOptions.CoreAPI = nil
	options.RecommendedOptions.Admission = nil
	options.RecommendedOptions.Features.EnablePriorityAndFairness = false

	return options, listener
}

func (o unavailableRESTOptions) GetRESTOptions(schema.GroupResource, runtime.Object) (generic.RESTOptions, error) {
	return generic.RESTOptions{}, o.err
}
