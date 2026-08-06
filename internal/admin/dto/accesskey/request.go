// Package accesskey 定义访问密钥管理接口的请求和响应模型
package accesskey

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

const maxAccessKeyNameLength = 128

// IDReq 是访问密钥路径参数
type IDReq struct {
	ID string `uri:"id"`
}

// CreateAccessKeyReq 是创建访问密钥的请求
type CreateAccessKeyReq struct {
	AccessKeyConfig
}

// UpdateAccessKeyReq 是更新访问密钥的请求
type UpdateAccessKeyReq struct {
	AccessKeyConfig
}

// AccessKeyConfig 是访问密钥可编辑配置
type AccessKeyConfig struct {
	Name          string     `json:"name"`
	AllowedModels []string   `json:"allowedModels"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
}

// SetEnabledReq 是启停访问密钥的请求
type SetEnabledReq struct {
	Enabled *bool `json:"enabled"`
}

// Validate 校验访问密钥 ID
func (r *IDReq) Validate() error {
	r.ID = strings.TrimSpace(r.ID)
	if r.ID == "" {
		return errors.New("访问密钥 ID 不能为空")
	}
	return nil
}

// Validate 校验创建访问密钥请求
func (r *CreateAccessKeyReq) Validate() error {
	if err := r.AccessKeyConfig.validate(); err != nil {
		return err
	}
	if r.ExpiresAt != nil && !r.ExpiresAt.After(time.Now()) {
		return errors.New("访问密钥有效期必须晚于当前时间")
	}
	return nil
}

// Validate 校验更新访问密钥请求
func (r *UpdateAccessKeyReq) Validate() error {
	return r.AccessKeyConfig.validate()
}

// Validate 校验启停访问密钥请求
func (r *SetEnabledReq) Validate() error {
	if r.Enabled == nil {
		return errors.New("启用状态不能为空")
	}
	return nil
}

// Value 返回已校验的启停值
func (r *SetEnabledReq) Value() bool {
	return *r.Enabled
}

func (c *AccessKeyConfig) validate() error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return errors.New("访问密钥名称不能为空")
	}
	if utf8.RuneCountInString(c.Name) > maxAccessKeyNameLength {
		return errors.New("访问密钥名称不能超过 128 个字符")
	}

	models := make([]string, 0, len(c.AllowedModels))
	seen := make(map[string]struct{}, len(c.AllowedModels))
	for _, model := range c.AllowedModels {
		model = strings.TrimSpace(model)
		if model == "" {
			return errors.New("允许访问的模型名称不能为空")
		}
		if _, exists := seen[model]; exists {
			continue
		}
		seen[model] = struct{}{}
		models = append(models, model)
	}
	slices.Sort(models)
	c.AllowedModels = models
	if c.ExpiresAt != nil {
		expiresAt := c.ExpiresAt.UTC()
		c.ExpiresAt = &expiresAt
	}
	return nil
}
