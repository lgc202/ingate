package route

import (
	"strings"

	"github.com/samber/lo"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

func parseForwarding(forwarding *adminv1.RouteForwarding) ([]resource.UpstreamRef, *resource.AIRoute, error) {
	if forwarding == nil {
		return nil, nil, adminv1.ErrorInvalidArgument("请选择路由转发方式")
	}

	switch kind := forwarding.GetKind().(type) {
	case *adminv1.RouteForwarding_Service:
		serviceRefs, err := parseServiceTargets(kind.Service.GetTargets())
		if err != nil {
			return nil, nil, err
		}
		return serviceRefs, nil, nil
	case *adminv1.RouteForwarding_Ai:
		aiRoute, err := parseAIRoute(kind.Ai)
		if err != nil {
			return nil, nil, err
		}
		return nil, aiRoute, nil
	default:
		return nil, nil, adminv1.ErrorInvalidArgument("请选择路由转发方式")
	}
}

func parseServiceTargets(targets []*adminv1.ServiceTarget) ([]resource.UpstreamRef, error) {
	if len(targets) == 0 {
		return nil, adminv1.ErrorInvalidArgument("至少需要配置一个目标服务")
	}
	if len(targets) > routeconfig.MaxServiceTargets {
		return nil, adminv1.ErrorInvalidArgument("目标服务数量超过限制")
	}

	serviceRefs := make([]resource.UpstreamRef, len(targets))
	seenServiceIDs := make(map[string]bool, len(targets))
	for i, target := range targets {
		if target == nil {
			return nil, adminv1.ErrorInvalidArgument("目标服务不能为空")
		}
		serviceID, valid := resourceconfig.NormalizeID(target.GetServiceId())
		if !valid {
			return nil, adminv1.ErrorInvalidArgument("目标服务 ID 不正确")
		}
		if seenServiceIDs[serviceID] {
			return nil, adminv1.ErrorInvalidArgument("目标服务不能重复")
		}
		weight := int(target.GetWeight())
		if weight < routeconfig.MinTargetWeight || weight > routeconfig.MaxTargetWeight {
			return nil, adminv1.ErrorInvalidArgument("目标服务权重超出允许范围")
		}
		seenServiceIDs[serviceID] = true
		serviceRefs[i] = resource.UpstreamRef{Name: serviceID, Weight: weight}
	}
	return serviceRefs, nil
}

func parseAIRoute(aiRoute *adminv1.AIRoute) (*resource.AIRoute, error) {
	if aiRoute == nil || len(aiRoute.GetModels()) == 0 {
		return nil, adminv1.ErrorInvalidArgument("至少需要配置一个客户端模型")
	}
	modelInputs := aiRoute.GetModels()
	if len(modelInputs) > routeconfig.MaxAIModels {
		return nil, adminv1.ErrorInvalidArgument("客户端模型数量超过限制")
	}

	models := make([]resource.AIModel, len(modelInputs))
	seenModelNames := make(map[string]bool, len(modelInputs))
	for i, modelInput := range modelInputs {
		if modelInput == nil {
			return nil, adminv1.ErrorInvalidArgument("客户端模型不能为空")
		}
		modelName := strings.TrimSpace(modelInput.GetName())
		if !routeconfig.IsValidModelName(modelName) || modelName != modelInput.GetName() {
			return nil, adminv1.ErrorInvalidArgument("客户端模型名格式不正确")
		}
		if seenModelNames[modelName] {
			return nil, adminv1.ErrorInvalidArgument("客户端模型名不能重复")
		}
		seenModelNames[modelName] = true

		targets, err := parseAIModelTargets(modelInput.GetTargets())
		if err != nil {
			return nil, err
		}
		models[i] = resource.AIModel{Name: modelName, Targets: targets}
	}
	return &resource.AIRoute{Models: models}, nil
}

func parseAIModelTargets(targets []*adminv1.AIModelTarget) ([]resource.AIModelTarget, error) {
	if len(targets) == 0 {
		return nil, adminv1.ErrorInvalidArgument("至少需要配置一条模型线路")
	}
	if len(targets) > routeconfig.MaxAIModelTargets {
		return nil, adminv1.ErrorInvalidArgument("模型线路数量超过限制")
	}

	modelTargets := make([]resource.AIModelTarget, len(targets))
	seenServiceIDs := make(map[string]bool, len(targets))
	for i, target := range targets {
		if target == nil {
			return nil, adminv1.ErrorInvalidArgument("模型线路不能为空")
		}
		serviceID, valid := resourceconfig.NormalizeID(target.GetServiceId())
		if !valid {
			return nil, adminv1.ErrorInvalidArgument("模型服务 ID 不正确")
		}
		actualModel := strings.TrimSpace(target.GetModel())
		if !routeconfig.IsValidModelName(actualModel) || actualModel != target.GetModel() {
			return nil, adminv1.ErrorInvalidArgument("真实模型名格式不正确")
		}
		if seenServiceIDs[serviceID] {
			return nil, adminv1.ErrorInvalidArgument("同一个客户端模型不能重复选择模型服务")
		}
		weight := int(target.GetWeight())
		if weight < routeconfig.MinTargetWeight || weight > routeconfig.MaxTargetWeight {
			return nil, adminv1.ErrorInvalidArgument("模型线路权重超出允许范围")
		}
		seenServiceIDs[serviceID] = true
		modelTargets[i] = resource.AIModelTarget{
			UpstreamRef: serviceID,
			Model:       actualModel,
			Weight:      weight,
		}
	}
	return modelTargets, nil
}

func forwardingResponse(spec resource.RouteSpec) *adminv1.RouteForwarding {
	if spec.AI != nil {
		return &adminv1.RouteForwarding{
			Kind: &adminv1.RouteForwarding_Ai{Ai: aiRouteResponse(spec.AI)},
		}
	}

	targets := lo.Map(spec.UpstreamRefs, func(serviceRef resource.UpstreamRef, _ int) *adminv1.ServiceTarget {
		return &adminv1.ServiceTarget{
			ServiceId: serviceRef.Name,
			Weight:    uint32(serviceRef.Weight),
		}
	})
	return &adminv1.RouteForwarding{
		Kind: &adminv1.RouteForwarding_Service{
			Service: &adminv1.ServiceForwarding{Targets: targets},
		},
	}
}

func aiRouteResponse(aiRoute *resource.AIRoute) *adminv1.AIRoute {
	models := lo.Map(aiRoute.Models, func(model resource.AIModel, _ int) *adminv1.AIModel {
		return &adminv1.AIModel{
			Name: model.Name,
			Targets: lo.Map(model.Targets, func(target resource.AIModelTarget, _ int) *adminv1.AIModelTarget {
				return &adminv1.AIModelTarget{
					ServiceId: target.UpstreamRef,
					Model:     target.Model,
					Weight:    uint32(target.Weight),
				}
			}),
		}
	})
	return &adminv1.AIRoute{Models: models}
}
