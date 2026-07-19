package compiler

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

// configFingerprint 使用长度前缀编码生成确定性配置指纹，避免字段拼接歧义
func configFingerprint(fields ...string) string {
	var data []byte
	for _, field := range fields {
		data = binary.AppendUvarint(data, uint64(len(field)))
		data = append(data, field...)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
