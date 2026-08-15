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
		if _, err := s.gateways.Get(ctx, gatewayID); err != nil {
			if errors.Is(err, biz.ErrResourceNotFound) {
				return biz.NewUserError(fmt.Sprintf("关联网关 %q 不存在", gatewayID))
			}
			return err
		}
	}

	for _, ref := range spec.UpstreamRefs {
		_, err := s.upstreams.Get(ctx, ref.Name)
		if err != nil {
			if errors.Is(err, biz.ErrResourceNotFound) {
				return biz.NewUserError(fmt.Sprintf("关联服务 %q 不存在", ref.Name))
			}
			return err
		}
	}
	return nil
}
