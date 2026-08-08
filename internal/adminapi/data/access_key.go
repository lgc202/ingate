package data

import (
	"context"
	"errors"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	"github.com/lgc202/ingate/internal/adminapi/data/cache"
	accesskeydao "github.com/lgc202/ingate/internal/adminapi/data/dao/accesskey"
)

// accessKeyRepository 组合 MySQL 中的管理面事实和 Redis 中的数据面执行索引
type accessKeyRepository struct {
	dao         *accesskeydao.DAO
	credentials *cache.CredentialIndex
}

// NewAccessKeyRepository 创建访问密钥数据访问实现
func NewAccessKeyRepository(dao *accesskeydao.DAO, credentials *cache.CredentialIndex) *accessKeyRepository {
	return &accessKeyRepository{dao: dao, credentials: credentials}
}

func (r *accessKeyRepository) Reconcile(ctx context.Context) error {
	records, err := r.dao.List(ctx)
	if err != nil {
		return err
	}
	credentials := make([]cache.Credential, 0, len(records))
	for _, record := range records {
		credentials = append(credentials, credentialFromAccessKey(accessKeyFromRecord(record)))
	}
	return r.credentials.Reconcile(ctx, credentials)
}

func (r *accessKeyRepository) List(ctx context.Context) ([]biz.AccessKey, error) {
	records, err := r.dao.List(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	lastUsed, err := r.credentials.LastUsed(ctx, ids)
	if err != nil {
		return nil, err
	}
	accessKeys := make([]biz.AccessKey, 0, len(records))
	for _, record := range records {
		accessKey := accessKeyFromRecord(record)
		if usedAt, exists := lastUsed[record.ID]; exists {
			accessKey.LastUsedAt = &usedAt
		}
		accessKeys = append(accessKeys, accessKey)
	}
	return accessKeys, nil
}

func (r *accessKeyRepository) Get(ctx context.Context, accessKeyID string) (biz.AccessKey, error) {
	record, err := r.dao.Get(ctx, accessKeyID)
	if err != nil {
		return biz.AccessKey{}, accessKeyError(err)
	}
	return accessKeyFromRecord(record), nil
}

func (r *accessKeyRepository) NameExists(ctx context.Context, name, excludeID string) (bool, error) {
	return r.dao.NameExists(ctx, name, excludeID)
}

func (r *accessKeyRepository) Create(ctx context.Context, accessKey biz.AccessKey) error {
	credential := credentialFromAccessKey(accessKey)
	// 新 Secret 不可猜测，可先发布；事实库失败时必须在返回前撤销
	if err := r.credentials.Save(ctx, credential); err != nil {
		return err
	}
	if err := r.dao.Create(ctx, recordFromAccessKey(accessKey)); err != nil {
		_ = r.credentials.Delete(ctx, credential)
		return accessKeyError(err)
	}
	return nil
}

func (r *accessKeyRepository) Update(ctx context.Context, current, next biz.AccessKey) error {
	// 权限收窄必须先进入数据面，避免接口成功后旧权限继续生效
	if err := r.credentials.Save(ctx, credentialFromAccessKey(next)); err != nil {
		return err
	}
	if err := r.dao.Update(ctx, recordFromAccessKey(next)); err != nil {
		_ = r.credentials.Save(ctx, credentialFromAccessKey(current))
		return accessKeyError(err)
	}
	return nil
}

func (r *accessKeyRepository) SetEnabled(ctx context.Context, current, next biz.AccessKey) error {
	if next.Enabled {
		if err := r.dao.SetEnabled(ctx, recordFromAccessKey(next)); err != nil {
			return accessKeyError(err)
		}
		if err := r.credentials.Save(ctx, credentialFromAccessKey(next)); err != nil {
			_ = r.dao.SetEnabled(ctx, recordFromAccessKey(current))
			return err
		}
		return nil
	}

	if err := r.credentials.Save(ctx, credentialFromAccessKey(next)); err != nil {
		return err
	}
	if err := r.dao.SetEnabled(ctx, recordFromAccessKey(next)); err != nil {
		_ = r.credentials.Save(ctx, credentialFromAccessKey(current))
		return accessKeyError(err)
	}
	return nil
}

func (r *accessKeyRepository) Rotate(ctx context.Context, current, next biz.AccessKey) error {
	if err := r.credentials.Rotate(ctx, current.SecretHash, credentialFromAccessKey(next)); err != nil {
		return err
	}
	if err := r.dao.Rotate(ctx, recordFromAccessKey(next)); err != nil {
		_ = r.credentials.Rotate(ctx, next.SecretHash, credentialFromAccessKey(current))
		return accessKeyError(err)
	}
	return nil
}

func (r *accessKeyRepository) Delete(ctx context.Context, accessKey biz.AccessKey) error {
	credential := credentialFromAccessKey(accessKey)
	if err := r.credentials.Delete(ctx, credential); err != nil {
		return err
	}
	if err := r.dao.Delete(ctx, accessKey.ID); err != nil {
		_ = r.credentials.Save(ctx, credential)
		return accessKeyError(err)
	}
	return nil
}

func accessKeyError(err error) error {
	switch {
	case errors.Is(err, accesskeydao.ErrNotFound):
		return biz.ErrAccessKeyNotFound
	case errors.Is(err, accesskeydao.ErrNameConflict):
		return biz.ErrAccessKeyNameConflict
	default:
		return err
	}
}

func accessKeyFromRecord(record accesskeydao.Record) biz.AccessKey {
	return biz.AccessKey{
		ID:            record.ID,
		Name:          record.Name,
		SecretHash:    record.SecretHash,
		SecretPrefix:  record.SecretPrefix,
		SecretSuffix:  record.SecretSuffix,
		Enabled:       record.Enabled,
		AllowedModels: append([]string(nil), record.AllowedModels...),
		ExpiresAt:     record.ExpiresAt,
		CreatedAt:     record.CreatedAt,
		UpdatedAt:     record.UpdatedAt,
	}
}

func recordFromAccessKey(accessKey biz.AccessKey) accesskeydao.Record {
	return accesskeydao.Record{
		ID:            accessKey.ID,
		Name:          accessKey.Name,
		SecretHash:    accessKey.SecretHash,
		SecretPrefix:  accessKey.SecretPrefix,
		SecretSuffix:  accessKey.SecretSuffix,
		Enabled:       accessKey.Enabled,
		AllowedModels: append([]string(nil), accessKey.AllowedModels...),
		ExpiresAt:     accessKey.ExpiresAt,
		CreatedAt:     accessKey.CreatedAt,
		UpdatedAt:     accessKey.UpdatedAt,
	}
}

func credentialFromAccessKey(accessKey biz.AccessKey) cache.Credential {
	return cache.Credential{
		ID:            accessKey.ID,
		SecretHash:    accessKey.SecretHash,
		Enabled:       accessKey.Enabled,
		AllowedModels: append([]string(nil), accessKey.AllowedModels...),
		ExpiresAt:     accessKey.ExpiresAt,
	}
}
