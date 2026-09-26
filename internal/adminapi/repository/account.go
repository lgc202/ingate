package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/domain"
	"github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	"gorm.io/gorm"
)

var (
	// ErrAccountNotFound 表示账户不存在。
	ErrAccountNotFound = errors.New("account not found")
	// ErrAccountAlreadyExists 表示账户主键或登录名已被占用。
	ErrAccountAlreadyExists = errors.New("account already exists")
	// ErrAccountChanged 表示账户密码已被其他请求修改。
	ErrAccountChanged = errors.New("account changed")
)

// Account 提供管理员账户的持久化操作。
type Account struct {
	store *mysql.Store
}

// NewAccount 创建 Account Repository。
func NewAccount(store *mysql.Store) *Account {
	return &Account{store: store}
}

// Create 创建账户。
func (r *Account) Create(ctx context.Context, account domain.Account) error {
	row := mysql.Account{
		ID:             account.ID,
		Username:       account.Username,
		PasswordHash:   account.PasswordHash,
		SessionVersion: account.SessionVersion,
	}
	if err := r.store.CreateAccount(ctx, row); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("%w: %w", ErrAccountAlreadyExists, err)
		}
		return err
	}

	return nil
}

// Get 按主键读取账户。
func (r *Account) Get(ctx context.Context, id uint64) (domain.Account, error) {
	row, err := r.store.GetAccount(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Account{}, fmt.Errorf("%w: %w", ErrAccountNotFound, err)
		}
		return domain.Account{}, err
	}

	return accountFromMySQL(row), nil
}

// GetByUsername 按登录名读取账户。
func (r *Account) GetByUsername(ctx context.Context, username string) (domain.Account, error) {
	row, err := r.store.GetAccountByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Account{}, fmt.Errorf("%w: %w", ErrAccountNotFound, err)
		}
		return domain.Account{}, err
	}

	return accountFromMySQL(row), nil
}

// UpdatePassword 修改账户密码；并发修改时返回 ErrAccountChanged。
func (r *Account) UpdatePassword(ctx context.Context, id uint64, oldHash, newHash string) error {
	updated, err := r.store.UpdatePassword(ctx, id, oldHash, newHash)
	if err != nil {
		return err
	}
	if !updated {
		return ErrAccountChanged
	}

	return nil
}

func accountFromMySQL(row mysql.Account) domain.Account {
	return domain.Account{
		ID:             row.ID,
		Username:       row.Username,
		PasswordHash:   row.PasswordHash,
		SessionVersion: row.SessionVersion,
	}
}
