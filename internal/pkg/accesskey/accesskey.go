// Package accesskey 定义 Ingate 调用方访问密钥的生成和校验格式
package accesskey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

const (
	prefix        = "igk_"
	secretBytes   = 32
	separator     = "."
	digestHexSize = sha256.Size * 2
)

var errInvalid = errors.New("invalid Ingate access key")

// Generate 创建包含公开 key ID 和高熵随机 secret 的完整访问密钥
func Generate(keyID string) (string, error) {
	random := make([]byte, secretBytes)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return prefix + keyID + separator + base64.RawURLEncoding.EncodeToString(random), nil
}

// KeyID 从格式正确的完整访问密钥中提取公开标识
func KeyID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, prefix) {
		return "", errInvalid
	}
	id, encodedSecret, ok := strings.Cut(strings.TrimPrefix(value, prefix), separator)
	if !ok || id == "" || encodedSecret == "" {
		return "", errInvalid
	}
	secret, err := base64.RawURLEncoding.DecodeString(encodedSecret)
	if err != nil || len(secret) != secretBytes {
		return "", errInvalid
	}
	return id, nil
}

// Digest 计算持久化和常量时间比较使用的 SHA-256 十六进制摘要
func Digest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

// ValidDigest 判断字符串是否为访问密钥使用的 SHA-256 摘要
func ValidDigest(value string) bool {
	if len(value) != digestHexSize {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
