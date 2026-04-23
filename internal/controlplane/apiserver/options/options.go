package options

import (
	"fmt"
	"net"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	cliflag "k8s.io/component-base/cli/flag"
	netutils "k8s.io/utils/net"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
	ingatescheme "github.com/lgc202/ingate/pkg/apis/scheme"
)

const defaultCertDirectory = "_output/certificates"

// Options contains the server options needed to boot a functional generic apiserver.
type Options struct {
	GenericServerRunOptions *genericoptions.ServerRunOptions
	SecureServing           *genericoptions.SecureServingOptionsWithLoopback
	Etcd                    *genericoptions.EtcdOptions
	Admission               *genericoptions.AdmissionOptions
	Auth                    *AuthOptions
	AlternateDNS            []string
}

type completedOptions struct {
	GenericServerRunOptions *genericoptions.ServerRunOptions
	SecureServing           *genericoptions.SecureServingOptionsWithLoopback
	Etcd                    *genericoptions.EtcdOptions
	Admission               *genericoptions.AdmissionOptions
	Auth                    *AuthOptions
	AlternateDNS            []string
}

// CompletedOptions is a private-wrapper style options object, following kube/OneX patterns.
type CompletedOptions struct {
	*completedOptions
}

func NewOptions() *Options {
	genericServerRunOptions := genericoptions.NewServerRunOptions()

	secureServing := genericoptions.NewSecureServingOptions().WithLoopback()
	secureServing.BindAddress = netutils.ParseIPSloppy("127.0.0.1")
	secureServing.BindPort = 18443
	secureServing.ServerCert.CertDirectory = defaultCertDirectory
	secureServing.ServerCert.PairName = "apiserver"

	storageCodec := ingatescheme.Codecs.LegacyCodec(
		gatewayv1alpha1.SchemeGroupVersion,
		policyv1alpha1.SchemeGroupVersion,
	)
	etcd := genericoptions.NewEtcdOptions(storagebackend.NewDefaultConfig("/ingate", storageCodec))
	etcd.StorageConfig.Transport.ServerList = []string{"http://127.0.0.1:2379"}
	etcd.DefaultStorageMediaType = runtime.ContentTypeJSON

	admissionOptions := genericoptions.NewAdmissionOptions()
	admissionOptions.Plugins = admission.NewPlugins()
	RegisterAllAdmissionPlugins(admissionOptions.Plugins)
	admissionOptions.RecommendedPluginOrder = append([]string(nil), AllOrderedPlugins...)
	admissionOptions.DefaultOffPlugins = DefaultOffAdmissionPlugins()

	return &Options{
		GenericServerRunOptions: genericServerRunOptions,
		SecureServing:           secureServing,
		Etcd:                    etcd,
		Admission:               admissionOptions,
		Auth:                    NewAuthOptions(),
		AlternateDNS:            []string{"localhost", "ingate.local"},
	}
}

func (o *Options) AddFlags(fss *cliflag.NamedFlagSets) {
	o.GenericServerRunOptions.AddUniversalFlags(fss.FlagSet("generic"))
	o.SecureServing.AddFlags(fss.FlagSet("secure serving"))
	o.Etcd.AddFlags(fss.FlagSet("etcd"))
	o.Admission.AddFlags(fss.FlagSet("admission"))
	o.Auth.AddFlags(fss)
}

func (o *Options) Complete() (CompletedOptions, error) {
	if o == nil {
		return CompletedOptions{completedOptions: &completedOptions{}}, nil
	}

	completed := &completedOptions{
		GenericServerRunOptions: o.GenericServerRunOptions,
		SecureServing:           o.SecureServing,
		Etcd:                    o.Etcd,
		Admission:               o.Admission,
		Auth:                    o.Auth,
		AlternateDNS:            append([]string(nil), o.AlternateDNS...),
	}

	if err := completed.GenericServerRunOptions.DefaultAdvertiseAddress(completed.SecureServing.SecureServingOptions); err != nil {
		return CompletedOptions{}, err
	}

	advertiseAddress := completed.GenericServerRunOptions.AdvertiseAddress
	if advertiseAddress == nil || advertiseAddress.IsUnspecified() {
		return CompletedOptions{}, fmt.Errorf("advertise address must be resolved before creating serving certificates")
	}

	if err := completed.SecureServing.MaybeDefaultWithSelfSignedCerts(
		advertiseAddress.String(),
		completed.AlternateDNS,
		[]net.IP{netutils.ParseIPSloppy("127.0.0.1")},
	); err != nil {
		return CompletedOptions{}, fmt.Errorf("failed to prepare secure serving certificates: %w", err)
	}

	return CompletedOptions{completedOptions: completed}, nil
}

func (o *Options) Validate() []error {
	var errs []error
	if o == nil {
		return errs
	}
	err := o.GenericServerRunOptions.Validate()
	err = append(err, o.SecureServing.Validate()...)
	err = append(err, o.Etcd.Validate()...)
	err = append(err, o.Admission.Validate()...)
	err = append(err, o.Auth.Validate()...)
	if strings.TrimSpace(o.Admission.ConfigFile) != "" {
		err = append(err, fmt.Errorf("admission-control-config-file is not supported in the current ingate apiserver build"))
	}
	return err
}
