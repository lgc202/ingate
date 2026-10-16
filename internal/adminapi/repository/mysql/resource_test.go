package mysql

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/go-cmp/cmp"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// transactionRecorder 只记录测试实际传入 BeginTx 的选项，其他操作交给原连接池。
type transactionRecorder struct {
	*sql.DB
	options []sql.TxOptions
}

// BeginTx 复制实际事务选项，并将事务创建交给原连接池。
func (r *transactionRecorder) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	var options sql.TxOptions
	if opts != nil {
		options = *opts
	}

	r.options = append(r.options, options)

	return r.DB.BeginTx(ctx, opts)
}

func TestGetGatewayReadsListenersInOneTransaction(t *testing.T) {
	store, mock := newResourceTestStore(t)
	recordReadOnlyTransaction(t, store)
	createdAt := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery("FROM `gateways` WHERE namespace = \\? AND name = ").WithArgs("team-a", "edge", 1).WillReturnRows(
		sqlmock.NewRows([]string{"namespace", "id", "name", "display_name", "version", "created_at", "updated_at"}).
			AddRow("team-a", 1, "edge", "Edge", "7", createdAt, createdAt),
	)
	mock.ExpectQuery("FROM `gateway_listeners` WHERE gateway_id =").WithArgs(1).WillReturnRows(
		sqlmock.NewRows([]string{"id", "gateway_id", "name", "protocol", "port", "hostname"}).
			AddRow(2, 1, "http", "HTTP", 8080, "example.com"),
	)
	mock.ExpectCommit()

	gateway, err := store.GetGateway(t.Context(), "team-a", "edge")
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

func TestListGatewaysKeepsListenersWithTheirOwners(t *testing.T) {
	store, mock := newResourceTestStore(t)
	recordReadOnlyTransaction(t, store)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `gateways` WHERE namespace = \\?").WithArgs("team-a").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(11))
	mock.ExpectQuery("FROM `gateways` WHERE namespace = \\? ORDER BY name LIMIT").WithArgs("team-a", 4, 8).
		WillReturnRows(sqlmock.NewRows([]string{"namespace", "id", "name", "version"}).
			AddRow("team-a", 41, "alpha", "rev-7").
			AddRow("team-a", 7, "bravo", "rev-8").
			AddRow("team-a", 89, "charlie", "rev-9"))
	// 子记录按父 ID 排序，不能按主对象在当前页的位置绑定。
	mock.ExpectQuery("FROM `gateway_listeners` WHERE gateway_id IN .*ORDER BY gateway_id, name").
		WithArgs(41, 7, 89).
		WillReturnRows(sqlmock.NewRows([]string{"id", "gateway_id", "name", "protocol", "port", "hostname"}).
			AddRow(6, 7, "http", "HTTP", 9090, "bravo.example.com").
			AddRow(4, 41, "http", "HTTP", 8080, "alpha.example.com").
			AddRow(8, 41, "metrics", "HTTP", 8081, ""))
	mock.ExpectCommit()

	got, err := store.ListGateways(t.Context(), ResourceListParams{Namespace: "team-a", PageSize: 4, PageNum: 3})
	if err != nil {
		t.Fatalf("ListGateways(page 3) error = %v, want nil", err)
	}

	want := GatewayPage{
		TotalCount: 11,
		Items: []Gateway{
			{
				ID:        41,
				Namespace: "team-a",
				Name:      "alpha",
				Version:   "rev-7",
				Listeners: []GatewayListener{
					{
						ID:        4,
						GatewayID: 41,
						Name:      "http",
						Protocol:  "HTTP",
						Port:      8080,
						Hostname:  "alpha.example.com",
					},
					{
						ID:        8,
						GatewayID: 41,
						Name:      "metrics",
						Protocol:  "HTTP",
						Port:      8081,
					},
				},
			},
			{
				ID:        7,
				Namespace: "team-a",
				Name:      "bravo",
				Version:   "rev-8",
				Listeners: []GatewayListener{
					{
						ID:        6,
						GatewayID: 7,
						Name:      "http",
						Protocol:  "HTTP",
						Port:      9090,
						Hostname:  "bravo.example.com",
					},
				},
			},
			{
				ID:        89,
				Namespace: "team-a",
				Name:      "charlie",
				Version:   "rev-9",
			},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ListGateways(page 3) mismatch (-want +got):\n%s", diff)
	}
}

func TestListBackendsGroupsAndOrdersEndpoints(t *testing.T) {
	store, mock := newResourceTestStore(t)
	recordReadOnlyTransaction(t, store)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `backends` WHERE namespace = \\?").WithArgs("team-a").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(11))
	mock.ExpectQuery("FROM `backends` WHERE namespace = \\? ORDER BY name LIMIT").WithArgs("team-a", 4, 8).
		WillReturnRows(sqlmock.NewRows([]string{"namespace", "id", "name", "version"}).
			AddRow("team-a", 41, "alpha", "rev-7").
			AddRow("team-a", 7, "bravo", "rev-8").
			AddRow("team-a", 89, "charlie", "rev-9"))
	mock.ExpectQuery("FROM `backend_endpoints` WHERE backend_id IN .*ORDER BY backend_id, address, port").
		WithArgs(41, 7, 89).
		WillReturnRows(sqlmock.NewRows([]string{"id", "backend_id", "address", "port", "weight"}).
			AddRow(6, 7, "10.0.0.2", 8080, 5).
			AddRow(4, 41, "10.0.0.1", 8080, 1).
			AddRow(8, 41, "10.0.0.1", 9090, 2).
			AddRow(9, 41, "10.0.0.2", 8080, 1))
	mock.ExpectCommit()

	got, err := store.ListBackends(t.Context(), ResourceListParams{Namespace: "team-a", PageSize: 4, PageNum: 3})
	if err != nil {
		t.Fatalf("ListBackends(page 3) error = %v, want nil", err)
	}

	want := BackendPage{
		TotalCount: 11,
		Items: []Backend{
			{
				ID:        41,
				Namespace: "team-a",
				Name:      "alpha",
				Version:   "rev-7",
				Endpoints: []BackendEndpoint{
					{
						ID:        4,
						BackendID: 41,
						Address:   "10.0.0.1",
						Port:      8080,
						Weight:    1,
					},
					{
						ID:        8,
						BackendID: 41,
						Address:   "10.0.0.1",
						Port:      9090,
						Weight:    2,
					},
					{
						ID:        9,
						BackendID: 41,
						Address:   "10.0.0.2",
						Port:      8080,
						Weight:    1,
					},
				},
			},
			{
				ID:        7,
				Namespace: "team-a",
				Name:      "bravo",
				Version:   "rev-8",
				Endpoints: []BackendEndpoint{
					{
						ID:        6,
						BackendID: 7,
						Address:   "10.0.0.2",
						Port:      8080,
						Weight:    5,
					},
				},
			},
			{
				ID:        89,
				Namespace: "team-a",
				Name:      "charlie",
				Version:   "rev-9",
			},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ListBackends(page 3) mismatch (-want +got):\n%s", diff)
	}
}

func TestGetBackendOrdersEndpointsByAddressAndPort(t *testing.T) {
	store, mock := newResourceTestStore(t)
	recordReadOnlyTransaction(t, store)
	createdAt := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery("FROM `backends` WHERE namespace = \\? AND name = ").WithArgs("team-a", "backend", 1).WillReturnRows(
		sqlmock.NewRows([]string{"namespace", "id", "name", "version", "created_at", "updated_at"}).
			AddRow("team-a", 1, "backend", "9", createdAt, createdAt),
	)
	mock.ExpectQuery("FROM `backend_endpoints` WHERE backend_id = .*ORDER BY address, port").WithArgs(1).WillReturnRows(
		sqlmock.NewRows([]string{"id", "backend_id", "address", "port", "weight"}).
			AddRow(2, 1, "10.0.0.1", 8080, 1).
			AddRow(3, 1, "10.0.0.2", 8080, 2),
	)
	mock.ExpectCommit()

	backend, err := store.GetBackend(t.Context(), "team-a", "backend")
	if err != nil {
		t.Fatalf("GetBackend(backend): %v", err)
	}

	want := []BackendEndpoint{
		{
			ID:        2,
			BackendID: 1,
			Address:   "10.0.0.1",
			Port:      8080,
			Weight:    1,
		},
		{
			ID:        3,
			BackendID: 1,
			Address:   "10.0.0.2",
			Port:      8080,
			Weight:    2,
		},
	}
	if diff := cmp.Diff(want, backend.Endpoints); diff != "" {
		t.Errorf("GetBackend(backend) endpoints mismatch (-want +got):\n%s", diff)
	}
}

func TestGetGatewayRollsBackListenerReadFailure(t *testing.T) {
	store, mock := newResourceTestStore(t)
	recordReadOnlyTransaction(t, store)
	wantErr := errors.New("listener read failed")
	mock.ExpectBegin()
	mock.ExpectQuery("FROM `gateways` WHERE namespace = \\? AND name = ").WithArgs("team-a", "edge", 1).
		WillReturnRows(sqlmock.NewRows([]string{"namespace", "id", "name", "version"}).AddRow("team-a", 41, "edge", "rev-7"))
	mock.ExpectQuery("FROM `gateway_listeners` WHERE gateway_id =").WithArgs(41).WillReturnError(wantErr)
	mock.ExpectRollback()

	got, err := store.GetGateway(t.Context(), "team-a", "edge")
	if !errors.Is(err, wantErr) {
		t.Errorf("GetGateway(edge, listener failure) error = %v, want wrapped %v", err, wantErr)
	}

	if diff := cmp.Diff(Gateway{}, got); diff != "" {
		t.Errorf("GetGateway(edge, listener failure) returned partial state (-want +got):\n%s", diff)
	}
}

func recordReadOnlyTransaction(t *testing.T, store *Store) {
	t.Helper()

	db, err := store.db.DB()
	if err != nil {
		t.Fatalf("gorm.DB.DB() error = %v, want nil", err)
	}

	recorder := &transactionRecorder{DB: db}
	// 只改本测试的 GORM 连接池入口，不改变 SQL、事务执行或清理责任。
	store.db.Statement.ConnPool = recorder
	t.Cleanup(func() {
		want := []sql.TxOptions{{ReadOnly: true, Isolation: sql.LevelRepeatableRead}}
		if diff := cmp.Diff(want, recorder.options); diff != "" {
			t.Errorf("read transaction BeginTx options mismatch (-want +got):\n%s", diff)
		}
	})
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
