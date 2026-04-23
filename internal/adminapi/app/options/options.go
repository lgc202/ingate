package options

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

const (
	DefaultBindAddress                 = "127.0.0.1"
	DefaultPort                        = 18080
	DefaultAPIServerAddress            = "https://127.0.0.1:18443"
	DefaultAPIServerToken              = "ingate-dev-admin-token"
	DefaultAPIServerInsecureSkipVerify = true
	DefaultAdminToken                  = "ingate-dev-admin-api-token"
)

type Options struct {
	BindAddress                    string
	Port                           int
	APIServerAddress               string
	APIServerToken                 string
	APIServerInsecureSkipTLSVerify bool
	AdminToken                     string
}

type CompletedOptions struct {
	BindAddress                    string
	Port                           int
	APIServerAddress               string
	APIServerToken                 string
	APIServerInsecureSkipTLSVerify bool
	AdminToken                     string
}

func NewOptions() *Options {
	return &Options{
		BindAddress:                    DefaultBindAddress,
		Port:                           DefaultPort,
		APIServerAddress:               DefaultAPIServerAddress,
		APIServerToken:                 DefaultAPIServerToken,
		APIServerInsecureSkipTLSVerify: DefaultAPIServerInsecureSkipVerify,
		AdminToken:                     DefaultAdminToken,
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.BindAddress, "bind-address", o.BindAddress, "The IP address for ingate-admin-api to bind.")
	fs.IntVar(&o.Port, "port", o.Port, "The HTTP port for ingate-admin-api to serve on.")
	fs.StringVar(&o.APIServerAddress, "apiserver-address", o.APIServerAddress, "The ingate-apiserver base URL.")
	fs.StringVar(&o.APIServerToken, "apiserver-token", o.APIServerToken, "Bearer token used when calling ingate-apiserver.")
	fs.BoolVar(&o.APIServerInsecureSkipTLSVerify, "apiserver-insecure-skip-tls-verify", o.APIServerInsecureSkipTLSVerify, "Skip TLS verification when calling ingate-apiserver. Development only.")
	fs.StringVar(&o.AdminToken, "admin-token", o.AdminToken, "Bearer token required when calling admin API endpoints.")
}

func (o *Options) Complete() (CompletedOptions, error) {
	if o == nil {
		o = NewOptions()
	}
	return CompletedOptions{
		BindAddress:                    strings.TrimSpace(o.BindAddress),
		Port:                           o.Port,
		APIServerAddress:               strings.TrimSpace(o.APIServerAddress),
		APIServerToken:                 strings.TrimSpace(o.APIServerToken),
		APIServerInsecureSkipTLSVerify: o.APIServerInsecureSkipTLSVerify,
		AdminToken:                     strings.TrimSpace(o.AdminToken),
	}, nil
}

func (o CompletedOptions) Validate() []error {
	var errs []error
	if o.BindAddress == "" {
		errs = append(errs, fmt.Errorf("bind-address must not be empty"))
	}
	if o.Port < 1 || o.Port > 65535 {
		errs = append(errs, fmt.Errorf("port must be a valid TCP port"))
	}
	if o.APIServerAddress == "" {
		errs = append(errs, fmt.Errorf("apiserver-address must not be empty"))
	}
	if o.APIServerToken == "" {
		errs = append(errs, fmt.Errorf("apiserver-token must not be empty"))
	}
	if o.AdminToken == "" {
		errs = append(errs, fmt.Errorf("admin-token must not be empty"))
	}
	return errs
}
