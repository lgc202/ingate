package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/go-cmp/cmp"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestGetGatewayReadsListenersInOneTransaction(t *testing.T) {
	store, mock := newResourceTestStore(t)
	createdAt := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery("FROM `gateways` WHERE name = ").WillReturnRows(
		sqlmock.NewRows([]string{"id", "name", "display_name", "version", "created_at", "updated_at"}).
			AddRow(1, "edge", "Edge", "7", createdAt, createdAt),
	)
	mock.ExpectQuery("FROM `gateway_listeners` WHERE gateway_id =").WillReturnRows(
		sqlmock.NewRows([]string{"id", "gateway_id", "name", "protocol", "port", "hostname"}).
			AddRow(2, 1, "http", "HTTP", 8080, "example.com"),
	)
	mock.ExpectCommit()

	gateway, err := store.GetGateway(context.Background(), "edge")
	if err != nil {
		t.Fatalf("GetGateway(edge): %v", err)
	}
	if gateway.Version != "7" || len(gateway.Listeners) != 1 {
		t.Errorf("GetGateway(edge) = %+v, want version 7 and one listener", gateway)
		return
	}
	if gateway.Listeners[0].Name != "http" || gateway.Listeners[0].Port != 8080 {
		t.Errorf("GetGateway(edge) listener = %+v, want http on port 8080", gateway.Listeners[0])
	}
}

func TestListBackendsOrdersEndpointsByAddressAndPort(t *testing.T) {
	store, mock := newResourceTestStore(t)
	createdAt := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `backends`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("FROM `backends` ORDER BY name LIMIT").WillReturnRows(
		sqlmock.NewRows([]string{"id", "name", "version", "created_at", "updated_at"}).
			AddRow(1, "backend", "9", createdAt, createdAt),
	)
	mock.ExpectQuery("FROM `backend_endpoints` WHERE backend_id IN .*ORDER BY backend_id, address, port").WillReturnRows(
		sqlmock.NewRows([]string{"id", "backend_id", "address", "port", "weight"}).
			AddRow(2, 1, "10.0.0.1", 8080, 1).
			AddRow(3, 1, "10.0.0.1", 9090, 2).
			AddRow(4, 1, "10.0.0.2", 8080, 1),
	)
	mock.ExpectCommit()

	page, err := store.ListBackends(context.Background(), ResourceListParams{PageSize: 20, PageNum: 1})
	if err != nil {
		t.Fatalf("ListBackends(): %v", err)
	}
	if page.TotalCount != 1 || len(page.Items) != 1 {
		t.Fatalf("ListBackends() = %+v; want one backend", page)
	}

	want := []BackendEndpoint{
		{ID: 2, BackendID: 1, Address: "10.0.0.1", Port: 8080, Weight: 1},
		{ID: 3, BackendID: 1, Address: "10.0.0.1", Port: 9090, Weight: 2},
		{ID: 4, BackendID: 1, Address: "10.0.0.2", Port: 8080, Weight: 1},
	}
	if diff := cmp.Diff(want, page.Items[0].Endpoints); diff != "" {
		t.Errorf("ListBackends() endpoints mismatch (-want +got):\n%s", diff)
	}
}

func TestGetBackendOrdersEndpointsByAddressAndPort(t *testing.T) {
	store, mock := newResourceTestStore(t)
	createdAt := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery("FROM `backends` WHERE name = ").WillReturnRows(
		sqlmock.NewRows([]string{"id", "name", "version", "created_at", "updated_at"}).
			AddRow(1, "backend", "9", createdAt, createdAt),
	)
	mock.ExpectQuery("FROM `backend_endpoints` WHERE backend_id = .*ORDER BY address, port").WillReturnRows(
		sqlmock.NewRows([]string{"id", "backend_id", "address", "port", "weight"}).
			AddRow(2, 1, "10.0.0.1", 8080, 1).
			AddRow(3, 1, "10.0.0.2", 8080, 2),
	)
	mock.ExpectCommit()

	backend, err := store.GetBackend(context.Background(), "backend")
	if err != nil {
		t.Fatalf("GetBackend(backend): %v", err)
	}

	want := []BackendEndpoint{
		{ID: 2, BackendID: 1, Address: "10.0.0.1", Port: 8080, Weight: 1},
		{ID: 3, BackendID: 1, Address: "10.0.0.2", Port: 8080, Weight: 2},
	}
	if diff := cmp.Diff(want, backend.Endpoints); diff != "" {
		t.Errorf("GetBackend(backend) endpoints mismatch (-want +got):\n%s", diff)
	}
}

func newResourceTestStore(t *testing.T) (*Store, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New(): %v", err)
	}

	t.Cleanup(func() {
		mock.ExpectClose()
		if err := db.Close(); err != nil {
			t.Errorf("Close(): %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("SQL expectations: %v", err)
		}
	})

	gormDB, err := gorm.Open(gormmysql.New(gormmysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open(): %v", err)
	}

	return &Store{db: gormDB}, mock
}
