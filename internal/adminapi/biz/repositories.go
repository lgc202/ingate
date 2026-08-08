package biz

import (
	"context"
	"errors"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

var (
	// ErrAccessKeyNotFound 表示访问密钥不存在
	ErrAccessKeyNotFound = errors.New("access key not found")
	// ErrAccessKeyNameConflict 表示访问密钥名称违反唯一约束
	ErrAccessKeyNameConflict = errors.New("access key name already exists")
)

type GatewayRepository interface {
	List(context.Context) (*resource.GatewayList, error)
	Get(context.Context, string) (*resource.Gateway, error)
	Create(context.Context, *resource.Gateway) (*resource.Gateway, error)
	Update(context.Context, *resource.Gateway) (*resource.Gateway, error)
	Delete(context.Context, string) error
}

type RouteRepository interface {
	List(context.Context) (*resource.RouteList, error)
	Get(context.Context, string) (*resource.Route, error)
	Create(context.Context, *resource.Route) (*resource.Route, error)
	Update(context.Context, *resource.Route) (*resource.Route, error)
	Delete(context.Context, string) error
}

type UpstreamRepository interface {
	List(context.Context) (*resource.UpstreamList, error)
	Get(context.Context, string) (*resource.Upstream, error)
	Create(context.Context, *resource.Upstream) (*resource.Upstream, error)
	Update(context.Context, *resource.Upstream) (*resource.Upstream, error)
	Delete(context.Context, string) error
}

type CertificateRepository interface {
	List(context.Context) (*resource.CertificateList, error)
	Get(context.Context, string) (*resource.Certificate, error)
	Create(context.Context, *resource.Certificate) (*resource.Certificate, error)
	Update(context.Context, *resource.Certificate) (*resource.Certificate, error)
	Delete(context.Context, string) error
}

type RateLimitPolicyRepository interface {
	List(context.Context) (*resource.RateLimitPolicyList, error)
	Get(context.Context, string) (*resource.RateLimitPolicy, error)
	Create(context.Context, *resource.RateLimitPolicy) (*resource.RateLimitPolicy, error)
	Update(context.Context, *resource.RateLimitPolicy) (*resource.RateLimitPolicy, error)
	Delete(context.Context, string) error
}

type AccessControlPolicyRepository interface {
	List(context.Context) (*resource.AccessControlPolicyList, error)
	Get(context.Context, string) (*resource.AccessControlPolicy, error)
	Create(context.Context, *resource.AccessControlPolicy) (*resource.AccessControlPolicy, error)
	Update(context.Context, *resource.AccessControlPolicy) (*resource.AccessControlPolicy, error)
	Delete(context.Context, string) error
}

type TokenQuotaPolicyRepository interface {
	List(context.Context) (*resource.TokenQuotaPolicyList, error)
	Get(context.Context, string) (*resource.TokenQuotaPolicy, error)
	Create(context.Context, *resource.TokenQuotaPolicy) (*resource.TokenQuotaPolicy, error)
	Update(context.Context, *resource.TokenQuotaPolicy) (*resource.TokenQuotaPolicy, error)
	Delete(context.Context, string) error
}

type AccessKeyRepository interface {
	Reconcile(context.Context) error
	List(context.Context) ([]AccessKey, error)
	Get(context.Context, string) (AccessKey, error)
	NameExists(context.Context, string, string) (bool, error)
	Create(context.Context, AccessKey) error
	Update(context.Context, AccessKey, AccessKey) error
	SetEnabled(context.Context, AccessKey, AccessKey) error
	Rotate(context.Context, AccessKey, AccessKey) error
	Delete(context.Context, AccessKey) error
}
