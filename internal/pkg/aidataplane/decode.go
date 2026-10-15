package aidataplane

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	sigsjson "sigs.k8s.io/json"
)

// Decode 严格解码一份完整配置；校验失败时不返回部分结果。
// 输入字段区分大小写，未知、重复、缺失或 null 字段均拒绝。
// Revision 按强类型内容重新编码核对，输入的缩进和对象成员顺序不参与哈希。
// 本函数不加载文件、不交换运行视图，内容哈希也不提供身份认证。
func Decode(data []byte) (*Config, error) {
	if !utf8.Valid(data) {
		return nil, errors.New("AI config must be valid UTF-8")
	}

	var c Config
	strictErrors, err := sigsjson.UnmarshalStrict(
		data,
		&c,
		sigsjson.DisallowDuplicateFields,
		sigsjson.DisallowUnknownFields,
	)
	if err != nil {
		return nil, fmt.Errorf("decode AI config: %w", err)
	}
	if len(strictErrors) != 0 {
		return nil, fmt.Errorf("decode AI config fields: %w", errors.Join(strictErrors...))
	}

	if err := Validate(c); err != nil {
		return nil, fmt.Errorf("validate AI config: %w", err)
	}

	if len(c.Revision) != sha256.Size*2 || c.Revision != strings.ToLower(c.Revision) {
		return nil, errors.New("revision must be 64 lowercase hexadecimal characters")
	}
	if _, err := hex.DecodeString(c.Revision); err != nil {
		return nil, fmt.Errorf("decode revision: %w", err)
	}

	revision, err := contentRevision(c)
	if err != nil {
		return nil, fmt.Errorf("compute AI config revision: %w", err)
	}
	if revision != c.Revision {
		return nil, errors.New("AI config revision does not match content")
	}

	return &c, nil
}
