package account

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lgc202/ingate/internal/adminapi/domain"
	"github.com/lgc202/ingate/internal/adminapi/repository"
	"golang.org/x/crypto/bcrypt"
)

type fakeAccountRepository struct {
	account             domain.Account
	updatePasswordCalls int
	updatePasswordErr   error
}

type fakeSessionRepository struct {
	items       map[string]domain.Session
	createCalls int
}

func TestAccountSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	accounts := new(fakeAccountRepository)
	sessions := &fakeSessionRepository{items: make(map[string]domain.Session)}
	service := New(accounts, sessions)

	if err := service.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap() error = %v, want nil", err)
	}

	login, err := service.Login(ctx, "admin", "admin")
	if err != nil {
		t.Fatalf("Login(admin, admin) error = %v, want nil", err)
	}

	identity, err := service.Authenticate(ctx, login.Token)
	if err != nil {
		t.Fatalf("Authenticate(initial token) error = %v, want nil", err)
	}

	if identity.AccountID != initialAdminID || identity.ID != sessionID(login.Token) {
		t.Errorf("Authenticate(initial token) = %+v, want account %d and hashed token ID",
			identity, initialAdminID)
	}

	if err := service.ChangePassword(ctx, initialAdminID, "wrong", "new-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("ChangePassword(wrong current password) error = %v, want ErrInvalidCredentials", err)
	}

	if _, err := service.Authenticate(ctx, login.Token); err != nil {
		t.Errorf("Authenticate(initial token after failed change) error = %v, want nil", err)
	}

	if err := service.ChangePassword(ctx, initialAdminID, "admin", "new-password"); err != nil {
		t.Fatalf("ChangePassword(admin, new-password) error = %v, want nil", err)
	}

	if _, err := service.Authenticate(ctx, login.Token); !errors.Is(err, ErrSessionRequired) {
		t.Errorf("Authenticate(initial token after password change) error = %v, want ErrSessionRequired", err)
	}

	if err := service.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap(existing admin) error = %v, want nil", err)
	}

	if _, err := service.Login(ctx, "admin", "admin"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Login(admin, old password) error = %v, want ErrInvalidCredentials", err)
	}

	login, err = service.Login(ctx, "admin", "new-password")
	if err != nil {
		t.Fatalf("Login(admin, new-password) error = %v, want nil", err)
	}

	if _, err := service.Authenticate(ctx, login.Token); err != nil {
		t.Fatalf("Authenticate(new token) error = %v, want nil", err)
	}

	if err := service.Logout(ctx, sessionID(login.Token)); err != nil {
		t.Fatalf("Logout(new token) error = %v, want nil", err)
	}

	if _, err := service.Authenticate(ctx, login.Token); !errors.Is(err, ErrSessionRequired) {
		t.Errorf("Authenticate(new token after logout) error = %v, want ErrSessionRequired", err)
	}
}

func TestPasswordVerificationRejectsIncorrectPassword(t *testing.T) {
	hash := passwordHash(t, "current-password")
	operations := []struct {
		name string
		run  func(context.Context, *Service, string) error
	}{
		{
			name: "Login",
			run: func(ctx context.Context, service *Service, password string) error {
				_, err := service.Login(ctx, "admin", password)
				return err
			},
		},
		{
			name: "ChangePassword",
			run: func(ctx context.Context, service *Service, password string) error {
				return service.ChangePassword(ctx, initialAdminID, password, "new-password")
			},
		},
	}

	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			for _, password := range []string{"wrong-password", "", strings.Repeat("x", 73)} {
				t.Run("password="+password, func(t *testing.T) {
					account := domain.Account{
						ID:             initialAdminID,
						Username:       "admin",
						PasswordHash:   hash,
						SessionVersion: 7,
					}
					accounts := &fakeAccountRepository{account: account}
					sessions := &fakeSessionRepository{items: make(map[string]domain.Session)}
					service := New(accounts, sessions)

					err := operation.run(t.Context(), service, password)
					if !errors.Is(err, ErrInvalidCredentials) {
						t.Errorf("%s(password=%q) error = %v, want ErrInvalidCredentials", operation.name, password, err)
					}

					if !errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
						t.Errorf("%s(incorrect password) error = %v, want bcrypt mismatch cause", operation.name, err)
					}

					if errors.Is(err, ErrUnavailable) {
						t.Errorf("%s(password=%q) error = %v, want no ErrUnavailable", operation.name, password, err)
					}

					if sessions.createCalls != 0 || len(sessions.items) != 0 {
						t.Errorf("%s(password=%q) created sessions: calls=%d, items=%d, want both 0",
							operation.name, password, sessions.createCalls, len(sessions.items))
					}

					if accounts.updatePasswordCalls != 0 || accounts.account != account {
						t.Errorf("%s(password=%q) changed account: update calls=%d, unchanged=%t, want 0 and true",
							operation.name, password, accounts.updatePasswordCalls, accounts.account == account)
					}
				})
			}
		})
	}
}

