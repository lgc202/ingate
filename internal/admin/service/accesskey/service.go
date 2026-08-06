// Package accesskey 承载客户端访问密钥的生命周期和数据面发布语义
package accesskey

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/admin/accesskeyindex"
	"github.com/lgc202/ingate/internal/admin/pkg/xerrors"
	accesskeystore "github.com/lgc202/ingate/internal/admin/store/accesskey"
	sharedaccesskey "github.com/lgc202/ingate/internal/pkg/accesskey"
)

const (
	secretMarker       = "igk_"
	secretRandomBytes  = 32
	secretPrefixLength = 12
	secretSuffixLength = 4
)

// AccessKey 是控制台使用的访问密钥业务对象
type AccessKey struct {
	ID            string
	Name          string
	Prefix        string
	Suffix        string
	Enabled       bool
	AllowedModels []string
	ExpiresAt     *time.Time
	LastUsedAt    *time.Time
	CreatedAt     time.Time
}

// Service 管理访问密钥元数据并同步数据面执行索引
type Service struct {
	store *accesskeystore.Store
	index *accesskeyindex.Index
}

// New 创建访问密钥 service
func New(store *accesskeystore.Store, index *accesskeyindex.Index) *Service {
	return &Service{store: store, index: index}
}

// Reconcile 用 MySQL 中的当前事实重建 Redis 执行索引
func (s *Service) Reconcile(ctx context.Context) error {
	records, err := s.store.List(ctx)
	if err != nil {
		return err
	}
	credentials := make([]accesskeyindex.Credential, 0, len(records))
	for _, record := range records {
		credentials = append(credentials, credentialFromRecord(record))
	}
	return s.index.Reconcile(ctx, credentials)
}

// List 返回访问密钥列表，并合并 Redis 中的最后认证时间
func (s *Service) List(ctx context.Context) ([]AccessKey, error) {
	records, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	lastUsed, err := s.index.LastUsed(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]AccessKey, 0, len(records))
	for _, record := range records {
		key := accessKeyFromRecord(record)
		if usedAt, exists := lastUsed[record.ID]; exists {
			key.LastUsedAt = &usedAt
		}
		result = append(result, key)
	}
	return result, nil
}

