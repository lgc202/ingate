package controllermanager

import (
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestOptionsConfigRegistersExternalTypes(t *testing.T) {
	options := NewOptions()
	options.AIConfigNamespace = "control-plane"
	options.AIIngressSocket = "/run/ingate/ingress.sock"
	options.AIEgressSocket = "/run/ingate/egress.sock"
	options.Kubeconfig = filepath.Join(t.TempDir(), "kubeconfig")
	kubeconfig := clientcmdapi.Config{
		Clusters:       map[string]*clientcmdapi.Cluster{"ingate": {Server: "https://127.0.0.1:6443"}},
		Contexts:       map[string]*clientcmdapi.Context{"ingate": {Cluster: "ingate"}},
		CurrentContext: "ingate",
	}
	if err := clientcmd.WriteToFile(kubeconfig, options.Kubeconfig); err != nil {
		t.Fatalf("WriteToFile(temporary kubeconfig) error = %v, want nil", err)
	}

	config, err := options.Config()
	if err != nil {
		t.Fatalf("Options.Config(temporary kubeconfig) error = %v, want nil", err)
	}

	if config.AIConfigNamespace != options.AIConfigNamespace {
		t.Errorf("Options.Config() AIConfigNamespace = %q, want %q", config.AIConfigNamespace, options.AIConfigNamespace)
	}

	if config.AIEgressSocket != options.AIEgressSocket {
		t.Errorf("Options.Config() AIEgressSocket = %q, want %q", config.AIEgressSocket, options.AIEgressSocket)
	}

	if config.AIIngressSocket != options.AIIngressSocket {
		t.Errorf("Options.Config() AIIngressSocket = %q, want %q", config.AIIngressSocket, options.AIIngressSocket)
	}

	for _, kind := range []schema.GroupVersionKind{
		corev1.SchemeGroupVersion.WithKind("Namespace"),
		corev1.SchemeGroupVersion.WithKind("NamespaceList"),
		corev1.SchemeGroupVersion.WithKind("ConfigMap"),
		corev1.SchemeGroupVersion.WithKind("ConfigMapList"),
		gatewayv1alpha1.SchemeGroupVersion.WithKind("Gateway"),
	} {
		if !config.ManagerOptions.Scheme.Recognizes(kind) {
			t.Errorf("Options.Config() Scheme.Recognizes(%s) = false, want true", kind)
		}
	}

	internalNamespace := corev1.SchemeGroupVersion.WithKind("Namespace")
	internalNamespace.Version = runtime.APIVersionInternal
	if config.ManagerOptions.Scheme.Recognizes(internalNamespace) {
		t.Errorf("Options.Config() Scheme.Recognizes(%s) = true, want external types only", internalNamespace)
	}
}

func TestOptionsAIConfigNamespaceFlag(t *testing.T) {
	options := NewOptions()
	flags := pflag.NewFlagSet(componentName, pflag.ContinueOnError)
	options.AddFlags(flags)
	if options.AIConfigNamespace != "" {
		t.Errorf("NewOptions() AIConfigNamespace = %q, want no default namespace", options.AIConfigNamespace)
	}

	if err := flags.Parse([]string{"--ai-config-namespace=control-plane"}); err != nil {
		t.Fatalf("Parse(AI namespace flag) error = %v", err)
	}

	if options.AIConfigNamespace != "control-plane" {
		t.Errorf("Parse(AI namespace flag) AIConfigNamespace = %q, want control-plane", options.AIConfigNamespace)
	}
}

func TestOptionsConfigRejectsInvalidAIConfigNamespace(t *testing.T) {
	for _, namespace := range []string{"", "Control-Plane", "team/a"} {
		t.Run(namespace, func(t *testing.T) {
			options := NewOptions()
			options.AIConfigNamespace = namespace

			config, err := options.Config()

			if err == nil || config != nil {
				t.Errorf("Options.Config(namespace %q) config/error = %v/%v, want nil config and validation error", namespace, config, err)
			}
		})
	}
}

func TestOptionsAISockets(t *testing.T) {
	options := NewOptions()
	flags := pflag.NewFlagSet(componentName, pflag.ContinueOnError)
	options.AddFlags(flags)

	err := flags.Parse([]string{"--ai-ingress-socket=/run/ingate/ingress.sock", "--ai-egress-socket=/run/ingate/egress.sock"})
	if err != nil || options.AIEgressSocket != "/run/ingate/egress.sock" {
		t.Errorf("Parse(AI socket) = (%q, %v), want configured absolute path", options.AIEgressSocket, err)
	}

	if options.AIIngressSocket != "/run/ingate/ingress.sock" {
		t.Errorf("Parse(AI socket) ingress = %q, want configured absolute path", options.AIIngressSocket)
	}

	for _, socket := range []string{"relative.sock", "/run/invalid\x00.sock"} {
		options.AIEgressSocket = socket

		config, err := options.Config()

		if err == nil || config != nil {
			t.Errorf("Options.Config(socket %q) = (%v, %v), want nil and validation error before API connection", socket, config, err)
		}
	}
}

func TestValidateAISockets(t *testing.T) {
	for _, tt := range []struct {
		name    string
		ingress string
		egress  string
		wantErr bool
	}{
		{name: "no models"},
		{name: "separate paths", ingress: "/run/ingate/in.sock", egress: "/run/ingate/out.sock"},
		{name: "relative ingress", ingress: "in.sock", wantErr: true},
		{name: "NUL ingress", ingress: "/run/invalid\x00.sock", wantErr: true},
		{name: "same path", ingress: "/run/ingate/ai.sock", egress: "/run/ingate/./ai.sock", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAISockets(tt.ingress, tt.egress)

			if (err != nil) != tt.wantErr {
				t.Errorf("validateAISockets(%q, %q) error = %v, want error %t", tt.ingress, tt.egress, err, tt.wantErr)
			}
		})
	}
}
