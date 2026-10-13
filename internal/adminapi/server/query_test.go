package server_test

import (
	"net/http"
	"net/url"
	"regexp"
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
	query.ExpectQuery("FROM `gateways` WHERE namespace = \\? AND name = ").WithArgs("team-a", "edge", 1).WillReturnRows(
		sqlmock.NewRows([]string{"namespace", "id", "name", "display_name", "version", "created_at", "updated_at"}).
			AddRow("team-a", 1, "edge", "Edge", "7", createdAt, createdAt),
	)
	query.ExpectQuery("FROM `gateway_listeners` WHERE gateway_id =").WithArgs(1).WillReturnRows(
		sqlmock.NewRows([]string{"id", "gateway_id", "name", "protocol", "port", "hostname"}).
			AddRow(1, 1, "http", "HTTP", 8080, "example.com"),
	)
	query.ExpectCommit()

	response := requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/gateways/edge", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var getReply adminv1.GetGatewayReply
	decodeReply(t, response, &getReply)
	if got := getReply.GetGateway(); got.GetName() != "edge" || got.GetVersion() != "7" || len(got.GetListeners()) != 1 {
		t.Errorf("GetGateway(edge) = %v, want edge version 7 with one listener", got)
	}

	query.ExpectBegin()
	query.ExpectQuery("SELECT count\\(\\*\\) FROM `gateways` WHERE namespace = \\?").WithArgs("team-a").WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(1),
	)
	query.ExpectQuery("FROM `gateways` WHERE namespace = \\? ORDER BY name LIMIT").WithArgs("team-a", 50).WillReturnRows(
		sqlmock.NewRows([]string{"namespace", "id", "name", "display_name", "version", "created_at", "updated_at"}).
			AddRow("team-a", 1, "edge", "Edge", "7", createdAt, createdAt),
	)
	query.ExpectQuery("FROM `gateway_listeners` WHERE gateway_id IN").WithArgs(1).WillReturnRows(
		sqlmock.NewRows([]string{"id", "gateway_id", "name", "protocol", "port", "hostname"}).
			AddRow(1, 1, "http", "HTTP", 8080, "example.com"),
	)
	query.ExpectCommit()

	response = requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/gateways", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var listReply adminv1.ListGatewaysReply
	decodeReply(t, response, &listReply)
	if listReply.GetTotalCount() != 1 || len(listReply.GetGateways()) != 1 || listReply.GetGateways()[0].GetVersion() != "7" {
		t.Errorf("ListGateways() = %v, want one Gateway with version 7 and total 1", &listReply)
	}

	query.ExpectBegin()
	query.ExpectQuery("FROM `gateways` WHERE namespace = \\? AND name = ").WithArgs("team-a", "missing", 1).WillReturnRows(
		sqlmock.NewRows([]string{"id", "name"}),
	)
	query.ExpectRollback()

	response = requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/gateways/missing", "")
	requireHTTPStatus(t, response, http.StatusNotFound)
	if actions := apiClient.Actions(); len(actions) != 0 {
		t.Errorf("Gateway read API Server actions = %v, want none", actions)
	}
}

