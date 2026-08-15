// Package authentication 提供 Console 登录参数和当前身份查询 API
package authentication

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminauth "github.com/lgc202/ingate/internal/adminapi/auth"
	"github.com/lgc202/ingate/internal/adminapi/conf"
)

// Service 提供不包含密钥的 OIDC 公共配置和当前请求身份
type Service struct {
	config *conf.Authentication
}

// NewService 创建认证协议服务
func NewService(config *conf.Authentication) *Service {
	return &Service{config: config}
}

func (s *Service) GetAuthenticationConfiguration(
	context.Context,
	*emptypb.Empty,
) (*adminv1.AuthenticationConfiguration, error) {
	reply := &adminv1.AuthenticationConfiguration{Enabled: s.config.GetEnabled()}
	if !reply.Enabled {
		return reply, nil
	}
	reply.Issuer = s.config.GetIssuer()
	reply.ClientId = s.config.GetClientId()
	reply.Scopes = append([]string(nil), s.config.GetScopes()...)
	return reply, nil
}

func (s *Service) GetCurrentPrincipal(ctx context.Context, _ *emptypb.Empty) (*adminv1.CurrentPrincipal, error) {
	principal, _ := adminauth.FromContext(ctx)
	return &adminv1.CurrentPrincipal{
		Subject: principal.Subject,
		Name:    principal.Name,
		Email:   principal.Email,
		Role:    string(principal.Role),
	}, nil
}
