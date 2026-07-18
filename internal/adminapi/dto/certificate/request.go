package certificate

import (
	"errors"
	"strings"

	certificateutil "github.com/lgc202/ingate/internal/pkg/certificate"
)

// Validate 校验创建 Certificate 请求
func (r *CreateCertificateReq) Validate() error {
	if err := r.CertificateConfig.validateBase(); err != nil {
		return err
	}
	if r.CertificatePEM == "" {
		return errors.New("证书内容不能为空")
	}
	if r.PrivateKeyPEM == "" {
		return errors.New("证书私钥不能为空")
	}
	return r.CertificateConfig.validateKeyPair()
}

// Validate 校验更新 Certificate 请求
func (r *UpdateCertificateReq) Validate() error {
	if r.Version == "" {
		return errors.New("证书版本不能为空")
	}
	if err := r.CertificateConfig.validateBase(); err != nil {
		return err
	}
	if r.CertificatePEM == "" && r.PrivateKeyPEM == "" {
		return nil
	}
	if r.CertificatePEM == "" || r.PrivateKeyPEM == "" {
		return errors.New("替换证书时必须同时提供证书内容和私钥")
	}
	return r.CertificateConfig.validateKeyPair()
}

func (c *CertificateConfig) validateBase() error {
	c.Name = strings.TrimSpace(c.Name)
	c.Description = strings.TrimSpace(c.Description)
	c.CertificatePEM = normalizePEM(c.CertificatePEM)
	c.PrivateKeyPEM = normalizePEM(c.PrivateKeyPEM)
	if c.Name == "" {
		return errors.New("证书名称不能为空")
	}
	return nil
}

func (c *CertificateConfig) validateKeyPair() error {
	if _, err := certificateutil.ParseKeyPair(c.CertificatePEM, c.PrivateKeyPEM); err != nil {
		return errors.New("证书内容与私钥格式不正确或不匹配")
	}
	return nil
}

func normalizePEM(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return value + "\n"
}
