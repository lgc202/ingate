// Package accesskey 定义 Ingate 调用方访问密钥的生成和校验格式。
package accesskey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	prefix        = "igk_"
	secretBytes   = 32
	separator     = "."
	digestHexSize = sha256.Size * 2
)

var errInvalidAccessKey = errors.New("invalid Ingate access key")

// Generate 创建包含公开 key ID 和高熵随机 secret 的完整访问密钥。
func Generate(keyID string) (string, error) {
	if !isCanonicalKeyID(keyID) {
		return "", errInvalidAccessKey
	}
	secret := make([]byte, secretBytes)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("read random secret: %w", err)
	}
	return prefix + keyID + separator + base64.RawURLEncoding.EncodeToString(secret), nil
}

// ParseKeyID 从格式正确的完整访问密钥中提取公开标识。
func ParseKeyID(value string) (string, error) {
	remainder, ok := strings.CutPrefix(value, prefix)
	if !ok {
		return "", errInvalidAccessKey
	}
	keyID, encodedSecret, ok := strings.Cut(remainder, separator)
	if !ok || !isCanonicalKeyID(keyID) || encodedSecret == "" {
		return "", errInvalidAccessKey
	}
	secret, err := base64.RawURLEncoding.DecodeString(encodedSecret)
	if err != nil || len(secret) != secretBytes {
		return "", errInvalidAccessKey
	}
	return keyID, nil
}

// Digest 计算持久化和常量时间比较使用的 SHA-256 十六进制摘要。
func Digest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

// IsValidDigest 判断字符串是否为访问密钥使用的 SHA-256 摘要。
func IsValidDigest(value string) bool {
	if len(value) != digestHexSize || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func isCanonicalKeyID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id.String() == value
}
