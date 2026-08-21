package route

import (
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

func forwarding(
	upstreams []*adminv1.RouteUpstream,
	ai *adminv1.AIRoute,
) ([]resource.UpstreamRef, *resource.AIRoute, error) {
	if ai != nil {
		if len(upstreams) != 0 {
			return nil, nil, adminservice.BadRequest("AI 路由不能同时配置普通目标服务")
		}
		aiRoute, err := aiForwarding(ai)
		return nil, aiRoute, err
	}
	if len(upstreams) == 0 {
		return nil, nil, adminservice.BadRequest("至少需要配置一个目标服务")
	}

	refs := make([]resource.UpstreamRef, 0, len(upstreams))
	seen := make(map[string]struct{}, len(upstreams))
	for _, input := range upstreams {
		if input == nil {
			return nil, nil, adminservice.BadRequest("目标服务不能为空")
		}
		id := strings.TrimSpace(input.GetUpstreamId())
		if _, exists := seen[id]; exists {
			return nil, nil, adminservice.BadRequest("目标服务不能重复")
		}
		seen[id] = struct{}{}
		refs = append(refs, resource.UpstreamRef{Name: id, Weight: int(input.GetWeight())})
	}
	return refs, nil, nil
}

func aiForwarding(input *adminv1.AIRoute) (*resource.AIRoute, error) {
	models := make([]resource.AIModel, 0, len(input.GetModels()))
	seenModels := make(map[string]struct{}, len(input.GetModels()))
	for _, modelInput := range input.GetModels() {
		if modelInput == nil {
			return nil, adminservice.BadRequest("客户端模型不能为空")
		}
		name := strings.TrimSpace(modelInput.GetName())
		if name == "" || name != modelInput.GetName() {
			return nil, adminservice.BadRequest("客户端模型名不能为空或包含首尾空格")
		}
		if _, exists := seenModels[name]; exists {
			return nil, adminservice.BadRequest("客户端模型名不能重复")
		}
		seenModels[name] = struct{}{}

		targets, err := aiModelTargets(modelInput.GetTargets())
		if err != nil {
			return nil, err
		}
		models = append(models, resource.AIModel{Name: name, Targets: targets})
	}
	return &resource.AIRoute{Models: models}, nil
}

func aiModelTargets(inputs []*adminv1.AIModelTarget) ([]resource.AIModelTarget, error) {
	targets := make([]resource.AIModelTarget, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		if input == nil {
			return nil, adminservice.BadRequest("模型线路不能为空")
		}
		upstreamID := strings.TrimSpace(input.GetUpstreamId())
		model := strings.TrimSpace(input.GetModel())
		if model == "" || model != input.GetModel() {
			return nil, adminservice.BadRequest("真实模型名不能为空或包含首尾空格")
		}
		if _, exists := seen[upstreamID]; exists {
			return nil, adminservice.BadRequest("同一个客户端模型不能重复选择模型服务")
		}
		seen[upstreamID] = struct{}{}
		targets = append(targets, resource.AIModelTarget{
			UpstreamRef: upstreamID,
			Model:       model,
			Weight:      int(input.GetWeight()),
		})
	}
	return targets, nil
}
