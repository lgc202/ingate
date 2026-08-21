package route

import (
	"context"
	"errors"
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func (s *Service) validateReferences(ctx context.Context, spec resource.RouteSpec) error {
	// 引用预检只改善控制台的保存反馈，资源发布结果仍由 Controller status 表达
	for _, gatewayID := range spec.GatewayRefs {
		_, err := s.gateways.Get(ctx, gatewayID)
		if err != nil {
			if errors.Is(err, biz.ErrResourceNotFound) {
				return biz.NewRuleViolation(fmt.Sprintf("关联网关 %q 不存在", gatewayID))
			}
			return err
		}
	}

	for _, ref := range spec.UpstreamRefs {
		upstream, err := s.upstreams.Get(ctx, ref.Name)
		if err != nil {
			if errors.Is(err, biz.ErrResourceNotFound) {
				return biz.NewRuleViolation(fmt.Sprintf("关联服务 %q 不存在", ref.Name))
			}
			return err
		}
		if upstream.Spec.Model != nil {
			return biz.NewRuleViolation(fmt.Sprintf("模型服务 %q 只能用于 AI 路由", upstream.Spec.DisplayName))
		}
	}

	if spec.AI == nil {
		return nil
	}
	for _, model := range spec.AI.Models {
		for _, target := range model.Targets {
			upstream, err := s.upstreams.Get(ctx, target.UpstreamRef)
			if err != nil {
				if errors.Is(err, biz.ErrResourceNotFound) {
					return biz.NewRuleViolation(fmt.Sprintf("关联模型服务 %q 不存在", target.UpstreamRef))
				}
				return err
			}
			if upstream.Spec.Model == nil {
				return biz.NewRuleViolation(fmt.Sprintf("服务 %q 不是模型服务", upstream.Spec.DisplayName))
			}
		}
	}
	return nil
}
