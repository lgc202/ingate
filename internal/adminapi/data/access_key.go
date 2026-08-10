package data

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	accesskeybiz "github.com/lgc202/ingate/internal/adminapi/biz/accesskey"
	"github.com/lgc202/ingate/internal/adminapi/data/cache"
	accesskeydao "github.com/lgc202/ingate/internal/adminapi/data/dao/accesskey"
)

// accessKeyRepository 组合 MySQL 中的管理面事实和 Redis 中的数据面执行索引
type accessKeyRepository struct {
	dao         *accesskeydao.DAO
	credentials *cache.CredentialIndex
	logger      *slog.Logger
}

// NewAccessKeyRepository 创建访问密钥数据访问实现
func NewAccessKeyRepository(
	dao *accesskeydao.DAO,
	credentials *cache.CredentialIndex,
	logger *slog.Logger,
) *accessKeyRepository {
	return &accessKeyRepository{dao: dao, credentials: credentials, logger: logger}
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

func (r *accessKeyRepository) List(ctx context.Context, page biz.PageRequest) (biz.PageResult[accesskeybiz.Key], error) {
	cursor, err := decodeAccessKeyCursor(page.Cursor)
	if err != nil {
		return biz.PageResult[accesskeybiz.Key]{}, err
	}
	records, err := r.dao.ListPage(ctx, page.Limit+1, cursor)
	if err != nil {
		return biz.PageResult[accesskeybiz.Key]{}, err
	}
	nextCursor := ""
	if int64(len(records)) > page.Limit {
		last := records[page.Limit-1]
		records = records[:page.Limit]
		nextCursor, err = encodeAccessKeyCursor(accesskeydao.PageCursor{CreatedAt: last.CreatedAt, ID: last.ID})
		if err != nil {
			return biz.PageResult[accesskeybiz.Key]{}, err
		}
	}
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	lastUsed, err := r.credentials.LastUsed(ctx, ids)
	if err != nil {
		// 最近使用时间是展示信息，Redis 短暂不可用时不应阻断 MySQL 中的访问密钥列表
		r.logger.WarnContext(ctx, "read access key last used time failed", "err", err)
		lastUsed = nil
	}
	accessKeys := make([]accesskeybiz.Key, 0, len(records))
	for _, record := range records {
		accessKey := accessKeyFromRecord(record)
		if usedAt, exists := lastUsed[record.ID]; exists {
			accessKey.LastUsedAt = &usedAt
		}
		accessKeys = append(accessKeys, accessKey)
	}
	return biz.PageResult[accesskeybiz.Key]{Items: accessKeys, NextCursor: nextCursor}, nil
}

func decodeAccessKeyCursor(value string) (*accesskeydao.PageCursor, error) {
	if value == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("%w: decode access key cursor", biz.ErrInvalidCursor)
	}
	var cursor accesskeydao.PageCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil || cursor.CreatedAt.IsZero() || cursor.ID == "" {
		return nil, fmt.Errorf("%w: parse access key cursor", biz.ErrInvalidCursor)
	}
	return &cursor, nil
}

