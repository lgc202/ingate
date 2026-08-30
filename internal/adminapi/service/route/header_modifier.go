package route

import (
	"slices"

	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

func parseHeaderModifier(modifier *adminv1.HeaderModifier) (*resource.HeaderModifier, error) {
	if modifier == nil {
		return nil, nil
	}
	actionCount := len(modifier.GetSet()) + len(modifier.GetAdd()) + len(modifier.GetRemove())
	if actionCount == 0 {
		return nil, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"至少需要配置一个 Header 修改动作",
		)
	}
	if actionCount > routeconfig.MaxHeaderModifierActions {
		return nil, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"Header 修改动作数量超过限制",
		)
	}

	usedNames := make(map[string]bool, actionCount)
	set, err := parseHeaderValues(modifier.GetSet(), usedNames)
	if err != nil {
		return nil, err
	}
	add, err := parseHeaderValues(modifier.GetAdd(), usedNames)
	if err != nil {
		return nil, err
	}
	remove := make([]string, len(modifier.GetRemove()))
	for i, headerName := range modifier.GetRemove() {
		name := httpheader.NormalizeName(headerName)
		if !httpheader.IsValidName(name) {
			return nil, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"待删除的 Header 名称格式不正确",
			)
		}
		if usedNames[name] {
			return nil, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"同一个 Header 只能配置一种修改动作",
			)
		}
		usedNames[name] = true
		remove[i] = name
	}
	return &resource.HeaderModifier{Set: set, Add: add, Remove: remove}, nil
}

func parseHeaderValues(
	values []*adminv1.HeaderValue,
	usedNames map[string]bool,
) ([]resource.HeaderValue, error) {
	headers := make([]resource.HeaderValue, len(values))
	for i, header := range values {
		if header == nil {
			return nil, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"Header 名称和值不能为空",
			)
		}
		name := httpheader.NormalizeName(header.GetName())
		value := httpheader.NormalizeValue(header.GetValue())
		if !httpheader.IsValidName(name) || value == "" || !httpheader.IsValidValue(value) {
			return nil, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"Header 名称或值格式不正确",
			)
		}
		if usedNames[name] {
			return nil, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"同一个 Header 只能配置一种修改动作",
			)
		}
		usedNames[name] = true
		headers[i] = resource.HeaderValue{Name: name, Value: value}
	}
	return headers, nil
}

func headerModifierResponse(modifier *resource.HeaderModifier) *adminv1.HeaderModifier {
	if modifier == nil {
		return nil
	}
	response := &adminv1.HeaderModifier{
		Set:    make([]*adminv1.HeaderValue, len(modifier.Set)),
		Add:    make([]*adminv1.HeaderValue, len(modifier.Add)),
		Remove: slices.Clone(modifier.Remove),
	}
	for i, header := range modifier.Set {
		response.Set[i] = &adminv1.HeaderValue{
			Name:  header.Name,
			Value: header.Value,
		}
	}
	for i, header := range modifier.Add {
		response.Add[i] = &adminv1.HeaderValue{
			Name:  header.Name,
			Value: header.Value,
		}
	}
	return response
}
