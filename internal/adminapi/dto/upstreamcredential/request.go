package upstreamcredential

import (
	"errors"
	"strings"

	credentialservice "github.com/lgc202/ingate/internal/adminapi/service/upstreamcredential"
	"github.com/lgc202/ingate/internal/pkg/bearer"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Validate 校验创建 UpstreamCredential 请求
func (r *CreateUpstreamCredentialReq) Validate() error {
	if err := r.UpstreamCredentialConfig.validateBase(); err != nil {
		return err
	}
	if r.APIKey == nil || r.APIKey.Value == "" {
		return errors.New("访问密钥不能为空")
	}
	if !bearer.ValidToken(r.APIKey.Value) {
		return errors.New("访问密钥格式不正确")
	}
	return nil
}

// Validate 校验更新 UpstreamCredential 请求
func (r *UpdateUpstreamCredentialReq) Validate() error {
	if r.Version == "" {
		return errors.New("访问凭据版本不能为空")
	}
	if err := r.UpstreamCredentialConfig.validateBase(); err != nil {
		return err
	}
	if r.APIKey == nil {
		return nil
	}
	if r.APIKey.Value == "" {
		return errors.New("访问密钥不能为空")
	}
	if !bearer.ValidToken(r.APIKey.Value) {
		return errors.New("访问密钥格式不正确")
	}
	return nil
}

// Validate 校验并规整 UpstreamCredential 路径参数
func (r *IDReq) Validate() error {
	r.ID = strings.TrimSpace(r.ID)
	if r.ID == "" {
		return errors.New("访问凭据 ID 不能为空")
	}
	return nil
}

// Params 将已校验的创建请求转换为 service 参数
func (r CreateUpstreamCredentialReq) Params() credentialservice.CreateParams {
	return credentialservice.CreateParams{CredentialParams: r.UpstreamCredentialConfig.params()}
}

// Params 将已校验的更新请求转换为 service 参数
func (r UpdateUpstreamCredentialReq) Params() credentialservice.UpdateParams {
	return credentialservice.UpdateParams{
		Version:          r.Version,
		CredentialParams: r.UpstreamCredentialConfig.params(),
	}
}

func (c *UpstreamCredentialConfig) validateBase() error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return errors.New("访问凭据名称不能为空")
	}
	if c.Type != resource.UpstreamCredentialTypeAPIKey {
		return errors.New("访问凭据类型不正确")
	}
	return nil
}

func (c UpstreamCredentialConfig) params() credentialservice.CredentialParams {
	params := credentialservice.CredentialParams{Name: c.Name, Type: c.Type}
	if c.APIKey != nil {
		params.APIKeyValue = c.APIKey.Value
	}
	return params
}
