package caller

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/apperror"
	"github.com/lgc202/ingate/internal/pkg/accesskey"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/callerconfig"
)

// IssuedAccessKey 包含只在本次调用中存在的完整访问密钥。
type IssuedAccessKey struct {
	AccessKey resource.AccessKey
	Secret    string
}

// IssueAccessKeyInput 描述签发访问密钥所需的信息。
type IssueAccessKeyInput struct {
	ExpectedGeneration int64
	DisplayName        string
	ExpiresAt          *time.Time
}

// IssueAccessKey 为现有 Caller 签发一份新的独立密钥。
func (uc *Usecase) IssueAccessKey(
	ctx context.Context,
	callerID string,
	input IssueAccessKeyInput,
) (IssuedAccessKey, error) {
	current, err := uc.store.Get(ctx, callerID)
	if err != nil {
		return IssuedAccessKey{}, err
	}
	if current.Generation != input.ExpectedGeneration {
		return IssuedAccessKey{}, apperror.ResourceVersionConflict()
	}
	if len(current.Spec.AccessKeys) >= callerconfig.MaxAccessKeys {
		return IssuedAccessKey{}, adminv1.ErrorBusinessRuleViolation("访问密钥数量已达上限")
	}
	for _, accessKey := range current.Spec.AccessKeys {
		if strings.EqualFold(accessKey.DisplayName, input.DisplayName) {
			return IssuedAccessKey{}, adminv1.ErrorResourceAlreadyExists("%s", fmt.Sprintf("访问密钥名称 %q 已存在", input.DisplayName))
		}
	}

	issuedAccessKey, err := newAccessKey(input.DisplayName, input.ExpiresAt)
	if err != nil {
		return IssuedAccessKey{}, err
	}
	spec := current.Spec
	spec.AccessKeys = append(slices.Clone(spec.AccessKeys), issuedAccessKey.AccessKey)
	if _, err := uc.store.ReplaceSpec(
		ctx,
		current,
		spec,
	); err != nil {
		return IssuedAccessKey{}, err
	}
	return issuedAccessKey, nil
}

// DisableAccessKey 立即停用 Caller 下的一份访问密钥。
func (uc *Usecase) DisableAccessKey(
	ctx context.Context,
	callerID string,
	accessKeyID string,
	expectedGeneration int64,
) (*resource.Caller, error) {
	current, err := uc.store.Get(ctx, callerID)
	if err != nil {
		return nil, err
	}
	if current.Generation != expectedGeneration {
		return nil, apperror.ResourceVersionConflict()
	}

	spec := current.Spec
	spec.AccessKeys = slices.Clone(spec.AccessKeys)
	index := slices.IndexFunc(spec.AccessKeys, func(accessKey resource.AccessKey) bool {
		return accessKey.ID == accessKeyID
	})
	if index < 0 {
		return nil, adminv1.ErrorResourceNotFound("访问密钥不存在")
	}
	if !spec.AccessKeys[index].Enabled {
		return current, nil
	}
	spec.AccessKeys[index].Enabled = false
	return uc.store.ReplaceSpec(ctx, current, spec)
}

func newAccessKey(displayName string, expiresAt *time.Time) (IssuedAccessKey, error) {
	now := time.Now().UTC()
	if expiresAt != nil && !expiresAt.After(now) {
		return IssuedAccessKey{}, adminv1.ErrorInvalidArgument("访问密钥到期时间必须晚于当前时间")
	}
	accessKeyID := uuid.NewString()
	secret, err := accesskey.Generate(accessKeyID)
	if err != nil {
		return IssuedAccessKey{}, fmt.Errorf("generate access key: %w", err)
	}
	accessKey := resource.AccessKey{
		ID:           accessKeyID,
		DisplayName:  displayName,
		SecretDigest: accesskey.Digest(secret),
		Enabled:      true,
		CreatedAt:    metav1.NewTime(now),
	}
	if expiresAt != nil {
		expiration := metav1.NewTime(expiresAt.UTC())
		accessKey.ExpiresAt = &expiration
	}
	return IssuedAccessKey{AccessKey: accessKey, Secret: secret}, nil
}
