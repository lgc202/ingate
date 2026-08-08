// Package accesskey 通过 MySQL 持久化访问密钥元数据
package accesskey

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	accesskeysql "github.com/lgc202/ingate/internal/adminapi/data/dao/accesskey/sqlc"
)

const (
	credentialIndexLockName           = "ingate:access-key-index"
	credentialIndexLockAcquireSQL     = "SELECT GET_LOCK(?, ?)"
	credentialIndexLockReleaseSQL     = "SELECT RELEASE_LOCK(?)"
	credentialIndexLockTimeoutSeconds = 5
	credentialIndexUnlockTimeout      = 5 * time.Second
)

var (
	// ErrNotFound 表示访问密钥不存在
	ErrNotFound = errors.New("access key not found")
	// ErrNameConflict 表示访问密钥名称违反唯一约束
	ErrNameConflict = errors.New("access key name already exists")
)

// Record 是管理面持久化的访问密钥，不保存可恢复的原始 Secret
type Record struct {
	ID            string
	Name          string
	SecretHash    [32]byte
	SecretPrefix  string
	SecretSuffix  string
	Enabled       bool
	AllowedModels []string
	ExpiresAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// DAO 封装访问密钥的 sqlc 数据访问代码
type DAO struct {
	db      *sql.DB
	queries *accesskeysql.Queries
}

// New 创建访问密钥 DAO
func New(db *sql.DB) *DAO {
	return &DAO{db: db, queries: accesskeysql.New(db)}
}

// WithCredentialIndexLock 跨 Admin API 实例串行化 MySQL 事实与 Redis 执行索引的变更
func (d *DAO) WithCredentialIndexLock(ctx context.Context, operation func(*DAO) error) (resultErr error) {
	connection, err := d.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open credential index lock connection: %w", err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, connection.Close())
	}()

	var acquired sql.NullInt64
	if err := connection.QueryRowContext(
		ctx,
		credentialIndexLockAcquireSQL,
		credentialIndexLockName,
		credentialIndexLockTimeoutSeconds,
	).Scan(&acquired); err != nil {
		return fmt.Errorf("acquire credential index lock: %w", err)
	}
	if !acquired.Valid || acquired.Int64 != 1 {
		return errors.New("credential index lock is unavailable")
	}

	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), credentialIndexUnlockTimeout)
		defer cancel()
		var released sql.NullInt64
		if err := connection.QueryRowContext(releaseCtx, credentialIndexLockReleaseSQL, credentialIndexLockName).Scan(&released); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("release credential index lock: %w", err))
			return
		}
		if !released.Valid || released.Int64 != 1 {
			resultErr = errors.Join(resultErr, errors.New("credential index lock was not released"))
		}
	}()

	return operation(&DAO{db: d.db, queries: accesskeysql.New(connection)})
}

// List 返回全部访问密钥元数据
func (d *DAO) List(ctx context.Context) ([]Record, error) {
	rows, err := d.queries.ListAccessKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("list access keys: %w", err)
	}
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		record, err := recordFromSQL(row)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

// Get 返回指定访问密钥元数据
func (d *DAO) Get(ctx context.Context, accessKeyID string) (Record, error) {
	row, err := d.queries.GetAccessKey(ctx, accessKeyID)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, fmt.Errorf("get access key: %w", err)
	}
	return recordFromSQL(row)
}

// NameExists 判断同名访问密钥是否已存在
func (d *DAO) NameExists(ctx context.Context, name, excludeID string) (bool, error) {
	count, err := d.queries.CountAccessKeysByName(ctx, &accesskeysql.CountAccessKeysByNameParams{
		Name: name,
		ID:   excludeID,
	})
	if err != nil {
		return false, fmt.Errorf("count access keys by name: %w", err)
	}
	return count > 0, nil
}

