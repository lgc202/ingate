package route

import (
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func buildForwarding(spec *resource.RouteSpec, upstreams []*adminv1.RouteUpstream, modelRouting *adminv1.ModelRouting) error {
	if len(upstreams) > 0 && modelRouting != nil {
		return adminservice.BadRequest("普通服务转发和模型选路不能同时配置")
	}
	if len(upstreams) == 0 && modelRouting == nil {
		return adminservice.BadRequest("必须配置普通服务转发或模型选路")
	}
	if modelRouting != nil {
		models, err := buildModelMappings(modelRouting.GetModels())
		if err != nil {
			return err
		}
		spec.ModelRouting = &resource.ModelRouting{Models: models}
		return nil
	}

	seen := make(map[string]struct{}, len(upstreams))
	for _, input := range upstreams {
		if input == nil {
			return adminservice.BadRequest("目标服务不能为空")
		}
		id := strings.TrimSpace(input.GetUpstreamId())
		if id == "" || input.GetWeight() < 1 || input.GetWeight() > 1000 {
			return adminservice.BadRequest("目标服务 ID 或权重不正确")
		}
		if _, exists := seen[id]; exists {
			return adminservice.BadRequest("目标服务不能重复")
		}
		seen[id] = struct{}{}
		spec.UpstreamRefs = append(spec.UpstreamRefs, resource.UpstreamRef{Name: id, Weight: int(input.GetWeight())})
	}
	return nil
}

func buildModelMappings(inputs []*adminv1.ModelMapping) ([]resource.ModelMapping, error) {
	if len(inputs) == 0 {
		return nil, adminservice.BadRequest("至少需要配置一个公开模型")
	}
	mappings := make([]resource.ModelMapping, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		if input == nil {
			return nil, adminservice.BadRequest("模型映射不能为空")
		}
		model := strings.TrimSpace(input.GetModel())
		upstreamID := strings.TrimSpace(input.GetUpstreamId())
		if model == "" || upstreamID == "" {
			return nil, adminservice.BadRequest("公开模型名称和模型服务不能为空")
		}
		if _, exists := seen[model]; exists {
			return nil, adminservice.BadRequest("公开模型名称不能重复")
		}
		seen[model] = struct{}{}
		mappings = append(mappings, resource.ModelMapping{
			Model:         model,
			UpstreamRef:   upstreamID,
			UpstreamModel: strings.TrimSpace(input.GetUpstreamModel()),
		})
	}
	return mappings, nil
}
