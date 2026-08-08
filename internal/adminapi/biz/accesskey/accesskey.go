// Package accesskey 实现访问密钥管理用例
package accesskey

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	sharedaccesskey "github.com/lgc202/ingate/internal/pkg/accesskey"
)

const (
	secretMarker       = "igk_"
	secretRandomBytes  = 32
	secretPrefixLength = 12
	secretSuffixLength = 4
)

// Key 是访问密钥业务对象，原始 Secret 不进入持久化层
type Key struct {
	ID            string
	Name          string
	SecretHash    [32]byte
	SecretPrefix  string
	SecretSuffix  string
	Enabled       bool
	AllowedModels []string
	ExpiresAt     *time.Time
	LastUsedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Repository 定义访问密钥用例需要的持久化能力
type Repository interface {
	List(context.Context) ([]Key, error)
	Get(context.Context, string) (Key, error)
	NameExists(context.Context, string, string) (bool, error)
	Create(context.Context, Key) error
	Update(context.Context, Key, Key) error
	SetEnabled(context.Context, Key, Key) error
	Rotate(context.Context, Key, Key) error
	Delete(context.Context, Key) error
}

// Usecase 管理访问密钥的业务生命周期
type Usecase struct {
	repository Repository
}

// NewUsecase 创建访问密钥用例
func NewUsecase(repository Repository) *Usecase {
	return &Usecase{repository: repository}
}

// List 返回访问密钥列表，并合并 Redis 中的最后认证时间
func (u *Usecase) List(ctx context.Context) ([]Key, error) {
	return u.repository.List(ctx)
}

// Create 创建访问密钥，完整 Secret 只随本次调用返回
func (u *Usecase) Create(
	ctx context.Context,
	name string,
	allowedModels []string,
	expiresAt *time.Time,
) (Key, string, error) {
	if err := u.ensureNameAvailable(ctx, name, ""); err != nil {
		return Key{}, "", err
	}
	secret, hash, prefix, suffix, err := newSecret()
	if err != nil {
		return Key{}, "", err
	}
	now := time.Now().UTC()
	accessKey := Key{
		ID:            uuid.NewString(),
		Name:          name,
		SecretHash:    hash,
		SecretPrefix:  prefix,
		SecretSuffix:  suffix,
		Enabled:       true,
		AllowedModels: append([]string(nil), allowedModels...),
		ExpiresAt:     expiresAt,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := u.repository.Create(ctx, accessKey); err != nil {
		if errors.Is(err, biz.ErrAccessKeyNameConflict) {
			return Key{}, "", biz.NewUserError(fmt.Sprintf("访问密钥名称 %q 已存在", name))
		}
		return Key{}, "", err
	}
	return accessKey, secret, nil
}

// Update 全量更新访问密钥的可编辑配置，不改变 Secret
func (u *Usecase) Update(
	ctx context.Context,
	accessKeyID string,
	name string,
	allowedModels []string,
	expiresAt *time.Time,
) (Key, error) {
	current, err := u.get(ctx, accessKeyID)
	if err != nil {
		return Key{}, err
	}
	if err := u.ensureNameAvailable(ctx, name, accessKeyID); err != nil {
		return Key{}, err
	}
	next := current
	next.Name = name
	next.AllowedModels = append([]string(nil), allowedModels...)
	next.ExpiresAt = expiresAt
	next.UpdatedAt = time.Now().UTC()

	if err := u.repository.Update(ctx, current, next); err != nil {
		if errors.Is(err, biz.ErrAccessKeyNameConflict) {
			return Key{}, biz.NewUserError(fmt.Sprintf("访问密钥名称 %q 已存在", name))
		}
		return Key{}, err
	}
	return next, nil
}

// SetEnabled 启用或停用访问密钥
func (u *Usecase) SetEnabled(ctx context.Context, accessKeyID string, enabled bool) (Key, error) {
	current, err := u.get(ctx, accessKeyID)
	if err != nil {
		return Key{}, err
	}
	if current.Enabled == enabled {
		return current, nil
	}
	next := current
	next.Enabled = enabled
	next.UpdatedAt = time.Now().UTC()

	if err := u.repository.SetEnabled(ctx, current, next); err != nil {
		return Key{}, err
	}
	return next, nil
}

// Rotate 生成新 Secret 并立即撤销原 Secret
func (u *Usecase) Rotate(ctx context.Context, accessKeyID string) (Key, string, error) {
	current, err := u.get(ctx, accessKeyID)
	if err != nil {
		return Key{}, "", err
	}
	secret, hash, prefix, suffix, err := newSecret()
	if err != nil {
		return Key{}, "", err
	}
	next := current
	next.SecretHash = hash
	next.SecretPrefix = prefix
	next.SecretSuffix = suffix
	next.UpdatedAt = time.Now().UTC()

	if err := u.repository.Rotate(ctx, current, next); err != nil {
		return Key{}, "", err
	}
	return next, secret, nil
}

// Delete 删除访问密钥并立即撤销数据面权限
func (u *Usecase) Delete(ctx context.Context, accessKeyID string) error {
	current, err := u.get(ctx, accessKeyID)
	if err != nil {
		return err
	}
	return u.repository.Delete(ctx, current)
}

func (u *Usecase) get(ctx context.Context, accessKeyID string) (Key, error) {
	accessKey, err := u.repository.Get(ctx, accessKeyID)
	if errors.Is(err, biz.ErrAccessKeyNotFound) {
		return Key{}, biz.NewUserError("访问密钥不存在")
	}
	return accessKey, err
}

func (u *Usecase) ensureNameAvailable(ctx context.Context, name, excludeID string) error {
	exists, err := u.repository.NameExists(ctx, name, excludeID)
	if err != nil {
		return err
	}
	if exists {
		return biz.NewUserError(fmt.Sprintf("访问密钥名称 %q 已存在", name))
	}
	return nil
}

func newSecret() (string, [32]byte, string, string, error) {
	random := make([]byte, secretRandomBytes)
	if _, err := rand.Read(random); err != nil {
		return "", [32]byte{}, "", "", fmt.Errorf("generate access key secret: %w", err)
	}
	secret := secretMarker + base64.RawURLEncoding.EncodeToString(random)
	return secret,
		sharedaccesskey.Hash(secret),
		secret[:secretPrefixLength],
		secret[len(secret)-secretSuffixLength:],
		nil
}
