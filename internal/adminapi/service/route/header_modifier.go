package route

import (
	"strings"

	"golang.org/x/net/http/httpguts"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func buildHeaderModifier(input *adminv1.HeaderModifier) (*resource.HeaderModifier, error) {
	modifier := &resource.HeaderModifier{}
	seen := make(map[string]struct{}, len(input.GetSet())+len(input.GetAdd())+len(input.GetRemove()))
	set, err := buildHeaderValues(input.GetSet(), seen)
	if err != nil {
		return nil, err
	}
	modifier.Set = set
	add, err := buildHeaderValues(input.GetAdd(), seen)
	if err != nil {
		return nil, err
	}
	modifier.Add = add
	for _, inputName := range input.GetRemove() {
		name := strings.ToLower(strings.TrimSpace(inputName))
		if !httpguts.ValidHeaderFieldName(name) {
			return nil, adminservice.BadRequest("待删除的 Header 名称格式不正确")
		}
		if _, exists := seen[name]; exists {
			return nil, adminservice.BadRequest("同一个 Header 只能配置一种修改动作")
		}
		seen[name] = struct{}{}
		modifier.Remove = append(modifier.Remove, name)
	}
	if len(modifier.Set) == 0 && len(modifier.Add) == 0 && len(modifier.Remove) == 0 {
		return nil, adminservice.BadRequest("至少需要配置一个 Header 修改动作")
	}
	return modifier, nil
}

func buildHeaderValues(inputs []*adminv1.HeaderValue, seen map[string]struct{}) ([]resource.HeaderValue, error) {
	values := make([]resource.HeaderValue, 0, len(inputs))
	for _, input := range inputs {
		if input == nil {
			return nil, adminservice.BadRequest("Header 名称和值不能为空")
		}
		name := strings.ToLower(strings.TrimSpace(input.GetName()))
		value := input.GetValue()
		if !httpguts.ValidHeaderFieldName(name) || value == "" || !httpguts.ValidHeaderFieldValue(value) {
			return nil, adminservice.BadRequest("Header 名称或值格式不正确")
		}
		if _, exists := seen[name]; exists {
			return nil, adminservice.BadRequest("同一个 Header 只能配置一种修改动作")
		}
		seen[name] = struct{}{}
		values = append(values, resource.HeaderValue{Name: name, Value: value})
	}
	return values, nil
}

func containsManagedHeader(modifier *resource.HeaderModifier) bool {
	if modifier == nil {
		return false
	}
	for _, header := range modifier.Set {
		if isAIManagedRequestHeader(header.Name) {
			return true
		}
	}
	for _, header := range modifier.Add {
		if isAIManagedRequestHeader(header.Name) {
			return true
		}
	}
	for _, name := range modifier.Remove {
		if isAIManagedRequestHeader(name) {
			return true
		}
	}
	return false
}

func isAIManagedRequestHeader(name string) bool {
	switch name {
	case ":authority", ":path", "accept-encoding", "anthropic-version", "authorization",
		"content-encoding", "content-length", "content-type", aiClusterHeader, "x-api-key", "x-goog-api-key":
		return true
	default:
		return false
	}
}
