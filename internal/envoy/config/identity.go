package config

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

// runtimeConfigID 使用长度前缀编码生成确定性运行配置摘要，避免字段拼接歧义
func runtimeConfigID(fields ...string) string {
	var data []byte
	for _, field := range fields {
		data = binary.AppendUvarint(data, uint64(len(field)))
		data = append(data, field...)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
