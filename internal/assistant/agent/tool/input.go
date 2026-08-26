package tool

import (
	"errors"
	"fmt"
	"strings"
)

type invalidInputError struct {
	reason string
}

func (e *invalidInputError) Error() string {
	return e.reason
}

func invalidInputf(format string, args ...any) error {
	return &invalidInputError{reason: fmt.Sprintf(format, args...)}
}

// InvalidInputReason 识别模型生成的参数错误，并返回可供模型修正调用的原因。
func InvalidInputReason(err error) (string, bool) {
	var inputErr *invalidInputError
	if !errors.As(err, &inputErr) {
		return "", false
	}
	return inputErr.Error(), true
}

func normalizeResourceScope(resourceType, resourceID string) (string, string, error) {
	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	resourceID = strings.TrimSpace(resourceID)
	if resourceType == "" {
		resourceType = "all"
	}
	if resourceType == "all" {
		if resourceID != "" {
			return "", "", invalidInputf("resource_id must be omitted when resource_type is all")
		}
		return resourceType, resourceID, nil
	}
	if resourceType != "gateway" && resourceType != "route" && resourceType != "service" {
		return "", "", invalidInputf(
			"unsupported resource_type %q; use all, gateway, route, or service",
			resourceType,
		)
	}
	if resourceID == "" {
		return "", "", invalidInputf(
			"resource_id is required when resource_type is %s; call list_%ss first and retry with the returned id",
			resourceType,
			resourceType,
		)
	}
	return resourceType, resourceID, nil
}
