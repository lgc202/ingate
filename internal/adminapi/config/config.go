package config

import (
	"fmt"

	appoptions "github.com/lgc202/ingate/internal/adminapi/app/options"
)

type Config struct {
	BindAddress                    string
	Port                           int
	APIServerAddress               string
	APIServerToken                 string
	APIServerInsecureSkipTLSVerify bool
	AdminToken                     string
}

func New(opts appoptions.CompletedOptions) Config {
	return Config{
		BindAddress:                    opts.BindAddress,
		Port:                           opts.Port,
		APIServerAddress:               opts.APIServerAddress,
		APIServerToken:                 opts.APIServerToken,
		APIServerInsecureSkipTLSVerify: opts.APIServerInsecureSkipTLSVerify,
		AdminToken:                     opts.AdminToken,
	}
}

func (c Config) ListenAddress() string {
	return fmt.Sprintf("%s:%d", c.BindAddress, c.Port)
}
