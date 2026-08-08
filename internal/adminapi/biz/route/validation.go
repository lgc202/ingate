package route

import (
	"context"
	"errors"
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	upstreambiz "github.com/lgc202/ingate/internal/adminapi/biz/upstream"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func (u *Usecase) validateNameUnique(ctx context.Context, name, excludeID string) error {
	routes, err := u.repository.List(ctx)
	if err != nil {
		return err
	}
	for _, route := range routes {
		if route.Name == excludeID {
			continue
		}
		if route.Spec.DisplayName == name {
			return biz.NewUserError(fmt.Sprintf("路由名称 %q 已存在", name))
		}
	}
	return nil
}

func (u *Usecase) validateReferences(ctx context.Context, spec resource.RouteSpec) error {
	for _, parentRef := range spec.ParentRefs {
		if _, err := u.gateways.Get(ctx, parentRef.Name); err != nil {
			if errors.Is(err, biz.ErrResourceNotFound) {
				return biz.NewUserError(fmt.Sprintf("关联网关 %q 不存在", parentRef.Name))
			}
			return err
		}
	}

	upstreams := make(map[string]*resource.Upstream)
	for _, rule := range spec.Rules {
		for _, ref := range rule.UpstreamRefs {
			upstream, err := u.getUpstream(ctx, upstreams, ref.Name)
			if err != nil {
				if errors.Is(err, biz.ErrResourceNotFound) {
					return biz.NewUserError(fmt.Sprintf("关联服务 %q 不存在", ref.Name))
				}
				return err
			}
			if upstream.Spec.Type == resource.UpstreamTypeModel {
				return biz.NewUserError(fmt.Sprintf("模型服务 %q 只能用于模型路由", upstreamDisplayName(upstream)))
			}
			if upstream.Spec.Protocol != resource.UpstreamProtocolHTTP {
				return biz.NewUserError(fmt.Sprintf("服务 %q 的协议不能用于普通 HTTP 路由", upstreamDisplayName(upstream)))
			}
		}
		if rule.ModelRouting == nil {
			continue
		}
		for _, model := range rule.ModelRouting.Models {
			upstream, err := u.getUpstream(ctx, upstreams, model.UpstreamRef)
			if err != nil {
				if errors.Is(err, biz.ErrResourceNotFound) {
					return biz.NewUserError(fmt.Sprintf("关联模型服务 %q 不存在", model.UpstreamRef))
				}
				return err
			}
			if !upstreambiz.ValidModel(upstream) {
				return biz.NewUserError(fmt.Sprintf("关联服务 %q 不是有效的大模型服务", upstreamDisplayName(upstream)))
			}
			upstreamModel := model.UpstreamModel
			if upstreamModel == "" {
				upstreamModel = model.Model
			}
			if !upstreambiz.ModelEnabled(upstream.Spec.Model, upstreamModel) {
				return biz.NewUserError(fmt.Sprintf("模型服务 %q 未启用厂商模型 %q", upstreamDisplayName(upstream), upstreamModel))
			}
		}
	}
	return nil
}

func (u *Usecase) getUpstream(
	ctx context.Context,
	upstreams map[string]*resource.Upstream,
	upstreamID string,
) (*resource.Upstream, error) {
	if upstream, ok := upstreams[upstreamID]; ok {
		return upstream, nil
	}
	upstream, err := u.upstreams.Get(ctx, upstreamID)
	if err != nil {
		return nil, err
	}
	upstreams[upstreamID] = upstream
	return upstream, nil
}

func upstreamDisplayName(upstream *resource.Upstream) string {
	if upstream.Spec.DisplayName != "" {
		return upstream.Spec.DisplayName
	}
	return upstream.Name
}
