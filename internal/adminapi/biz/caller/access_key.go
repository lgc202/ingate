package caller

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	"github.com/lgc202/ingate/internal/pkg/accesskey"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// IssuedKey 包含只在本次调用中存在的完整访问密钥
type IssuedKey struct {
	AccessKey resource.AccessKey
	Secret    string
}

// IssueAccessKey 为现有 Caller 签发一份新的独立密钥
func (s *Service) IssueAccessKey(
	ctx context.Context,
	callerID string,
	version int64,
	name string,
	expiresAt *time.Time,
) (IssuedKey, error) {
	current, err := s.repository.Get(ctx, callerID)
	if err != nil {
		return IssuedKey{}, err
	}
	if version != current.Generation {
		return IssuedKey{}, callerVersionConflict(current)
	}
	for _, key := range current.Spec.AccessKeys {
		if key.DisplayName == name {
			return IssuedKey{}, biz.NewRuleViolation(fmt.Sprintf("访问密钥名称 %q 已存在", name))
		}
	}

	issued, err := issueAccessKey(name, expiresAt)
	if err != nil {
		return IssuedKey{}, err
	}
	current.Spec.AccessKeys = append(current.Spec.AccessKeys, issued.AccessKey)
	if _, err := s.updateCaller(ctx, current, current.Spec); err != nil {
		return IssuedKey{}, err
	}
	return issued, nil
}

// DisableAccessKey 立即停用 Caller 下的一份访问密钥
func (s *Service) DisableAccessKey(
	ctx context.Context,
	callerID string,
	accessKeyID string,
	version int64,
) (*resource.Caller, error) {
	current, err := s.repository.Get(ctx, callerID)
	if err != nil {
		return nil, err
	}
	if version != current.Generation {
		return nil, callerVersionConflict(current)
	}

	index := slices.IndexFunc(current.Spec.AccessKeys, func(key resource.AccessKey) bool {
		return key.ID == accessKeyID
	})
	if index < 0 {
		return nil, biz.NewRuleViolation("访问密钥不存在")
	}
	if !current.Spec.AccessKeys[index].Enabled {
		return current, nil
	}
	current.Spec.AccessKeys[index].Enabled = false
	return s.updateCaller(ctx, current, current.Spec)
}

func issueAccessKey(name string, expiresAt *time.Time) (IssuedKey, error) {
	now := time.Now().UTC()
	if expiresAt != nil && !expiresAt.After(now) {
		return IssuedKey{}, biz.NewRuleViolation("访问密钥到期时间必须晚于当前时间")
	}
	keyID := uuid.NewString()
	secret, err := accesskey.Generate(keyID)
	if err != nil {
		return IssuedKey{}, fmt.Errorf("generate access key: %w", err)
	}
	key := resource.AccessKey{
		ID:           keyID,
		DisplayName:  name,
		SecretDigest: accesskey.Digest(secret),
		Enabled:      true,
		CreatedAt:    metav1.NewTime(now),
	}
	if expiresAt != nil {
		value := metav1.NewTime(expiresAt.UTC())
		key.ExpiresAt = &value
	}
	return IssuedKey{AccessKey: key, Secret: secret}, nil
}