func TestHTTPRouteQueryHTTP(t *testing.T) {
	httpServer, apiClient, query := newQueryTestServer(t)
	createdAt := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	columns := []string{"namespace", "id", "name", "display_name", "version", "created_at", "updated_at", "parents", "rules"}
	parents := `[{"name":"edge","sectionName":"http"}]`
	rules := `[{"matches":[{"pathType":"PathPrefix","pathValue":"/old"}],"backendRefs":[{"name":"backend"}]}]`
	query.ExpectQuery("FROM `httproutes` WHERE namespace = \\? AND name = ").WithArgs("team-a", "public", 1).
		WillReturnRows(sqlmock.NewRows(columns).AddRow("team-a", 1, "public", "订单路由", "7", createdAt, createdAt, parents, rules))
	response := requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/http-routes/public", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var getReply adminv1.GetHTTPRouteReply
	decodeReply(t, response, &getReply)
	got := getReply.GetRoute()
	if got.GetName() != "public" || got.GetDisplayName() != "订单路由" || got.GetVersion() != "7" || len(got.GetRules()) != 1 || len(got.GetParentRefs()) != 1 {
		t.Fatalf("GetHTTPRoute(public) = %v, want public with display name 订单路由, version 7, one rule and one parent", got)
	}
	if got.GetRules()[0].GetMatches()[0].GetPath().GetValue() != "/old" || got.GetRules()[0].GetBackendRefs()[0].GetName() != "backend" {
		t.Errorf("lost rule children: %v", got)
	}

	query.ExpectBegin()
	query.ExpectQuery("SELECT count\\(\\*\\) FROM `httproutes` WHERE namespace = \\?").WithArgs("team-a").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	query.ExpectQuery("FROM `httproutes` WHERE namespace = \\? ORDER BY name LIMIT").WithArgs("team-a", 50).
		WillReturnRows(sqlmock.NewRows(columns).AddRow("team-a", 1, "public", "订单路由", "7", createdAt, createdAt, parents, rules))
	query.ExpectCommit()
	response = requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/http-routes", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var listReply adminv1.ListHTTPRoutesReply
	decodeReply(t, response, &listReply)
	if listReply.GetTotalCount() != 1 || len(listReply.GetRoutes()) != 1 || listReply.GetRoutes()[0].GetVersion() != "7" {
		t.Errorf("ListHTTPRoutes() = %v, want one route with version 7 and total 1", &listReply)
	}

	query.ExpectQuery("FROM `httproutes` WHERE namespace = \\? AND name = ").WithArgs("team-a", "missing", 1).WillReturnRows(sqlmock.NewRows(columns))
	response = requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/http-routes/missing", "")
	requireHTTPStatus(t, response, http.StatusNotFound)
	if actions := apiClient.Actions(); len(actions) != 0 {
		t.Errorf("read API Server actions = %v, want none", actions)
	}
}

func TestBackendQueryHTTP(t *testing.T) {
	httpServer, apiClient, query := newQueryTestServer(t)
	createdAt := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	columns := []string{"namespace", "id", "name", "version", "created_at", "updated_at"}
	endpointColumns := []string{"id", "backend_id", "address", "port", "weight"}

	query.ExpectBegin()
	query.ExpectQuery("FROM `backends` WHERE namespace = \\? AND name = ").WithArgs("team-a", "backend", 1).WillReturnRows(
		sqlmock.NewRows(columns).AddRow("team-a", 1, "backend", "7", createdAt, createdAt),
	)
	query.ExpectQuery("FROM `backend_endpoints` WHERE backend_id =").WithArgs(1).WillReturnRows(
		sqlmock.NewRows(endpointColumns).AddRow(1, 1, "127.0.0.1", 8080, 1),
	)
	query.ExpectCommit()
	response := requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/backends/backend", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var getReply adminv1.GetBackendReply
	decodeReply(t, response, &getReply)
	if got := getReply.GetBackend(); got.GetName() != "backend" || got.GetVersion() != "7" || len(got.GetEndpoints()) != 1 || got.GetEndpoints()[0].GetPort() != 8080 {
		t.Errorf("GetBackend(backend) = %v, want backend with version 7 and one endpoint on port 8080", got)
	}

	query.ExpectBegin()
	query.ExpectQuery("SELECT count\\(\\*\\) FROM `backends` WHERE namespace = \\?").WithArgs("team-a").WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(1),
	)
	query.ExpectQuery("FROM `backends` WHERE namespace = \\? ORDER BY name LIMIT").WithArgs("team-a", 50).WillReturnRows(
		sqlmock.NewRows(columns).AddRow("team-a", 1, "backend", "7", createdAt, createdAt),
	)
	query.ExpectQuery("FROM `backend_endpoints` WHERE backend_id IN").WithArgs(1).WillReturnRows(
		sqlmock.NewRows(endpointColumns).AddRow(1, 1, "127.0.0.1", 8080, 1),
	)
	query.ExpectCommit()
	response = requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/backends", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var listReply adminv1.ListBackendsReply
	decodeReply(t, response, &listReply)
	if listReply.GetTotalCount() != 1 || len(listReply.GetBackends()) != 1 || listReply.GetBackends()[0].GetVersion() != "7" {
		t.Errorf("ListBackends() = %v, want one Backend with version 7 and total 1", &listReply)
	}

	query.ExpectBegin()
	query.ExpectQuery("FROM `backends` WHERE namespace = \\? AND name = ").WithArgs("team-a", "missing", 1).WillReturnRows(sqlmock.NewRows(columns))
	query.ExpectRollback()
	response = requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/backends/missing", "")
	requireHTTPStatus(t, response, http.StatusNotFound)
	if actions := apiClient.Actions(); len(actions) != 0 {
		t.Errorf("Backend read API Server actions = %v, want none", actions)
	}
}