// Create 创建访问密钥，完整 Secret 只随本次调用返回
func (s *Service) Create(
	ctx context.Context,
	name string,
	allowedModels []string,
	expiresAt *time.Time,
) (AccessKey, string, error) {
	if err := s.ensureNameAvailable(ctx, name, ""); err != nil {
		return AccessKey{}, "", err
	}
	secret, hash, prefix, suffix, err := newSecret()
	if err != nil {
		return AccessKey{}, "", err
	}
	now := time.Now().UTC()
	record := accesskeystore.Record{
		ID:            uuid.NewString(),
		Name:          name,
		SecretHash:    hash,
		SecretPrefix:  prefix,
		SecretSuffix:  suffix,
		Enabled:       true,
		AllowedModels: allowedModels,
		ExpiresAt:     expiresAt,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// 先发布不可猜测的新 Secret，数据库写入失败时立即撤销；调用成功前 Secret 不会离开进程
	if err := s.index.Save(ctx, credentialFromRecord(record)); err != nil {
		return AccessKey{}, "", err
	}
	if err := s.store.Create(ctx, record); err != nil {
		_ = s.index.Delete(ctx, credentialFromRecord(record))
		if errors.Is(err, accesskeystore.ErrNameConflict) {
			return AccessKey{}, "", xerrors.NewUserError(fmt.Sprintf("访问密钥名称 %q 已存在", name))
		}
		return AccessKey{}, "", err
	}
	return accessKeyFromRecord(record), secret, nil
}

// Update 全量更新访问密钥的可编辑配置，不改变 Secret
func (s *Service) Update(
	ctx context.Context,
	accessKeyID string,
	name string,
	allowedModels []string,
	expiresAt *time.Time,
) (AccessKey, error) {
	current, err := s.get(ctx, accessKeyID)
	if err != nil {
		return AccessKey{}, err
	}
	if err := s.ensureNameAvailable(ctx, name, accessKeyID); err != nil {
		return AccessKey{}, err
	}
	next := current
	next.Name = name
	next.AllowedModels = allowedModels
	next.ExpiresAt = expiresAt
	next.UpdatedAt = time.Now().UTC()

	// 模型范围收窄或有效期变更必须先进入数据面，避免接口已返回成功但旧权限仍可使用
	if err := s.index.Save(ctx, credentialFromRecord(next)); err != nil {
		return AccessKey{}, err
	}
	if err := s.store.Update(ctx, next); err != nil {
		_ = s.index.Save(ctx, credentialFromRecord(current))
		if errors.Is(err, accesskeystore.ErrNameConflict) {
			return AccessKey{}, xerrors.NewUserError(fmt.Sprintf("访问密钥名称 %q 已存在", name))
		}
		return AccessKey{}, err
	}
	return accessKeyFromRecord(next), nil
}

// SetEnabled 启用或停用访问密钥
func (s *Service) SetEnabled(ctx context.Context, accessKeyID string, enabled bool) (AccessKey, error) {
	current, err := s.get(ctx, accessKeyID)
	if err != nil {
		return AccessKey{}, err
	}
	if current.Enabled == enabled {
		return accessKeyFromRecord(current), nil
	}
	next := current
	next.Enabled = enabled
	next.UpdatedAt = time.Now().UTC()

	if enabled {
		// 启用先落事实库；索引失败时回退状态，避免数据库仍为停用却放行流量
		if err := s.store.SetEnabled(ctx, next); err != nil {
			return AccessKey{}, err
		}
		if err := s.index.Save(ctx, credentialFromRecord(next)); err != nil {
			_ = s.store.SetEnabled(ctx, current)
			return AccessKey{}, err
		}
	} else {
		// 停用先撤销数据面权限，数据库失败时恢复原索引
		if err := s.index.Save(ctx, credentialFromRecord(next)); err != nil {
			return AccessKey{}, err
		}
		if err := s.store.SetEnabled(ctx, next); err != nil {
			_ = s.index.Save(ctx, credentialFromRecord(current))
			return AccessKey{}, err
		}
	}
	return accessKeyFromRecord(next), nil
}

// Rotate 生成新 Secret 并立即撤销原 Secret
func (s *Service) Rotate(ctx context.Context, accessKeyID string) (AccessKey, string, error) {
	current, err := s.get(ctx, accessKeyID)
	if err != nil {
		return AccessKey{}, "", err
	}
	secret, hash, prefix, suffix, err := newSecret()
	if err != nil {
		return AccessKey{}, "", err
	}
	next := current
	next.SecretHash = hash
	next.SecretPrefix = prefix
	next.SecretSuffix = suffix
	next.UpdatedAt = time.Now().UTC()

	if err := s.index.Rotate(ctx, current.SecretHash, credentialFromRecord(next)); err != nil {
		return AccessKey{}, "", err
	}
	if err := s.store.Rotate(ctx, next); err != nil {
		_ = s.index.Rotate(ctx, next.SecretHash, credentialFromRecord(current))
		return AccessKey{}, "", err
	}
	return accessKeyFromRecord(next), secret, nil
}

// Delete 删除访问密钥并立即撤销数据面权限
func (s *Service) Delete(ctx context.Context, accessKeyID string) error {
	current, err := s.get(ctx, accessKeyID)
	if err != nil {
		return err
	}
	if err := s.index.Delete(ctx, credentialFromRecord(current)); err != nil {
		return err
	}
	if err := s.store.Delete(ctx, accessKeyID); err != nil {
		_ = s.index.Save(ctx, credentialFromRecord(current))
		return err
	}
	return nil
}

func (s *Service) get(ctx context.Context, accessKeyID string) (accesskeystore.Record, error) {
	record, err := s.store.Get(ctx, accessKeyID)
	if errors.Is(err, accesskeystore.ErrNotFound) {
		return accesskeystore.Record{}, xerrors.NewUserError("访问密钥不存在")
	}
	return record, err
}

func (s *Service) ensureNameAvailable(ctx context.Context, name, excludeID string) error {
	exists, err := s.store.NameExists(ctx, name, excludeID)
	if err != nil {
		return err
	}
	if exists {
		return xerrors.NewUserError(fmt.Sprintf("访问密钥名称 %q 已存在", name))
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

func accessKeyFromRecord(record accesskeystore.Record) AccessKey {
	allowedModels := make([]string, len(record.AllowedModels))
	copy(allowedModels, record.AllowedModels)
	return AccessKey{
		ID:            record.ID,
		Name:          record.Name,
		Prefix:        record.SecretPrefix,
		Suffix:        record.SecretSuffix,
		Enabled:       record.Enabled,
		AllowedModels: allowedModels,
		ExpiresAt:     record.ExpiresAt,
		CreatedAt:     record.CreatedAt,
	}
}

func credentialFromRecord(record accesskeystore.Record) accesskeyindex.Credential {
	allowedModels := make([]string, len(record.AllowedModels))
	copy(allowedModels, record.AllowedModels)
	return accesskeyindex.Credential{
		ID:            record.ID,
		SecretHash:    record.SecretHash,
		Enabled:       record.Enabled,
		AllowedModels: allowedModels,
		ExpiresAt:     record.ExpiresAt,
	}
}
