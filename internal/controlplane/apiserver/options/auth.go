package options

import (
	"strings"

	cliflag "k8s.io/component-base/cli/flag"

	"github.com/lgc202/ingate/internal/controlplane/apiserver/auth"
)

type AuthOptions struct {
	AdminToken              string
	AdminUser               string
	AdminGroups             []string
	ViewerToken             string
	ViewerUser              string
	ViewerGroups            []string
	AnonymousPaths          []string
	AuthorizationPolicyFile string
}

func NewAuthOptions() *AuthOptions {
	return &AuthOptions{
		AdminToken:              auth.DefaultAdminToken,
		AdminUser:               auth.DefaultAdminUser,
		AdminGroups:             append([]string(nil), auth.DefaultAdminGroups...),
		ViewerToken:             auth.DefaultViewerToken,
		ViewerUser:              auth.DefaultViewerUser,
		ViewerGroups:            append([]string(nil), auth.DefaultViewerGroups...),
		AnonymousPaths:          append([]string(nil), auth.DefaultAnonymousPaths...),
		AuthorizationPolicyFile: "",
	}
}

func (o *AuthOptions) AddFlags(fs *cliflag.NamedFlagSets) {
	if o == nil {
		return
	}

	flagSet := fs.FlagSet("auth")
	flagSet.StringVar(&o.AdminToken, "auth-admin-token", o.AdminToken, "Bearer token for the built-in admin user.")
	flagSet.StringVar(&o.AdminUser, "auth-admin-user", o.AdminUser, "Username for the built-in admin user.")
	flagSet.StringSliceVar(&o.AdminGroups, "auth-admin-groups", o.AdminGroups, "Groups for the built-in admin user.")
	flagSet.StringVar(&o.ViewerToken, "auth-viewer-token", o.ViewerToken, "Bearer token for the built-in viewer user.")
	flagSet.StringVar(&o.ViewerUser, "auth-viewer-user", o.ViewerUser, "Username for the built-in viewer user.")
	flagSet.StringSliceVar(&o.ViewerGroups, "auth-viewer-groups", o.ViewerGroups, "Groups for the built-in viewer user.")
	flagSet.StringSliceVar(&o.AnonymousPaths, "auth-anonymous-paths", o.AnonymousPaths, "Non-resource URL path prefixes that are accessible without a bearer token.")
	flagSet.StringVar(&o.AuthorizationPolicyFile, "authz-policy-file", o.AuthorizationPolicyFile, "Optional YAML file that overrides the built-in static authorization policy.")
}

func (o *AuthOptions) Validate() []error {
	var errs []error
	if o == nil {
		return errs
	}
	if strings.TrimSpace(o.AdminToken) == "" {
		errs = append(errs, errRequiredOption("auth-admin-token"))
	}
	if strings.TrimSpace(o.AdminUser) == "" {
		errs = append(errs, errRequiredOption("auth-admin-user"))
	}
	if strings.TrimSpace(o.ViewerToken) == "" {
		errs = append(errs, errRequiredOption("auth-viewer-token"))
	}
	if strings.TrimSpace(o.ViewerUser) == "" {
		errs = append(errs, errRequiredOption("auth-viewer-user"))
	}
	return errs
}
