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
	return r.dao.WithCredentialIndexLock(ctx, func(dao *accesskeydao.DAO) error {
		records, err := dao.List(ctx)
		if err != nil {
			return err
		}
		credentials := make([]cache.Credential, 0, len(records))
		for _, record := range records {
			credentials = append(credentials, credentialFromAccessKey(accessKeyFromRecord(record)))
		}
		return r.credentials.Reconcile(ctx, credentials)
	})
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
	return r.dao.WithCredentialIndexLock(ctx, func(dao *accesskeydao.DAO) error {
		// 新 Secret 仅随成功响应返回，先发布不会让调用方获得无持久化记录的凭据
		if err := r.credentials.Save(ctx, credentialFromAccessKey(accessKey)); err != nil {
			return err
		}
		return accessKeyError(dao.Create(ctx, recordFromAccessKey(accessKey)))
	})
}

func (r *accessKeyRepository) Update(ctx context.Context, current, next biz.AccessKey) error {
	return r.dao.WithCredentialIndexLock(ctx, func(dao *accesskeydao.DAO) error {
		// 配置变化前先撤销旧凭据，跨存储失败时保持拒绝访问，由周期同步恢复事实状态
		if err := r.credentials.Save(ctx, revokedCredential(current)); err != nil {
			return err
		}
		if err := dao.Update(ctx, recordFromAccessKey(next)); err != nil {
			return accessKeyError(err)
		}
		return r.credentials.Save(ctx, credentialFromAccessKey(next))
	})
}

func (r *accessKeyRepository) SetEnabled(ctx context.Context, current, next biz.AccessKey) error {
	return r.dao.WithCredentialIndexLock(ctx, func(dao *accesskeydao.DAO) error {
		if err := r.credentials.Save(ctx, revokedCredential(current)); err != nil {
			return err
		}
		if err := dao.SetEnabled(ctx, recordFromAccessKey(next)); err != nil {
			return accessKeyError(err)
		}
		if !next.Enabled {
			return nil
		}
		return r.credentials.Save(ctx, credentialFromAccessKey(next))
	})
}

func (r *accessKeyRepository) Rotate(ctx context.Context, current, next biz.AccessKey) error {
	return r.dao.WithCredentialIndexLock(ctx, func(dao *accesskeydao.DAO) error {
		if err := r.credentials.Save(ctx, revokedCredential(current)); err != nil {
			return err
		}
		if err := dao.Rotate(ctx, recordFromAccessKey(next)); err != nil {
			return accessKeyError(err)
		}
		return r.credentials.Save(ctx, credentialFromAccessKey(next))
	})
}

func (r *accessKeyRepository) Delete(ctx context.Context, accessKey biz.AccessKey) error {
	return r.dao.WithCredentialIndexLock(ctx, func(dao *accesskeydao.DAO) error {
		if err := r.credentials.Delete(ctx, credentialFromAccessKey(accessKey)); err != nil {
			return err
		}
		return accessKeyError(dao.Delete(ctx, accessKey.ID))
	})
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

func revokedCredential(accessKey biz.AccessKey) cache.Credential {
	credential := credentialFromAccessKey(accessKey)
	credential.Enabled = false
	return credential
}
