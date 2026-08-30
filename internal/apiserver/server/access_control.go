package server

import (
	"fmt"

	apiserverconfig "k8s.io/apiserver/pkg/apis/apiserver"
	"k8s.io/apiserver/pkg/authentication/request/anonymous"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	requestunion "k8s.io/apiserver/pkg/authentication/request/union"
	"k8s.io/apiserver/pkg/authentication/token/tokenfile"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	"k8s.io/apiserver/pkg/authorization/path"
	authorizerunion "k8s.io/apiserver/pkg/authorization/union"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

const (
	controlPlaneUsername = "ingate-control-plane"
	healthzPath          = "/healthz"
	livezPath            = "/livez"
	readyzPath           = "/readyz"
)

func configureAccessControl(
	config *genericapiserver.RecommendedConfig,
	bearerToken string,
) error {
	controlPlaneUser := &user.DefaultInfo{
		Name:   controlPlaneUsername,
		UID:    controlPlaneUsername,
		Groups: []string{user.AllAuthenticated, user.SystemPrivilegedGroup},
	}
	config.Authentication.Authenticator = requestunion.NewFailOnError(
		bearertoken.New(tokenfile.New(map[string]*user.DefaultInfo{
			bearerToken: controlPlaneUser,
		})),
		anonymous.NewAuthenticator([]apiserverconfig.AnonymousAuthCondition{
			{Path: healthzPath},
			{Path: livezPath},
			{Path: readyzPath},
		}),
	)

	healthAuthorizer, err := path.NewAuthorizer([]string{
		healthzPath,
		livezPath,
		readyzPath,
	})
	if err != nil {
		return fmt.Errorf("create API Server health authorizer: %w", err)
	}
	config.Authorization.Authorizer = authorizerunion.New(
		healthAuthorizer,
		authorizerfactory.NewPrivilegedGroups(user.SystemPrivilegedGroup),
	)
	return nil
}