func TestPasswordVerificationRejectsMalformedHash(t *testing.T) {
	hash := passwordHash(t, "current-password")
	tests := []struct {
		name      string
		hash      string
		wantCause error
	}{
		{name: "short", hash: "broken", wantCause: bcrypt.ErrHashTooShort},
		{name: "prefix", hash: "!" + hash[1:], wantCause: bcrypt.InvalidHashPrefixError('!')},
		{name: "version", hash: "$3" + hash[2:], wantCause: bcrypt.HashVersionTooNewError('3')},
		{name: "cost", hash: hash[:4] + "03" + hash[6:], wantCause: bcrypt.InvalidCostError(3)},
	}
	operations := []struct {
		name        string
		run         func(context.Context, *Service) error
		wantContext string
	}{
		{
			name: "Login",
			run: func(ctx context.Context, service *Service) error {
				_, err := service.Login(ctx, "admin", "current-password")
				return err
			},
			wantContext: "compare password",
		},
		{
			name: "ChangePassword",
			run: func(ctx context.Context, service *Service) error {
				return service.ChangePassword(ctx, initialAdminID, "current-password", "new-password")
			},
			wantContext: "compare current password",
		},
	}

	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					account := domain.Account{
						ID:             initialAdminID,
						Username:       "admin",
						PasswordHash:   tt.hash,
						SessionVersion: 7,
					}
					accounts := &fakeAccountRepository{account: account}
					sessions := &fakeSessionRepository{items: make(map[string]domain.Session)}
					service := New(accounts, sessions)

					err := operation.run(t.Context(), service)
					if !errors.Is(err, ErrUnavailable) {
						t.Errorf("%s(hash=%q) error = %v, want ErrUnavailable", operation.name, tt.hash, err)
					}

					if errors.Is(err, ErrInvalidCredentials) {
						t.Errorf("%s(hash=%q) error = %v, want no ErrInvalidCredentials", operation.name, tt.hash, err)
					}

					if !errors.Is(err, tt.wantCause) {
						t.Errorf("%s(hash=%q) error = %v, want cause %v", operation.name, tt.hash, err, tt.wantCause)
					}

					if err != nil && !strings.Contains(err.Error(), operation.wantContext) {
						t.Errorf("%s(hash=%q) error = %v, want context %q", operation.name, tt.hash, err, operation.wantContext)
					}

					if sessions.createCalls != 0 || len(sessions.items) != 0 {
						t.Errorf("%s(hash=%q) created sessions: calls=%d, items=%d, want both 0",
							operation.name, tt.hash, sessions.createCalls, len(sessions.items))
					}

					if accounts.updatePasswordCalls != 0 || accounts.account != account {
						t.Errorf("%s(hash=%q) changed account: update calls=%d, unchanged=%t, want 0 and true",
							operation.name, tt.hash, accounts.updatePasswordCalls, accounts.account == account)
					}
				})
			}
		})
	}
}

