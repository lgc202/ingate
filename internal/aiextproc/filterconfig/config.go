// Package filterconfig 定义 Controller、Envoy、AI ExtProc 与 ALS 之间的内部执行协议
package filterconfig

import (
	"errors"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/structpb"
)

const (
	// MetadataNamespace 隔离 Ingate 写入 Envoy 的 AI 执行元数据
	MetadataNamespace = "ingate.ai"
	// ClientHostField 保存进入网关时的请求 Host，避免上游改写覆盖客户端请求信息
	ClientHostField = "client_host"
	// ClientPathField 保存进入网关时不含查询参数的请求路径
	ClientPathField = "client_path"
	// AttributeNamespace 是 Envoy 向上游 ExtProc 传递 xDS 属性的固定命名空间
	AttributeNamespace = "envoy.filters.http.ext_proc"

	// ModelHeader 把请求体中的客户端模型转换为 Envoy 可以匹配的内部 Header
	ModelHeader = "x-ingate-ai-model"
	// UpstreamModelHeader 由 Envoy 在选中模型线路后写入实际模型名
	UpstreamModelHeader = "x-ingate-ai-upstream-model"
	// RequestIDHeader 关联同一请求的入口 ExtProc 流和上游 ExtProc 流
	RequestIDHeader = "x-ingate-ai-request-id"

	// ServiceIDAttribute 读取 Envoy 最终选择的模型 Service ID
	ServiceIDAttribute = "xds.upstream_host_metadata.filter_metadata['ingate.ai']['service_id']"
	// ProtocolAttribute 读取模型 Service 使用的厂商协议
	ProtocolAttribute = "xds.upstream_host_metadata.filter_metadata['ingate.ai']['protocol']"
)

// Protocol 表示模型 Service 实际使用的 HTTP API 协议
type Protocol string

const (
	// ProtocolOpenAI 保持 OpenAI Chat Completions 请求与响应格式
	ProtocolOpenAI Protocol = "openai"
	// ProtocolAnthropic 把 OpenAI Chat Completions 转换为 Anthropic Messages
	ProtocolAnthropic Protocol = "anthropic"
)

// SelectedService 是 Envoy 完成负载均衡后交给 AI ExtProc 的非敏感执行信息
// 凭据不进入 xDS 属性，后续由 AI ExtProc 根据 Service ID 从本地配置中读取
type SelectedService struct {
	ID       string
	Protocol Protocol
	Model    string
}

// FromAttributes 从上游 ExtProc 属性和内部 Header 中读取 Envoy 最终选择的模型线路
// Service 协议属于 xDS 端点元数据，真实模型属于 Route 的加权线路，因此来自不同位置
func FromAttributes(attributes map[string]*structpb.Struct, model string) (SelectedService, error) {
	// Envoy 按固定命名空间聚合 attributes，内部字段名仍保留完整 xDS 表达式
	values := attributes[AttributeNamespace]
	if values == nil {
		return SelectedService{}, errors.New("AI upstream attributes are missing")
	}

	selected := SelectedService{
		ID:       attributeString(values, ServiceIDAttribute),
		Protocol: Protocol(attributeString(values, ProtocolAttribute)),
		Model:    model,
	}
	if err := selected.Validate(); err != nil {
		return SelectedService{}, fmt.Errorf("validate selected model service: %w", err)
	}
	return selected, nil
}

// Validate 校验选中线路是否包含协议转换所需的最小信息
func (s SelectedService) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return errors.New("service ID must not be empty")
	}
	if strings.TrimSpace(s.Model) == "" {
		return errors.New("upstream model must not be empty")
	}
	switch s.Protocol {
	case ProtocolOpenAI, ProtocolAnthropic:
		return nil
	default:
		return errors.New("upstream protocol is not supported")
	}
}

func attributeString(attributes *structpb.Struct, path string) string {
	// 缺失字段由 protobuf getter 返回零值，统一交给 SelectedService.Validate 报错
	return attributes.GetFields()[path].GetStringValue()
}
