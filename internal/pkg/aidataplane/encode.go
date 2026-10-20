package aidataplane

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Encode 校验并编码配置，重新计算 Revision，不修改调用方的值或集合顺序。
// 调用方须先完成领域解析、默认值和排序，所有数组必须非 nil。
// 本函数用于中间表示的离线校验与测试夹具，不参与 Delta ADS 分发或逐资源版本计算。
// 不验证传入的旧 Revision，也不解码任意 JSON。
func Encode(c Config) ([]byte, error) {
	if err := Validate(c); err != nil {
		return nil, fmt.Errorf("validate AI config: %w", err)
	}

	revision, err := contentRevision(c)
	if err != nil {
		return nil, fmt.Errorf("compute AI config revision: %w", err)
	}

	c.Revision = revision

	data, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("encode AI config: %w", err)
	}

	return data, nil
}

// contentRevision 对固定顺序的强类型内容求哈希，不包含 revision 自身。
func contentRevision(c Config) (string, error) {
	c.Revision = ""
	content, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("encode AI config content: %w", err)
	}

	hash := sha256.Sum256(content)

	return hex.EncodeToString(hash[:]), nil
}
