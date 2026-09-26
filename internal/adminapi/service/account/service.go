// Package account 实现管理员账户用例。
package account

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/lgc202/ingate/internal/adminapi/domain"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	"golang.org/x/crypto/bcrypt"
)

const (
	initialAdminID       = 1
	initialAdminUsername = "admin"
	initialAdminPassword = "admin"
	sessionDuration      = 24 * time.Hour
	sessionTokenSize     = 32
)

var (
	// ErrInvalidCredentials 表示账户不存在或密码不匹配。
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrSessionRequired 表示登录态不存在、过期或已被密码修改撤销。
	ErrSessionRequired = errors.New("session required")
	// ErrUnavailable 表示认证依赖暂时不可用。
	ErrUnavailable = errors.New("authentication unavailable")
	// ErrPasswordUnchanged 表示新旧密码相同。
	ErrPasswordUnchanged = errors.New("new password is unchanged")
)

// LoginResult 保存新建的登录态及其关联账户。
type LoginResult struct {
	Token      string
	ExpireTime time.Time
	Account    domain.Account
}

// AuthenticatedSession 保存已验证的登录态身份。
type AuthenticatedSession struct {
	ID        string
	AccountID uint64
}

// Service 实现管理员账户用例。
type Service struct {
	accounts accountRepository
	sessions sessionRepository
}

type accountRepository interface {
	Create(context.Context, domain.Account) error
	Get(context.Context, uint64) (domain.Account, error)
	GetByUsername(context.Context, string) (domain.Account, error)
	UpdatePassword(context.Context, uint64, string, string) error
}

type sessionRepository interface {
	Create(context.Context, string, domain.Session, time.Time) error
	Get(context.Context, string) (domain.Session, error)
	Delete(context.Context, string) error
}

// New 创建管理员账户 Service。
func New(accounts accountRepository, sessions sessionRepository) *Service {
	return &Service{accounts: accounts, sessions: sessions}
}

// Bootstrap 在首次启动时创建默认管理员。
// 已存在的管理员不会被覆盖；并发启动时只有一个实例能够插入账户。
func (s *Service) Bootstrap(ctx context.Context) error {
	account, err := s.accounts.Get(ctx, initialAdminID)
	if err == nil {
		return requireInitialAdmin(account)
	}
	if !errors.Is(err, repository.ErrAccountNotFound) {
		return fmt.Errorf("read initial admin: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(initialAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash initial admin password: %w", err)
	}

	account = domain.Account{
		ID:             initialAdminID,
		Username:       initialAdminUsername,
		PasswordHash:   string(hash),
		SessionVersion: 1,
	}
	if err := s.accounts.Create(ctx, account); err != nil {
		if !errors.Is(err, repository.ErrAccountAlreadyExists) {
			return fmt.Errorf("create initial admin: %w", err)
		}

		// 另一个实例可能刚完成创建；只接受预期的初始管理员。
		account, err = s.accounts.Get(ctx, initialAdminID)
		if err != nil {
			return fmt.Errorf("read initial admin after conflict: %w", err)
		}
		return requireInitialAdmin(account)
	}

	return nil
}

// Login 校验管理员密码并创建 24 小时有效的 Bearer Session。
func (s *Service) Login(ctx context.Context, username, password string) (LoginResult, error) {
	account, err := s.accounts.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrAccountNotFound) {
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, fmt.Errorf("%w: read account: %w", ErrUnavailable, err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) || errors.Is(err, bcrypt.ErrPasswordTooLong) {
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, fmt.Errorf("%w: compare password: %w", ErrUnavailable, err)
	}

	randomBytes := make([]byte, sessionTokenSize)
	if _, err := rand.Read(randomBytes); err != nil {
		return LoginResult{}, fmt.Errorf("generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(randomBytes)
	expireTime := time.Now().UTC().Add(sessionDuration)
	if err := s.sessions.Create(ctx, sessionID(token), domain.Session{
		AccountID:      account.ID,
		AccountVersion: account.SessionVersion,
	}, expireTime); err != nil {
		return LoginResult{}, fmt.Errorf("%w: create session: %w", ErrUnavailable, err)
	}

	return LoginResult{Token: token, ExpireTime: expireTime, Account: account}, nil
}

// Authenticate 校验 Bearer Token 和账户当前的会话版本。
func (s *Service) Authenticate(ctx context.Context, token string) (AuthenticatedSession, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != sessionTokenSize {
		return AuthenticatedSession{}, ErrSessionRequired
	}

	id := sessionID(token)
	session, err := s.sessions.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return AuthenticatedSession{}, ErrSessionRequired
		}
		return AuthenticatedSession{}, fmt.Errorf("%w: read session: %w", ErrUnavailable, err)
	}

	account, err := s.accounts.Get(ctx, session.AccountID)
	if err != nil {
		if errors.Is(err, repository.ErrAccountNotFound) {
			return AuthenticatedSession{}, ErrSessionRequired
		}
		return AuthenticatedSession{}, fmt.Errorf("%w: read account: %w", ErrUnavailable, err)
	}
	if account.SessionVersion != session.AccountVersion {
		return AuthenticatedSession{}, ErrSessionRequired
	}

	return AuthenticatedSession{ID: id, AccountID: account.ID}, nil
}

// Logout 撤销当前登录态。
func (s *Service) Logout(ctx context.Context, sessionID string) error {
	if err := s.sessions.Delete(ctx, sessionID); err != nil {
		return fmt.Errorf("%w: delete session: %w", ErrUnavailable, err)
	}

	return nil
}

// Get 读取当前管理员账户。
func (s *Service) Get(ctx context.Context, accountID uint64) (domain.Account, error) {
	account, err := s.accounts.Get(ctx, accountID)
	if err != nil {
		if errors.Is(err, repository.ErrAccountNotFound) {
			return domain.Account{}, ErrSessionRequired
		}
		return domain.Account{}, fmt.Errorf("%w: read account: %w", ErrUnavailable, err)
	}

	return account, nil
}

// ChangePassword 校验当前密码并撤销账户的全部旧登录态。
func (s *Service) ChangePassword(ctx context.Context, accountID uint64, currentPassword, newPassword string) error {
	account, err := s.Get(ctx, accountID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(currentPassword)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) || errors.Is(err, bcrypt.ErrPasswordTooLong) {
			return ErrInvalidCredentials
		}
		return fmt.Errorf("%w: compare current password: %w", ErrUnavailable, err)
	}
	if currentPassword == newPassword {
		return ErrPasswordUnchanged
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}
	if err := s.accounts.UpdatePassword(ctx, accountID, account.PasswordHash, string(hash)); err != nil {
		if errors.Is(err, repository.ErrAccountChanged) {
			return ErrInvalidCredentials
		}
		return fmt.Errorf("%w: update password: %w", ErrUnavailable, err)
	}

	return nil
}

func requireInitialAdmin(account domain.Account) error {
	if account.Username != initialAdminUsername {
		return fmt.Errorf("account %d is not the initial admin", initialAdminID)
	}

	return nil
}

func sessionID(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}
