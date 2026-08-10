package upstream

import (
	"net/url"
	"path"
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func buildModelSpec(input *adminv1.ModelConfig) (*resource.ModelSpec, error) {
	if input == nil {
		return nil, adminservice.BadRequest("模型服务必须配置厂商和模型目录")
	}
	provider, err := modelProvider(input.GetProvider())
	if err != nil {
		return nil, err
	}
	basePath := strings.TrimSpace(input.GetBasePath())
	if !validBasePath(basePath) {
		return nil, adminservice.BadRequest("模型 API 基础路径格式不正确")
	}
	if len(input.GetModels()) == 0 {
		return nil, adminservice.BadRequest("至少需要配置一个厂商模型")
	}

	models := make([]string, 0, len(input.GetModels()))
	seen := make(map[string]struct{}, len(input.GetModels()))
	for _, inputModel := range input.GetModels() {
		model := strings.TrimSpace(inputModel)
		if model == "" {
			return nil, adminservice.BadRequest("厂商模型名称不能为空")
		}
		if _, exists := seen[model]; exists {
			return nil, adminservice.BadRequest("厂商模型名称不能重复")
		}
		seen[model] = struct{}{}
		models = append(models, model)
	}
	return &resource.ModelSpec{Provider: provider, BasePath: basePath, Models: models}, nil
}

func modelProvider(value adminv1.ModelProvider) (resource.ModelProvider, error) {
	switch value {
	case adminv1.ModelProvider_MODEL_PROVIDER_OPENAI:
		return resource.ModelProviderOpenAI, nil
	case adminv1.ModelProvider_MODEL_PROVIDER_DEEPSEEK:
		return resource.ModelProviderDeepSeek, nil
	case adminv1.ModelProvider_MODEL_PROVIDER_QWEN:
		return resource.ModelProviderQwen, nil
	case adminv1.ModelProvider_MODEL_PROVIDER_ANTHROPIC:
		return resource.ModelProviderAnthropic, nil
	case adminv1.ModelProvider_MODEL_PROVIDER_GEMINI:
		return resource.ModelProviderGemini, nil
	case adminv1.ModelProvider_MODEL_PROVIDER_CUSTOM:
		return resource.ModelProviderCustom, nil
	default:
		return "", adminservice.BadRequest("模型厂商不正确")
	}
}

func validBasePath(value string) bool {
	if value == "" || !strings.HasPrefix(value, "/") || (value != "/" && strings.HasSuffix(value, "/")) {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != value {
		return false
	}
	return path.Clean(value) == value
}
