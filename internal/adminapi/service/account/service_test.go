package account

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lgc202/ingate/internal/adminapi/domain"
	"github.com/lgc202/ingate/internal/adminapi/repository"
)

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

type fakeAccountRepository struct {
	account domain.Account
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
	if r.account.ID != id || r.account.PasswordHash != oldHash {
		return repository.ErrAccountChanged
	}
	r.account.PasswordHash = newHash
	r.account.SessionVersion++
	return nil
}

type fakeSessionRepository struct {
	items map[string]domain.Session
}

func (r *fakeSessionRepository) Create(_ context.Context, id string, session domain.Session, _ time.Time) error {
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
