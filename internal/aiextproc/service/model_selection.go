package service

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"

	aiprotocol "github.com/lgc202/ingate/internal/pkg/aiextproc"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

// selectedModelService 是 Envoy 完成负载均衡后交给 upstream ExtProc 的非敏感线路信息
// 凭据不进入 xDS 属性，后续根据 Service ID 从本地配置中读取
type selectedModelService struct {
	id       string
	protocol aiprotocol.UpstreamProtocol
	model    string
}

func selectedModelServiceFromAttributes(
	attributes map[string]*structpb.Struct,
	model string,
) (selectedModelService, error) {
	// Envoy 按固定命名空间聚合 attributes，内部字段名仍保留完整 xDS 表达式
	values := attributes[aiprotocol.AttributeNamespace]
	if values == nil {
		return selectedModelService{}, errors.New("AI upstream attributes are missing")
	}

	selected := selectedModelService{
		id:       attributeString(values, aiprotocol.ServiceIDAttribute),
		protocol: aiprotocol.UpstreamProtocol(attributeString(values, aiprotocol.ServiceProtocolAttribute)),
		model:    model,
	}
	if err := selected.validate(); err != nil {
		return selectedModelService{}, fmt.Errorf("validate selected model service: %w", err)
	}
	return selected, nil
}

func (s selectedModelService) validate() error {
	if !resourceconfig.IsCanonicalID(s.id) {
		return errors.New("service ID must be a canonical UUID")
	}
	if !routeconfig.IsValidModelName(s.model) {
		return errors.New("upstream model is invalid")
	}
	switch s.protocol {
	case aiprotocol.UpstreamProtocolOpenAI, aiprotocol.UpstreamProtocolAnthropic:
		return nil
	default:
		return errors.New("upstream protocol is not supported")
	}
}

func attributeString(attributes *structpb.Struct, path string) string {
	// 缺失字段由 protobuf getter 返回零值，统一交给 selectedModelService.validate 报错
	return attributes.GetFields()[path].GetStringValue()
}
