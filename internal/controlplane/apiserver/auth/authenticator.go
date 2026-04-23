package auth

import (
	"fmt"
	"net/http"
	"strings"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	requestbearertoken "k8s.io/apiserver/pkg/authentication/request/bearertoken"
	requestunion "k8s.io/apiserver/pkg/authentication/request/union"
	tokenfile "k8s.io/apiserver/pkg/authentication/token/tokenfile"
	userinfo "k8s.io/apiserver/pkg/authentication/user"
)

type prefixAnonymousAuthenticator struct {
	allowedPrefixes []string
}

func NewRequestAuthenticator(adminToken, adminUser string, adminGroups []string, viewerToken, viewerUser string, viewerGroups, anonymousPaths []string) (authenticator.Request, error) {
	if adminToken == "" {
		return nil, fmt.Errorf("admin token must not be empty")
	}
	if adminUser == "" {
		return nil, fmt.Errorf("admin user must not be empty")
	}
	if viewerToken == "" {
		return nil, fmt.Errorf("viewer token must not be empty")
	}
	if viewerUser == "" {
		return nil, fmt.Errorf("viewer user must not be empty")
	}

	adminResolvedGroups := append([]string(nil), adminGroups...)
	if len(adminResolvedGroups) == 0 {
		adminResolvedGroups = append(adminResolvedGroups, DefaultAdminGroups...)
	}

	viewerResolvedGroups := append([]string(nil), viewerGroups...)
	if len(viewerResolvedGroups) == 0 {
		viewerResolvedGroups = append(viewerResolvedGroups, DefaultViewerGroups...)
	}

	tokenAuthenticator := tokenfile.New(map[string]*userinfo.DefaultInfo{
		adminToken: {
			Name:   adminUser,
			Groups: adminResolvedGroups,
		},
		viewerToken: {
			Name:   viewerUser,
			Groups: viewerResolvedGroups,
		},
	})

	return requestunion.New(
		requestbearertoken.New(tokenAuthenticator),
		prefixAnonymousAuthenticator{allowedPrefixes: append([]string(nil), anonymousPaths...)},
	), nil
}

func (a prefixAnonymousAuthenticator) AuthenticateRequest(req *http.Request) (*authenticator.Response, bool, error) {
	for _, prefix := range a.allowedPrefixes {
		if req.URL.Path == prefix || strings.HasPrefix(req.URL.Path, prefix+"/") {
			return &authenticator.Response{User: &userinfo.DefaultInfo{
				Name:   userinfo.Anonymous,
				Groups: []string{userinfo.AllUnauthenticated},
			}}, true, nil
		}
	}

	return nil, false, nil
}
