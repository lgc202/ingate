package mysql

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// Account 对应 MySQL accounts 表中的完整记录。
type Account struct {
	ID             uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	Username       string    `gorm:"column:username"`
	PasswordHash   string    `gorm:"column:password_hash"`
	SessionVersion uint64    `gorm:"column:session_version"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoCreateTime;autoUpdateTime"`
}

// TableName 返回 Account 使用的数据库表名。
func (Account) TableName() string {
	return "accounts"
}

// CreateAccount 创建账户。
func (s *Store) CreateAccount(ctx context.Context, account Account) error {
	return s.db.WithContext(ctx).Create(&account).Error
}

// GetAccount 按主键读取账户。
func (s *Store) GetAccount(ctx context.Context, id uint64) (Account, error) {
	var account Account
	if err := s.db.WithContext(ctx).First(&account, "id = ?", id).Error; err != nil {
		return Account{}, err
	}

	return account, nil
}

// GetAccountByUsername 按登录名读取账户。
func (s *Store) GetAccountByUsername(ctx context.Context, username string) (Account, error) {
	var account Account
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&account).Error; err != nil {
		return Account{}, err
	}

	return account, nil
}

// UpdatePassword 原子更新密码哈希和会话版本；密码已变化时不更新。
func (s *Store) UpdatePassword(ctx context.Context, id uint64, oldHash, newHash string) (bool, error) {
	result := s.db.WithContext(ctx).
		Model(&Account{}).
		Where("id = ? AND password_hash = ?", id, oldHash).
		Updates(map[string]any{
			"password_hash":   newHash,
			"session_version": gorm.Expr("session_version + 1"),
		})
	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected == 1, nil
}
