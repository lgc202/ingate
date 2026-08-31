package biz

import (
	"crypto/subtle"
	"errors"
	"time"

	"github.com/lgc202/ingate/internal/pkg/accesskey"
)

var (
	// ErrUnauthenticated 表示请求未携带当前有效的 Caller 访问密钥。
	ErrUnauthenticated = errors.New("caller authentication failed")
	// ErrForbidden 表示 Caller 已通过身份认证，但无权访问目标 Route。
	ErrForbidden = errors.New("caller is not authorized for route")
)

// Identity 是一次成功身份认证后写入请求记录的稳定归属。
type Identity struct {
	CallerID    string
	AccessKeyID string
}

// Credential 保存授权决策需要的最小 Caller 凭据信息。
// 摘要和 Route 授权集合在构造时复制，授权过程无法修改缓存中的原始配置。
type Credential struct {
	callerID  string
	digest    string
	routeIDs  map[string]struct{}
	expiresAt time.Time
}

// CredentialSource 提供当前已同步的 Caller 凭据。
// 接口定义在消费方，数据来源可以独立替换而不影响授权规则。
type CredentialSource interface {
	Lookup(keyID string) (Credential, bool)
}

// Authorizer 根据 Caller 凭据和 Route 授权关系作出访问决策。
type Authorizer struct {
	credentials CredentialSource
}

// NewAuthorizer 创建 Caller 授权服务。
func NewAuthorizer(credentials CredentialSource) *Authorizer {
	return &Authorizer{credentials: credentials}
}

// NewCredential 复制 Caller 当前有效的凭据和 Route 授权集合。
func NewCredential(
	callerID string,
	digest string,
	routeIDs []string,
	expiresAt time.Time,
) Credential {
	authorizedRoutes := make(map[string]struct{}, len(routeIDs))
	for _, routeID := range routeIDs {
		authorizedRoutes[routeID] = struct{}{}
	}
	return Credential{
		callerID:  callerID,
		digest:    digest,
		routeIDs:  authorizedRoutes,
		expiresAt: expiresAt,
	}
}

// Authorize 验证完整访问密钥并检查 Caller 是否有权访问 Route。
func (a *Authorizer) Authorize(value, routeID string) (Identity, error) {
	keyID, err := accesskey.ParseKeyID(value)
	if err != nil {
		return Identity{}, ErrUnauthenticated
	}
	credential, exists := a.credentials.Lookup(keyID)
	if !exists {
		return Identity{}, ErrUnauthenticated
	}
	actualDigest := accesskey.Digest(value)
	if subtle.ConstantTimeCompare([]byte(actualDigest), []byte(credential.digest)) != 1 {
		return Identity{}, ErrUnauthenticated
	}
	if !credential.expiresAt.IsZero() && !time.Now().Before(credential.expiresAt) {
		return Identity{}, ErrUnauthenticated
	}

	identity := Identity{CallerID: credential.callerID, AccessKeyID: keyID}
	if _, authorized := credential.routeIDs[routeID]; !authorized {
		// 密钥已经确认身份，只是缺少当前 Route 权限；保留身份用于记录越权请求
		return identity, ErrForbidden
	}
	return identity, nil
}
