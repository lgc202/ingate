// Package auth 验证 Admin API 的 OIDC 身份并执行固定的产品角色授权
package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/lgc202/ingate/internal/adminapi/conf"
)

var (
	// ErrCredentialsMissing 表示请求没有提供 Bearer Token
	ErrCredentialsMissing = errors.New("authentication credentials are missing")
	// ErrCredentialsInvalid 表示 Bearer Token 无法通过 OIDC 校验
	ErrCredentialsInvalid = errors.New("authentication credentials are invalid")
)

// Role 是 Ingate 管理面的固定权限级别
type Role string

const (
	RoleNone     Role = ""
	RoleViewer   Role = "viewer"
	RoleOperator Role = "operator"
	RoleAdmin    Role = "admin"
)

// Principal 是已验证请求在后续用例和审计中的稳定身份
type Principal struct {
	Subject string
	Name    string
	Email   string
	Role    Role
}

// Authenticator 复用 OIDC Discovery 和 JWKS 缓存校验每个请求的 JWT
type Authenticator struct {
	enabled       bool
	verifier      *oidc.IDTokenVerifier
	rolesClaim    string
	adminRoles    map[string]struct{}
	operatorRoles map[string]struct{}
	viewerRoles   map[string]struct{}
}

// NewAuthenticator 初始化单例 OIDC Verifier；禁用认证时不访问外部身份服务
func NewAuthenticator(config *conf.Authentication) (*Authenticator, error) {
	authenticator := &Authenticator{enabled: config.GetEnabled()}
	if !authenticator.enabled {
		return authenticator, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.GetDiscoveryTimeout().AsDuration())
	defer cancel()
	provider, err := oidc.NewProvider(ctx, config.GetIssuer())
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider: %w", err)
	}
	authenticator.verifier = provider.Verifier(&oidc.Config{ClientID: config.GetAudience()})
	authenticator.rolesClaim = config.GetRolesClaim()
	authenticator.adminRoles = stringSet(config.GetAdminRoles())
	authenticator.operatorRoles = stringSet(config.GetOperatorRoles())
	authenticator.viewerRoles = stringSet(config.GetViewerRoles())
	return authenticator, nil
}

// Authenticate 校验 Authorization Header，并只提取授权与审计需要的声明
func (a *Authenticator) Authenticate(ctx context.Context, authorization string) (Principal, error) {
	if !a.enabled {
		return Principal{Subject: "authentication-disabled", Name: "Local administrator", Role: RoleAdmin}, nil
	}
	scheme, rawToken, ok := strings.Cut(strings.TrimSpace(authorization), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return Principal{}, ErrCredentialsMissing
	}
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return Principal{}, ErrCredentialsMissing
	}
	token, err := a.verifier.Verify(ctx, rawToken)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: %w", ErrCredentialsInvalid, err)
	}
	var claims map[string]any
	if err := token.Claims(&claims); err != nil {
		return Principal{}, fmt.Errorf("%w: decode claims: %w", ErrCredentialsInvalid, err)
	}
	return Principal{
		Subject: token.Subject,
		Name:    firstString(claims, "name", "preferred_username"),
		Email:   claimString(claims["email"]),
		Role:    a.resolveRole(claimStringsAtPath(claims, a.rolesClaim)),
	}, nil
}

func (a *Authenticator) resolveRole(roles []string) Role {
	role := RoleNone
	for _, value := range roles {
		if _, exists := a.adminRoles[value]; exists {
			return RoleAdmin
		}
		if _, exists := a.operatorRoles[value]; exists {
			role = RoleOperator
			continue
		}
		if role == RoleNone {
			if _, exists := a.viewerRoles[value]; exists {
				role = RoleViewer
			}
		}
	}
	return role
}

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			set[value] = struct{}{}
		}
	}
	return set
}

func claimStringsAtPath(claims map[string]any, path string) []string {
	var value any = claims
	for part := range strings.SplitSeq(path, ".") {
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = object[part]
	}
	switch values := value.(type) {
	case string:
		return []string{values}
	case []any:
		result := make([]string, 0, len(values))
		for _, item := range values {
			if text := claimString(item); text != "" {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

func firstString(claims map[string]any, names ...string) string {
	for _, name := range names {
		if value := claimString(claims[name]); value != "" {
			return value
		}
	}
	return ""
}

func claimString(value any) string {
	text, _ := value.(string)
	return text
}
