package server_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	kratosmiddleware "github.com/go-kratos/kratos/v3/middleware"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	clientfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGatewayQueryHTTP(t *testing.T) {
	httpServer, apiClient, query := newQueryTestServer(t)
	createdAt := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)

	query.ExpectBegin()
	query.ExpectQuery("FROM `gateways` WHERE name = ").WithArgs("edge", 1).WillReturnRows(
		sqlmock.NewRows([]string{"id", "name", "display_name", "resource_version", "created_at", "updated_at"}).
			AddRow(1, "edge", "Edge", "7", createdAt, createdAt),
	)
	query.ExpectQuery("FROM `gateway_listeners` WHERE gateway_id =").WithArgs(1).WillReturnRows(
		sqlmock.NewRows([]string{"id", "gateway_id", "name", "protocol", "port", "hostname"}).
			AddRow(1, 1, "http", "HTTP", 8080, "example.com"),
	)
	query.ExpectCommit()

	response := requestResource(httpServer, http.MethodGet, "/api/admin/v1/gateways/edge", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var getReply adminv1.GetGatewayReply
	decodeReply(t, response, &getReply)
	if got := getReply.GetGateway(); got.GetName() != "edge" || got.GetVersion() != "7" || len(got.GetListeners()) != 1 {
		t.Errorf("GetGateway(edge) = %v, want edge version 7 with one listener", got)
	}

	query.ExpectBegin()
	query.ExpectQuery("SELECT count\\(\\*\\) FROM `gateways`").WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(1),
	)
	query.ExpectQuery("FROM `gateways` ORDER BY name LIMIT").WithArgs(50).WillReturnRows(
		sqlmock.NewRows([]string{"id", "name", "display_name", "resource_version", "created_at", "updated_at"}).
			AddRow(1, "edge", "Edge", "7", createdAt, createdAt),
	)
	query.ExpectQuery("FROM `gateway_listeners` WHERE gateway_id IN").WithArgs(1).WillReturnRows(
		sqlmock.NewRows([]string{"id", "gateway_id", "name", "protocol", "port", "hostname"}).
			AddRow(1, 1, "http", "HTTP", 8080, "example.com"),
	)
	query.ExpectCommit()

	response = requestResource(httpServer, http.MethodGet, "/api/admin/v1/gateways", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var listReply adminv1.ListGatewaysReply
	decodeReply(t, response, &listReply)
	if listReply.GetTotalCount() != 1 || len(listReply.GetGateways()) != 1 {
		t.Errorf("ListGateways() = %v, want one Gateway and total 1", &listReply)
	}

	query.ExpectBegin()
	query.ExpectQuery("FROM `gateways` WHERE name = ").WithArgs("missing", 1).WillReturnRows(
		sqlmock.NewRows([]string{"id", "name"}),
	)
	query.ExpectRollback()

	response = requestResource(httpServer, http.MethodGet, "/api/admin/v1/gateways/missing", "")
	requireHTTPStatus(t, response, http.StatusNotFound)
	if actions := apiClient.Actions(); len(actions) != 0 {
		t.Errorf("Gateway read API Server actions = %v, want none", actions)
	}
}

