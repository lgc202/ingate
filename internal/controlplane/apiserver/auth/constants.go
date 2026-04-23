package auth

import userinfo "k8s.io/apiserver/pkg/authentication/user"

const (
	DefaultAdminToken       = "ingate-dev-admin-token"
	DefaultAdminUser        = "ingate-admin"
	DefaultViewerToken      = "ingate-dev-viewer-token"
	DefaultViewerUser       = "ingate-viewer"
	DefaultViewerGroup      = "ingate:viewers"
	DefaultAuthHeaderPrefix = "Bearer "
)

var DefaultAdminGroups = []string{userinfo.SystemPrivilegedGroup}

var DefaultViewerGroups = []string{DefaultViewerGroup}

var DefaultAnonymousPaths = []string{
	"/healthz",
	"/readyz",
	"/livez",
	"/apis",
	"/openapi",
	"/version",
}
