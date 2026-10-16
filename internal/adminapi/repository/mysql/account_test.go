package mysql

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestUpdatePasswordChangesHashAndSessionVersionTogether(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v, want nil", err)
	}

	t.Cleanup(func() {
		mock.ExpectClose()
		if err := db.Close(); err != nil {
			t.Errorf("Close() error = %v, want nil", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("SQL expectations error = %v, want nil", err)
		}
	})

	gormDB, err := gorm.Open(gormmysql.New(gormmysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v, want nil", err)
	}

	store := &Store{db: gormDB}

	query := "UPDATE `accounts` SET .*`session_version`=session_version \\+ 1.* WHERE id = \\? AND password_hash = \\?"
	mock.ExpectExec(query).
		WithArgs("new-hash", sqlmock.AnyArg(), 1, "old-hash").
		WillReturnResult(sqlmock.NewResult(0, 1))
	updated, err := store.UpdatePassword(context.Background(), 1, "old-hash", "new-hash")
	if err != nil || !updated {
		t.Errorf("UpdatePassword(1, old-hash, new-hash) = (%t, %v), want (true, nil)", updated, err)
	}

	mock.ExpectExec(query).
		WithArgs("new-hash", sqlmock.AnyArg(), 1, "old-hash").
		WillReturnResult(sqlmock.NewResult(0, 0))
	updated, err = store.UpdatePassword(context.Background(), 1, "old-hash", "new-hash")
	if err != nil || updated {
		t.Errorf("UpdatePassword(1, old-hash, new-hash) after conflict = (%t, %v), want (false, nil)", updated, err)
	}
}