func encodeAccessKeyCursor(cursor accesskeydao.PageCursor) (string, error) {
	encoded, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("encode access key cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func (r *accessKeyRepository) NameExists(ctx context.Context, name, excludeID string) (bool, error) {
	return r.dao.NameExists(ctx, name, excludeID)
}

func (r *accessKeyRepository) Create(ctx context.Context, accessKey accesskeybiz.Key) error {
	return r.dao.WithCredentialIndexLock(ctx, func(dao *accesskeydao.DAO) error {
		// 新 Secret 仅随成功响应返回，先发布不会让调用方获得无持久化记录的凭据
		if err := r.credentials.Save(ctx, credentialFromAccessKey(accessKey)); err != nil {
			return err
		}
		if err := dao.Create(ctx, recordFromAccessKey(accessKey)); err != nil {
			if cleanupErr := r.credentials.Delete(ctx, credentialFromAccessKey(accessKey)); cleanupErr != nil {
				r.logger.WarnContext(ctx, "clean up unpublished access key failed", "err", cleanupErr)
			}
			return accessKeyError(err)
		}
		return nil
	})
}

func (r *accessKeyRepository) Update(ctx context.Context, accessKeyID string, input accesskeybiz.UpdateInput) (accesskeybiz.Key, error) {
	var next accesskeybiz.Key
	err := r.dao.WithCredentialIndexLock(ctx, func(dao *accesskeydao.DAO) error {
		current, err := r.current(ctx, dao, accessKeyID, input.Version)
		if err != nil {
			return err
		}
		next = current
		next.Version++
		next.Name = input.Name
		next.AllowedModels = append([]string(nil), input.AllowedModels...)
		next.ExpiresAt = input.ExpiresAt
		next.UpdatedAt = input.UpdatedAt
		// 配置变化前先撤销旧凭据，跨存储失败时保持拒绝访问，由周期同步恢复事实状态
		if err := r.credentials.Save(ctx, revokedCredential(current)); err != nil {
			return err
		}
		if err := dao.Update(ctx, recordFromAccessKey(next), current.Version); err != nil {
			return accessKeyError(err)
		}
		return r.credentials.Save(ctx, credentialFromAccessKey(next))
	})
	return next, err
}

func (r *accessKeyRepository) SetEnabled(ctx context.Context, accessKeyID string, input accesskeybiz.EnabledInput) (accesskeybiz.Key, error) {
	var next accesskeybiz.Key
	err := r.dao.WithCredentialIndexLock(ctx, func(dao *accesskeydao.DAO) error {
		current, err := r.current(ctx, dao, accessKeyID, input.Version)
		if err != nil {
			return err
		}
		if current.Enabled == input.Enabled {
			next = current
			return nil
		}
		next = current
		next.Version++
		next.Enabled = input.Enabled
		next.UpdatedAt = input.UpdatedAt
		if err := r.credentials.Save(ctx, revokedCredential(current)); err != nil {
			return err
		}
		if err := dao.SetEnabled(ctx, recordFromAccessKey(next), current.Version); err != nil {
			return accessKeyError(err)
		}
		if !next.Enabled {
			return nil
		}
		return r.credentials.Save(ctx, credentialFromAccessKey(next))
	})
	return next, err
}

func (r *accessKeyRepository) Rotate(ctx context.Context, accessKeyID string, input accesskeybiz.RotationInput) (accesskeybiz.Key, error) {
	var next accesskeybiz.Key
	err := r.dao.WithCredentialIndexLock(ctx, func(dao *accesskeydao.DAO) error {
		current, err := r.current(ctx, dao, accessKeyID, input.Version)
		if err != nil {
			return err
		}
		next = current
		next.Version++
		next.SecretHash = input.SecretHash
		next.SecretPrefix = input.SecretPrefix
		next.SecretSuffix = input.SecretSuffix
		next.UpdatedAt = input.UpdatedAt
		if err := r.credentials.Save(ctx, revokedCredential(current)); err != nil {
			return err
		}
		if err := dao.Rotate(ctx, recordFromAccessKey(next), current.Version); err != nil {
			return accessKeyError(err)
		}
		return r.credentials.Save(ctx, credentialFromAccessKey(next))
	})
	return next, err
}

func (r *accessKeyRepository) Delete(ctx context.Context, accessKeyID string, version int64) error {
	return r.dao.WithCredentialIndexLock(ctx, func(dao *accesskeydao.DAO) error {
		accessKey, err := r.current(ctx, dao, accessKeyID, version)
		if err != nil {
			return err
		}
		if err := r.credentials.Delete(ctx, credentialFromAccessKey(accessKey)); err != nil {
			return err
		}
		return accessKeyError(dao.Delete(ctx, accessKey.ID, accessKey.Version))
	})
}

func (r *accessKeyRepository) current(
	ctx context.Context,
	dao *accesskeydao.DAO,
	accessKeyID string,
	version int64,
) (accesskeybiz.Key, error) {
	record, err := dao.Get(ctx, accessKeyID)
	if err != nil {
		return accesskeybiz.Key{}, accessKeyError(err)
	}
	if record.Version != version {
		return accesskeybiz.Key{}, biz.ErrResourceVersionConflict
	}
	return accessKeyFromRecord(record), nil
}

func accessKeyError(err error) error {
	switch {
	case errors.Is(err, accesskeydao.ErrNotFound):
		return biz.ErrAccessKeyNotFound
	case errors.Is(err, accesskeydao.ErrNameConflict):
		return biz.ErrAccessKeyNameConflict
	case errors.Is(err, accesskeydao.ErrVersionConflict):
		return biz.ErrResourceVersionConflict
	default:
		return err
	}
}

func accessKeyFromRecord(record accesskeydao.Record) accesskeybiz.Key {
	return accesskeybiz.Key{
		ID:            record.ID,
		Version:       record.Version,
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

func recordFromAccessKey(accessKey accesskeybiz.Key) accesskeydao.Record {
	return accesskeydao.Record{
		ID:            accessKey.ID,
		Version:       accessKey.Version,
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

func credentialFromAccessKey(accessKey accesskeybiz.Key) cache.Credential {
	return cache.Credential{
		ID:            accessKey.ID,
		SecretHash:    accessKey.SecretHash,
		Enabled:       accessKey.Enabled,
		AllowedModels: append([]string(nil), accessKey.AllowedModels...),
		ExpiresAt:     accessKey.ExpiresAt,
	}
}

func revokedCredential(accessKey accesskeybiz.Key) cache.Credential {
	credential := credentialFromAccessKey(accessKey)
	credential.Enabled = false
	return credential
}
