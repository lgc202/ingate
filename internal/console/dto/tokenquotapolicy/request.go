package tokenquotapolicy

import (
	"errors"
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"

	consoledto "github.com/lgc202/ingate/internal/console/dto"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Validate 校验 TokenQuotaPolicy 路径参数
func (r *IDReq) Validate() error {
	r.ID = strings.TrimSpace(r.ID)
	if r.ID == "" {
		return errors.New("Token 配额策略 ID 不能为空")
	}
	return nil
}

// Validate 校验创建 TokenQuotaPolicy 请求
func (r *CreateTokenQuotaPolicyReq) Validate() error {
	return r.TokenQuotaPolicyConfig.Validate()
}

// Validate 校验更新 TokenQuotaPolicy 请求
func (r *UpdateTokenQuotaPolicyReq) Validate() error {
	if r.Version == "" {
		return errors.New("版本不能为空")
	}
	return r.TokenQuotaPolicyConfig.Validate()
}

// Validate 校验启用状态请求
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

// Validate 校验 Token 配额策略核心配置
func (c *TokenQuotaPolicyConfig) Validate() error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return errors.New("名称不能为空")
	}
	if err := consoledto.ValidatePolicyTargets(c.Targets); err != nil {
		return err
	}
	if err := c.Subject.Validate(); err != nil {
		return err
	}
	if c.Quota.Tokens <= 0 {
		return errors.New("Token 额度必须大于 0")
	}
	if c.Quota.Tokens > resource.TokenQuotaMaxTokens {
		return errors.New("Token 额度超出支持范围")
	}
	if c.Quota.WindowSeconds <= 0 {
		return errors.New("统计周期必须大于 0")
	}
	if c.Quota.WindowSeconds > resource.TokenQuotaMaxWindowSeconds {
		return errors.New("统计周期超出支持范围")
	}
	switch c.FailurePolicy {
	case FailurePolicyFailOpen, FailurePolicyFailClose:
	default:
		return errors.New("失败策略不正确")
	}
	return nil
}

// Validate 校验 Token 预算池共享维度
func (s *Subject) Validate() error {
	s.HeaderName = strings.TrimSpace(s.HeaderName)
	switch s.Type {
	case SubjectTypeShared, SubjectTypeIP:
		s.HeaderName = ""
		return nil
	case SubjectTypeHeader:
		if s.HeaderName == "" {
			return errors.New("按请求头区分额度时必须填写请求头名称")
		}
		if len(k8svalidation.IsHTTPHeaderName(s.HeaderName)) > 0 {
			return errors.New("请求头名称不正确")
		}
		return nil
	default:
		return errors.New("额度划分方式不正确")
	}
}
