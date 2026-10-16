package mysql

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestUpsertBackendTLSReplacesAndClears(t *testing.T) {
	for _, tt := range []struct {
		name  string
		tls   *BackendTLSSettings
		value any
	}{
		{
			name:  "enable",
			tls:   &BackendTLSSettings{SNI: "service.example.com", WellKnownCACertificates: "System"},
			value: `{"sni":"service.example.com","wellKnownCACertificates":"System"}`,
		},
		{name: "clear", value: nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store, mock := newResourceTestStore(t)
			backend := Backend{
				Namespace: "team-a",
				Name:      "service",
				Version:   "9",
				TLS:       tt.tls,
				CreatedAt: time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC),
			}
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT `id` FROM `backends`").WithArgs("team-a", "service", 1).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(41))
			mock.ExpectExec(regexp.QuoteMeta("UPDATE `backends` SET `display_name`=?,`tls`=?,`version`=?,`created_at`=?,`updated_at`=? WHERE `id` = ?")).
				WithArgs("", tt.value, "9", backend.CreatedAt, sqlmock.AnyArg(), 41).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("DELETE FROM `backend_endpoints`").WithArgs(41).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()

			if err := store.UpsertBackend(t.Context(), backend); err != nil {
				t.Errorf("UpsertBackend(TLS=%+v) error = %v, want nil", tt.tls, err)
			}
		})
	}
}
