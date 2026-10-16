package migration

import (
	"errors"
	"strings"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
)

func TestUpRejectsInvalidDSNBeforeOpeningConnection(t *testing.T) {
	const dsn = "invalid-dsn-without-database-separator"
	_, wantCause := mysqldriver.ParseDSN(dsn)
	if wantCause == nil {
		t.Fatal("ParseDSN(invalid DSN) error = nil, want driver parse failure")
	}

	err := Up(t.Context(), dsn)
	if !errors.Is(err, wantCause) {
		t.Errorf("Up(invalid DSN) error = %v, want driver cause %v", err, wantCause)
	}

	if err == nil || !strings.HasPrefix(err.Error(), "parse MySQL DSN: ") {
		t.Errorf("Up(invalid DSN) error = %v, want original parse context", err)
	}
}