func TestRouteQueryHTTP(t *testing.T) {
	httpServer, apiClient, query := newQueryTestServer(t)
	createdAt := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	columns := []string{
		"id", "name", "gateway_name", "listener_name", "path_type", "path_value",
		"upstream_name", "resource_version", "created_at", "updated_at",
	}

	query.ExpectQuery("FROM `routes` WHERE name = ").WithArgs("public", 1).WillReturnRows(
		sqlmock.NewRows(columns).AddRow(1, "public", "edge", "http", "PathPrefix", "/old", "backend", "7", createdAt, createdAt),
	)
	response := requestResource(httpServer, http.MethodGet, "/api/admin/v1/routes/public", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var getReply adminv1.GetRouteReply
	decodeReply(t, response, &getReply)
	if got := getReply.GetRoute(); got.GetName() != "public" || got.GetPath().GetValue() != "/old" {
		t.Errorf("GetRoute(public) = %v, want public with path /old", got)
	}

	query.ExpectBegin()
	query.ExpectQuery("SELECT count\\(\\*\\) FROM `routes`").WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(1),
	)
	query.ExpectQuery("FROM `routes` ORDER BY name LIMIT").WithArgs(50).WillReturnRows(
		sqlmock.NewRows(columns).AddRow(1, "public", "edge", "http", "PathPrefix", "/old", "backend", "7", createdAt, createdAt),
	)
	query.ExpectCommit()
	response = requestResource(httpServer, http.MethodGet, "/api/admin/v1/routes", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var listReply adminv1.ListRoutesReply
	decodeReply(t, response, &listReply)
	if listReply.GetTotalCount() != 1 || len(listReply.GetRoutes()) != 1 {
		t.Errorf("ListRoutes() = %v, want one Route and total 1", &listReply)
	}

	query.ExpectQuery("FROM `routes` WHERE name = ").WithArgs("missing", 1).WillReturnRows(sqlmock.NewRows(columns))
	response = requestResource(httpServer, http.MethodGet, "/api/admin/v1/routes/missing", "")
	requireHTTPStatus(t, response, http.StatusNotFound)
	if actions := apiClient.Actions(); len(actions) != 0 {
		t.Errorf("Route read API Server actions = %v, want none", actions)
	}
}

func TestUpstreamQueryHTTP(t *testing.T) {
	httpServer, apiClient, query := newQueryTestServer(t)
	createdAt := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	columns := []string{"id", "name", "resource_version", "created_at", "updated_at"}
	endpointColumns := []string{"id", "upstream_id", "address", "port", "weight"}

	query.ExpectBegin()
	query.ExpectQuery("FROM `upstreams` WHERE name = ").WithArgs("backend", 1).WillReturnRows(
		sqlmock.NewRows(columns).AddRow(1, "backend", "7", createdAt, createdAt),
	)
	query.ExpectQuery("FROM `upstream_endpoints` WHERE upstream_id =").WithArgs(1).WillReturnRows(
		sqlmock.NewRows(endpointColumns).AddRow(1, 1, "127.0.0.1", 8080, 1),
	)
	query.ExpectCommit()
	response := requestResource(httpServer, http.MethodGet, "/api/admin/v1/upstreams/backend", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var getReply adminv1.GetUpstreamReply
	decodeReply(t, response, &getReply)
	if got := getReply.GetUpstream(); got.GetName() != "backend" || len(got.GetEndpoints()) != 1 || got.GetEndpoints()[0].GetPort() != 8080 {
		t.Errorf("GetUpstream(backend) = %v, want backend with one endpoint on port 8080", got)
	}

	query.ExpectBegin()
	query.ExpectQuery("SELECT count\\(\\*\\) FROM `upstreams`").WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(1),
	)
	query.ExpectQuery("FROM `upstreams` ORDER BY name LIMIT").WithArgs(50).WillReturnRows(
		sqlmock.NewRows(columns).AddRow(1, "backend", "7", createdAt, createdAt),
	)
	query.ExpectQuery("FROM `upstream_endpoints` WHERE upstream_id IN").WithArgs(1).WillReturnRows(
		sqlmock.NewRows(endpointColumns).AddRow(1, 1, "127.0.0.1", 8080, 1),
	)
	query.ExpectCommit()
	response = requestResource(httpServer, http.MethodGet, "/api/admin/v1/upstreams", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var listReply adminv1.ListUpstreamsReply
	decodeReply(t, response, &listReply)
	if listReply.GetTotalCount() != 1 || len(listReply.GetUpstreams()) != 1 {
		t.Errorf("ListUpstreams() = %v, want one Upstream and total 1", &listReply)
	}

	query.ExpectBegin()
	query.ExpectQuery("FROM `upstreams` WHERE name = ").WithArgs("missing", 1).WillReturnRows(sqlmock.NewRows(columns))
	query.ExpectRollback()
	response = requestResource(httpServer, http.MethodGet, "/api/admin/v1/upstreams/missing", "")
	requireHTTPStatus(t, response, http.StatusNotFound)
	if actions := apiClient.Actions(); len(actions) != 0 {
		t.Errorf("Upstream read API Server actions = %v, want none", actions)
	}
}

func TestResourceListPaginationHTTP(t *testing.T) {
	httpServer, apiClient, query := newQueryTestServer(t)
	query.ExpectBegin()
	query.ExpectQuery("SELECT count\\(\\*\\) FROM `gateways`").WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(3),
	)
	query.ExpectQuery("FROM `gateways` ORDER BY name LIMIT").WithArgs(1, 1).WillReturnRows(
		sqlmock.NewRows([]string{"id", "name", "display_name", "resource_version", "created_at", "updated_at"}).
			AddRow(2, "second", "", "", time.Time{}, time.Time{}),
	)
	query.ExpectQuery("FROM `gateway_listeners` WHERE gateway_id IN").WithArgs(2).WillReturnRows(
		sqlmock.NewRows([]string{"id", "gateway_id", "name", "protocol", "port", "hostname"}),
	)
	query.ExpectCommit()

	response := requestResource(httpServer, http.MethodGet, "/api/admin/v1/gateways?page_size=1&page_num=2", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var reply adminv1.ListGatewaysReply
	decodeReply(t, response, &reply)
	if len(reply.GetGateways()) != 1 || reply.GetTotalCount() != 3 {
		t.Errorf("ListGateways(page 2) = %v, want one Gateway and total 3", &reply)
		return
	}
	if got := reply.GetGateways()[0].GetName(); got != "second" {
		t.Errorf("ListGateways(page 2) name = %q, want second", got)
	}
	if actions := apiClient.Actions(); len(actions) != 0 {
		t.Errorf("ListGateways(page 2) API Server actions = %v, want none", actions)
	}
}

func newQueryTestServer(t *testing.T) (*kratoshttp.Server, *clientfake.Clientset, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		mock.ExpectClose()
		if err := db.Close(); err != nil {
			t.Errorf("SQL pool Close() error = %v, want nil", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("SQL expectations error = %v, want nil", err)
		}
	})

	gormDB, err := gorm.Open(gormmysql.New(gormmysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v, want nil", err)
	}
	store := mysql.New(gormDB)

	allow := func(next kratosmiddleware.Handler) kratosmiddleware.Handler { return next }
	httpServer, _, apiClient := newTestServersWithStore(allow, store)
	return httpServer, apiClient, mock
}
