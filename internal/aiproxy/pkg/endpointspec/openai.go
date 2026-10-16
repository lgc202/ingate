package endpointspec

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	sigsjson "sigs.k8s.io/json"
)

// parseOpenAIRequest 保留未知字段，仅统一两个创建操作的 JSON 对象与 model 边界。
func parseOpenAIRequest(body []byte) (map[string]json.RawMessage, string, error) {
	if !utf8.Valid(body) {
		return nil, "", errors.New("model request must be valid UTF-8")
	}

	var fields map[string]json.RawMessage
	strictErrors, err := sigsjson.UnmarshalStrict(body, &fields, sigsjson.DisallowDuplicateFields)
	if err != nil {
		return nil, "", fmt.Errorf("decode model request: %w", err)
	}

	if len(strictErrors) != 0 {
		return nil, "", fmt.Errorf("decode model request fields: %w", errors.Join(strictErrors...))
	}

	if fields == nil {
		return nil, "", errors.New("model request must be a JSON object")
	}

	model, exists := fields["model"]
	if !exists {
		return nil, "", errors.New("model request model is required")
	}

	var modelName string
	if err := json.Unmarshal(model, &modelName); err != nil {
		return nil, "", fmt.Errorf("decode request model: %w", err)
	}

	if modelName == "" {
		return nil, "", errors.New("request model must be a nonempty string")
	}

	return fields, modelName, nil
}
