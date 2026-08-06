// Package accesskey 定义管理面和 AI 数据面共享的访问密钥执行协议
package accesskey

import (
	"crypto/sha256"
	"encoding/hex"
)

const (
	CredentialSetKey = "ingate:access-key:credentials"
	LastUsedKey      = "ingate:access-key:last-used"

	FieldID            = "id"
	FieldAllowedModels = "allowed_models"
	FieldExpiresAt     = "expires_at"

	credentialKeyPrefix = "ingate:access-key:credential:"
)

// Hash 计算高熵访问密钥的数据面索引，原始 Secret 不进入 MySQL 或 Redis
func Hash(secret string) [sha256.Size]byte {
	return sha256.Sum256([]byte(secret))
}

// CredentialKey 返回访问密钥哈希对应的 Redis 记录键
func CredentialKey(hash [sha256.Size]byte) string {
	return credentialKeyPrefix + hex.EncodeToString(hash[:])
}
