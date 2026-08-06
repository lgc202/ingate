package accesskey

import (
	"time"

	accesskeyservice "github.com/lgc202/ingate/internal/admin/service/accesskey"
)

// AccessKey 是 Admin API 返回的访问密钥，不包含可恢复的完整 Secret
type AccessKey struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Prefix        string   `json:"prefix"`
	Suffix        string   `json:"suffix"`
	Enabled       bool     `json:"enabled"`
	AllowedModels []string `json:"allowedModels"`
	ExpiresAt     *string  `json:"expiresAt,omitempty"`
	LastUsedAt    *string  `json:"lastUsedAt,omitempty"`
	CreatedAt     string   `json:"createdAt"`
}

// ListAccessKeysResp 是访问密钥列表响应
type ListAccessKeysResp struct {
	AccessKeys []AccessKey `json:"accessKeys"`
}

// CreateAccessKeyResp 是创建访问密钥响应，Secret 只在本次响应出现
type CreateAccessKeyResp struct {
	AccessKey AccessKey `json:"accessKey"`
	Secret    string    `json:"secret"`
}

// UpdateAccessKeyResp 是更新或启停访问密钥响应
type UpdateAccessKeyResp struct {
	AccessKey AccessKey `json:"accessKey"`
}

// RotateAccessKeyResp 是轮换访问密钥响应，Secret 只在本次响应出现
type RotateAccessKeyResp struct {
	AccessKey AccessKey `json:"accessKey"`
	Secret    string    `json:"secret"`
}

// DeleteAccessKeyResp 是删除访问密钥响应
type DeleteAccessKeyResp struct {
	Success bool `json:"success"`
}

// NewListAccessKeysResp 创建访问密钥列表响应
func NewListAccessKeysResp(keys []accesskeyservice.AccessKey) ListAccessKeysResp {
	result := make([]AccessKey, 0, len(keys))
	for _, key := range keys {
		result = append(result, newAccessKey(key))
	}
	return ListAccessKeysResp{AccessKeys: result}
}

// NewCreateAccessKeyResp 创建访问密钥创建响应
func NewCreateAccessKeyResp(key accesskeyservice.AccessKey, secret string) CreateAccessKeyResp {
	return CreateAccessKeyResp{AccessKey: newAccessKey(key), Secret: secret}
}

// NewUpdateAccessKeyResp 创建访问密钥更新响应
func NewUpdateAccessKeyResp(key accesskeyservice.AccessKey) UpdateAccessKeyResp {
	return UpdateAccessKeyResp{AccessKey: newAccessKey(key)}
}

// NewRotateAccessKeyResp 创建访问密钥轮换响应
func NewRotateAccessKeyResp(key accesskeyservice.AccessKey, secret string) RotateAccessKeyResp {
	return RotateAccessKeyResp{AccessKey: newAccessKey(key), Secret: secret}
}

func newAccessKey(key accesskeyservice.AccessKey) AccessKey {
	return AccessKey{
		ID:            key.ID,
		Name:          key.Name,
		Prefix:        key.Prefix,
		Suffix:        key.Suffix,
		Enabled:       key.Enabled,
		AllowedModels: key.AllowedModels,
		ExpiresAt:     formatOptionalTime(key.ExpiresAt),
		LastUsedAt:    formatOptionalTime(key.LastUsedAt),
		CreatedAt:     key.CreatedAt.Format(time.RFC3339Nano),
	}
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339Nano)
	return &formatted
}