// Create 保存新访问密钥元数据
func (d *DAO) Create(ctx context.Context, record Record) error {
	allowedModels, err := json.Marshal(record.AllowedModels)
	if err != nil {
		return fmt.Errorf("encode access key models: %w", err)
	}
	err = d.queries.CreateAccessKey(ctx, &accesskeysql.CreateAccessKeyParams{
		ID:            record.ID,
		Name:          record.Name,
		SecretHash:    record.SecretHash[:],
		SecretPrefix:  record.SecretPrefix,
		SecretSuffix:  record.SecretSuffix,
		Enabled:       record.Enabled,
		AllowedModels: allowedModels,
		ExpiresAt:     nullableTime(record.ExpiresAt),
		CreatedAt:     record.CreatedAt,
		UpdatedAt:     record.UpdatedAt,
	})
	if isNameConflict(err) {
		return ErrNameConflict
	}
	if err != nil {
		return fmt.Errorf("create access key: %w", err)
	}
	return nil
}

// Update 保存访问密钥名称、模型范围和有效期
func (d *DAO) Update(ctx context.Context, record Record) error {
	allowedModels, err := json.Marshal(record.AllowedModels)
	if err != nil {
		return fmt.Errorf("encode access key models: %w", err)
	}
	result, err := d.queries.UpdateAccessKey(ctx, &accesskeysql.UpdateAccessKeyParams{
		Name:          record.Name,
		AllowedModels: allowedModels,
		ExpiresAt:     nullableTime(record.ExpiresAt),
		UpdatedAt:     record.UpdatedAt,
		ID:            record.ID,
	})
	if isNameConflict(err) {
		return ErrNameConflict
	}
	return checkResult(result, err, "update access key")
}

// SetEnabled 修改访问密钥启用状态
func (d *DAO) SetEnabled(ctx context.Context, record Record) error {
	result, err := d.queries.SetAccessKeyEnabled(ctx, &accesskeysql.SetAccessKeyEnabledParams{
		Enabled:   record.Enabled,
		UpdatedAt: record.UpdatedAt,
		ID:        record.ID,
	})
	return checkResult(result, err, "set access key enabled")
}

// Rotate 替换访问密钥哈希和展示片段
func (d *DAO) Rotate(ctx context.Context, record Record) error {
	result, err := d.queries.RotateAccessKey(ctx, &accesskeysql.RotateAccessKeyParams{
		SecretHash:   record.SecretHash[:],
		SecretPrefix: record.SecretPrefix,
		SecretSuffix: record.SecretSuffix,
		UpdatedAt:    record.UpdatedAt,
		ID:           record.ID,
	})
	return checkResult(result, err, "rotate access key")
}

// Delete 删除访问密钥元数据
func (d *DAO) Delete(ctx context.Context, accessKeyID string) error {
	result, err := d.queries.DeleteAccessKey(ctx, accessKeyID)
	return checkResult(result, err, "delete access key")
}

func recordFromSQL(row *accesskeysql.AccessKey) (Record, error) {
	if len(row.SecretHash) != len(Record{}.SecretHash) {
		return Record{}, fmt.Errorf("access key %q has invalid secret hash length %d", row.ID, len(row.SecretHash))
	}
	var allowedModels []string
	if err := json.Unmarshal(row.AllowedModels, &allowedModels); err != nil {
		return Record{}, fmt.Errorf("decode access key %q models: %w", row.ID, err)
	}
	record := Record{
		ID:            row.ID,
		Name:          row.Name,
		SecretPrefix:  row.SecretPrefix,
		SecretSuffix:  row.SecretSuffix,
		Enabled:       row.Enabled,
		AllowedModels: allowedModels,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
	copy(record.SecretHash[:], row.SecretHash)
	if row.ExpiresAt.Valid {
		record.ExpiresAt = &row.ExpiresAt.Time
	}
	return record, nil
}

func nullableTime(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: value.UTC(), Valid: true}
}

func checkResult(result sql.Result, err error, operation string) error {
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s rows affected: %w", operation, err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func isNameConflict(err error) bool {
	mySQLError, ok := errors.AsType[*mysql.MySQLError](err)
	return ok && mySQLError.Number == 1062 && strings.Contains(mySQLError.Message, "access_keys_name_uq")
}
