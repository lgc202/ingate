package mysql

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/go-cmp/cmp"
	"gorm.io/gorm"
)

func TestListMCPRoutesPreservesPathsAndParents(t *testing.T) {
	store, mock := newResourceTestStore(t)
	recordReadOnlyTransaction(t, store)
	route := MCPRoute{
		ID:          1,
		Namespace:   "team-a",
		Name:        "tools",
		DisplayName: "工具路由",
		Version:     "9",
		CreatedAt:   time.Date(2026, 10, 12, 1, 2, 3, 0, time.UTC),
		Parents: []MCPRouteParent{
			{Name: "edge", SectionName: "z"},
			{Name: "edge", SectionName: "a"},
		},
		Path:        "/工具/%2F/",
		BackendName: "network",
		BackendPath: "/upstream/",
	}
	columns := []string{"namespace", "id", "name", "display_name", "version", "created_at", "parents", "path", "backend_name", "backend_path"}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `mcproutes` WHERE namespace = \\?").WithArgs("team-a").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("FROM `mcproutes` WHERE namespace = \\? ORDER BY name LIMIT").WithArgs("team-a", 20).
		WillReturnRows(sqlmock.NewRows(columns).AddRow("team-a", route.ID, route.Name, route.DisplayName, route.Version, route.CreatedAt,
			projectionJSON(t, route.Parents), route.Path, route.BackendName, route.BackendPath))
	mock.ExpectCommit()

	got, err := store.ListMCPRoutes(t.Context(), MCPRouteListParams{Namespace: "team-a", PageSize: 20, PageNum: 1})
	if err != nil {
		t.Fatalf("ListMCPRoutes(page 1) error = %v, want nil", err)
	}

	want := MCPRoutePage{Items: []MCPRoute{route}, TotalCount: 1}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ListMCPRoutes(paths and parents) mismatch (-want +got):\n%s", diff)
	}
}

func TestGetMCPRouteRejectsInvalidJSONWithoutPartialState(t *testing.T) {
	store, mock := newResourceTestStore(t)
	mock.ExpectQuery("FROM `mcproutes` WHERE namespace = \\? AND name =").WithArgs("team-a", "broken", 1).
		WillReturnRows(sqlmock.NewRows([]string{"namespace", "name", "parents", "path"}).AddRow("team-a", "broken", `[{"name":`, "/mcp"))
	got, err := store.GetMCPRoute(t.Context(), "team-a", "broken")
	if err == nil {
		t.Fatal("GetMCPRoute(broken) error = nil, want JSON decoding error")
	}

	if diff := cmp.Diff(MCPRoute{}, got); diff != "" {
		t.Errorf("GetMCPRoute(broken) returned partial state (-want +got):\n%s", diff)
	}
}

func TestGetMCPRoutePreservesNotFoundCause(t *testing.T) {
	store, mock := newResourceTestStore(t)
	mock.ExpectQuery("FROM `mcproutes` WHERE namespace = \\? AND name =").WithArgs("team-a", "missing", 1).
		WillReturnRows(sqlmock.NewRows([]string{"name"}))
	got, err := store.GetMCPRoute(t.Context(), "team-a", "missing")
	if !errors.Is(err, ErrNotFound) || !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetMCPRoute(missing) error = %v, want ErrNotFound and GORM cause", err)
	}

	if diff := cmp.Diff(MCPRoute{}, got); diff != "" {
		t.Errorf("GetMCPRoute(missing) returned state (-want +got):\n%s", diff)
	}
}

func TestUpsertMCPRouteReplacesConfigAndClearsDisplayName(t *testing.T) {
	store, mock := newResourceTestStore(t)
	route := MCPRoute{
		Namespace:   "team-a",
		Name:        "tools",
		Version:     "9",
		CreatedAt:   time.Date(2026, 10, 12, 1, 2, 3, 0, time.UTC),
		Parents:     []MCPRouteParent{{Name: "other", SectionName: "http"}},
		Path:        "/",
		BackendName: "new-backend",
		BackendPath: "/new/mcp/",
	}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT `id` FROM `mcproutes` WHERE namespace = \\? AND name =").WithArgs("team-a", "tools", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(41))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `mcproutes` SET `display_name`=?,`version`=?,`created_at`=?,`updated_at`=?,`parents`=?,`path`=?,`backend_name`=?,`backend_path`=? WHERE `id` = ?")).
		WithArgs("", "9", route.CreatedAt, sqlmock.AnyArg(), projectionJSON(t, route.Parents), "/", "new-backend", "/new/mcp/", 41).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := store.UpsertMCPRoute(t.Context(), route); err != nil {
		t.Errorf("UpsertMCPRoute(tools) error = %v, want nil", err)
	}
}

func TestListMCPRoutesRejectsInvalidJSONWithoutPartialPage(t *testing.T) {
	store, mock := newResourceTestStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `mcproutes` WHERE namespace = \\?").WithArgs("team-a").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("FROM `mcproutes` WHERE namespace = \\? ORDER BY name LIMIT").WithArgs("team-a", 50).
		WillReturnRows(sqlmock.NewRows([]string{"namespace", "name", "parents"}).AddRow("team-a", "broken", `[{"name":`))
	mock.ExpectRollback()
	got, err := store.ListMCPRoutes(t.Context(), MCPRouteListParams{Namespace: "team-a", PageSize: 50, PageNum: 1})
	if err == nil {
		t.Fatal("ListMCPRoutes(broken JSON) error = nil, want decoding error")
	}

	if diff := cmp.Diff(MCPRoutePage{}, got); diff != "" {
		t.Errorf("ListMCPRoutes(broken JSON) returned partial page (-want +got):\n%s", diff)
	}
}

func TestUpsertMCPRouteRollsBackWriteFailure(t *testing.T) {
	store, mock := newResourceTestStore(t)
	wantErr := errors.New("projection write failed")
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT `id` FROM `mcproutes` WHERE namespace = \\? AND name =").WithArgs("team-a", "tools", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(41))
	mock.ExpectExec("UPDATE `mcproutes` SET").WillReturnError(wantErr)
	mock.ExpectRollback()
	err := store.UpsertMCPRoute(t.Context(), MCPRoute{Namespace: "team-a", Name: "tools"})
	if !errors.Is(err, wantErr) {
		t.Errorf("UpsertMCPRoute(tools) error = %v, want wrapped %v", err, wantErr)
	}
}

func TestListMCPRoutesCombinesLiteralNameAndGatewayFilters(t *testing.T) {
	store, mock := newResourceTestStore(t)
	condition := "display_name LIKE ? ESCAPE '!' AND JSON_CONTAINS(parents, JSON_OBJECT('name', ?))"
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `mcproutes` WHERE namespace = ? AND "+condition)).
		WithArgs("team-a", "%工具!%!_!!%", "other").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `mcproutes` WHERE namespace = ? AND "+condition+" ORDER BY name LIMIT ? OFFSET ?")).
		WithArgs("team-a", "%工具!%!_!!%", "other", 20, 20).WillReturnRows(sqlmock.NewRows([]string{"name"}))
	mock.ExpectCommit()
	got, err := store.ListMCPRoutes(t.Context(), MCPRouteListParams{
		Namespace:   "team-a",
		PageSize:    20,
		PageNum:     2,
		DisplayName: "工具%_!",
		GatewayName: "other",
	})
	if err != nil {
		t.Fatalf("ListMCPRoutes(filtered) error = %v, want nil", err)
	}

	if len(got.Items) != 0 || got.TotalCount != 0 {
		t.Errorf("ListMCPRoutes(no match) = %+v, want empty page and total 0", got)
	}
}