func TestMissingAuthenticationRecordsPreserveCause(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(make([]byte, sessionTokenSize))
	tests := []struct {
		name      string
		run       func(context.Context, *Service) error
		wantErr   error
		wantCause error
	}{
		{
			name: "login account missing",
			run: func(ctx context.Context, service *Service) error {
				_, err := service.Login(ctx, "admin", "password")
				return err
			},
			wantErr:   ErrInvalidCredentials,
			wantCause: repository.ErrAccountNotFound,
		},
		{
			name: "authenticated account missing",
			run: func(ctx context.Context, service *Service) error {
				_, err := service.Authenticate(ctx, token)
				return err
			},
			wantErr:   ErrSessionRequired,
			wantCause: repository.ErrAccountNotFound,
		},
		{
			name: "session missing",
			run: func(ctx context.Context, service *Service) error {
				otherToken := base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("x", sessionTokenSize)))
				_, err := service.Authenticate(ctx, otherToken)

				return err
			},
			wantErr:   ErrSessionRequired,
			wantCause: repository.ErrSessionNotFound,
		},
		{
			name: "current account missing",
			run: func(ctx context.Context, service *Service) error {
				_, err := service.Get(ctx, initialAdminID)
				return err
			},
			wantErr:   ErrSessionRequired,
			wantCause: repository.ErrAccountNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accounts := new(fakeAccountRepository)
			sessions := &fakeSessionRepository{items: map[string]domain.Session{
				sessionID(token): {AccountID: initialAdminID, AccountVersion: 1},
			}}
			service := New(accounts, sessions)

			err := tt.run(t.Context(), service)

			if !errors.Is(err, tt.wantErr) || !errors.Is(err, tt.wantCause) {
				t.Errorf("%s error = %v, want %v and cause %v", tt.name, err, tt.wantErr, tt.wantCause)
			}
		})
	}
}

func TestChangePasswordPreservesConflictCause(t *testing.T) {
	cause := fmt.Errorf("concurrent password update: %w", repository.ErrAccountChanged)
	accounts := &fakeAccountRepository{
		account: domain.Account{
			ID:           initialAdminID,
			Username:     "admin",
			PasswordHash: passwordHash(t, "current-password"),
		},
		updatePasswordErr: cause,
	}
	service := New(accounts, &fakeSessionRepository{items: make(map[string]domain.Session)})

	err := service.ChangePassword(t.Context(), initialAdminID, "current-password", "new-password")

	if !errors.Is(err, ErrInvalidCredentials) || !errors.Is(err, cause) {
		t.Errorf("ChangePassword(concurrent update) error = %v, want ErrInvalidCredentials and original cause", err)
	}
}

func (r *fakeAccountRepository) Create(_ context.Context, account domain.Account) error {
	if r.account.ID != 0 {
		return repository.ErrAccountAlreadyExists
	}

	r.account = account

	return nil
}

func (r *fakeAccountRepository) Get(_ context.Context, id uint64) (domain.Account, error) {
	if r.account.ID != id {
		return domain.Account{}, repository.ErrAccountNotFound
	}

	return r.account, nil
}

func (r *fakeAccountRepository) GetByUsername(_ context.Context, username string) (domain.Account, error) {
	if r.account.Username != username {
		return domain.Account{}, repository.ErrAccountNotFound
	}

	return r.account, nil
}

func (r *fakeAccountRepository) UpdatePassword(_ context.Context, id uint64, oldHash, newHash string) error {
	r.updatePasswordCalls++
	if r.updatePasswordErr != nil {
		return r.updatePasswordErr
	}

	if r.account.ID != id || r.account.PasswordHash != oldHash {
		return repository.ErrAccountChanged
	}

	r.account.PasswordHash = newHash
	r.account.SessionVersion++

	return nil
}

func (r *fakeSessionRepository) Create(_ context.Context, id string, session domain.Session, _ time.Time) error {
	r.createCalls++
	r.items[id] = session

	return nil
}

func (r *fakeSessionRepository) Get(_ context.Context, id string) (domain.Session, error) {
	session, ok := r.items[id]
	if !ok {
		return domain.Session{}, repository.ErrSessionNotFound
	}

	return session, nil
}

func (r *fakeSessionRepository) Delete(_ context.Context, id string) error {
	delete(r.items, id)
	return nil
}

func passwordHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword(test password, MinCost) error = %v, want nil", err)
	}

	return string(hash)
}