func TestResourceListPaginationHTTP(t *testing.T) {
	httpServer, apiClient, query := newQueryTestServer(t)
	query.ExpectBegin()
	query.ExpectQuery("SELECT count\\(\\*\\) FROM `gateways` WHERE namespace = \\?").WithArgs("team-a").WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(3),
	)
	query.ExpectQuery("FROM `gateways` WHERE namespace = \\? ORDER BY name LIMIT").WithArgs("team-a", 1, 1).WillReturnRows(
		sqlmock.NewRows([]string{"namespace", "id", "name", "display_name", "version", "created_at", "updated_at"}).
			AddRow("team-a", 2, "second", "", "", time.Time{}, time.Time{}),
	)
	query.ExpectQuery("FROM `gateway_listeners` WHERE gateway_id IN").WithArgs(2).WillReturnRows(
		sqlmock.NewRows([]string{"id", "gateway_id", "name", "protocol", "port", "hostname"}),
	)
	query.ExpectCommit()

	response := requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/gateways?page_size=1&page_num=2", "")
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

func TestResourceDisplayNameSearchHTTP(t *testing.T) {
	for _, tc := range []struct{ path, table string }{
		{path: "/api/admin/v1/namespaces/team-a/gateways", table: "gateways"},
		{path: "/api/admin/v1/namespaces/team-a/http-routes", table: "httproutes"},
		{path: "/api/admin/v1/namespaces/team-a/backends", table: "backends"},
		{path: "/api/admin/v1/namespaces/team-a/ai-backends", table: "aibackends"},
		{path: "/api/admin/v1/namespaces/team-a/model-routes", table: "modelroutes"},
		{path: "/api/admin/v1/namespaces/team-a/mcp-routes", table: "mcproutes"},
	} {
		t.Run(tc.table, func(t *testing.T) {
			httpServer, apiClient, query := newQueryTestServer(t)
			query.ExpectBegin()
			query.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `"+tc.table+"` WHERE namespace = ? AND display_name LIKE ? ESCAPE '!'")).
				WithArgs("team-a", "%订单!%!_!!%").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			query.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `"+tc.table+"` WHERE namespace = ? AND display_name LIKE ? ESCAPE '!' ORDER BY name LIMIT ?")).
				WithArgs("team-a", "%订单!%!_!!%", 50).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
			query.ExpectCommit()
			response := requestResource(httpServer, http.MethodGet, tc.path+"?display_name="+url.QueryEscape("订单%_!"), "")
			requireHTTPStatus(t, response, http.StatusOK)
			if actions := apiClient.Actions(); len(actions) != 0 {
				t.Errorf("List(%s, display_name) API actions = %v, want none", tc.path, actions)
			}
		})
	}
}

func TestHTTPRouteGatewaySearchHTTP(t *testing.T) {
	httpServer, _, query := newQueryTestServer(t)
	query.ExpectBegin()
	condition := "display_name LIKE ? ESCAPE '!' AND JSON_CONTAINS(parents, JSON_OBJECT('name', ?))"
	query.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `httproutes` WHERE namespace = ? AND "+condition)).
		WithArgs("team-a", "%订单%", "edge").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	query.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `httproutes` WHERE namespace = ? AND "+condition+" ORDER BY name LIMIT ?")).
		WithArgs("team-a", "%订单%", "edge", 50).WillReturnRows(sqlmock.NewRows([]string{"namespace", "id", "name", "display_name", "parents", "rules"}).
		AddRow("team-a", 1, "route-id", "生产订单路由", `[{"name":"edge"}]`, `[{"matches":[{"pathType":"PathPrefix","pathValue":"/"}],"backendRefs":[{"name":"backend"}]}]`))
	query.ExpectCommit()
	response := requestResource(httpServer, http.MethodGet, "/api/admin/v1/namespaces/team-a/http-routes?display_name="+url.QueryEscape("订单")+"&gateway_name=edge", "")
	requireHTTPStatus(t, response, http.StatusOK)
	var reply adminv1.ListHTTPRoutesReply
	decodeReply(t, response, &reply)
	if reply.GetTotalCount() != 1 || len(reply.GetRoutes()) != 1 || reply.GetRoutes()[0].GetDisplayName() != "生产订单路由" {
		t.Errorf("ListHTTPRoutes(订单, edge) = %v, want one matching route and total 1", &reply)
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
