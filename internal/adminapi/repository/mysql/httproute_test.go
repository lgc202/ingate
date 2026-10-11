package mysql

import (
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/go-cmp/cmp"
)

func TestListHTTPRoutesPreservesJSONOrder(t *testing.T) {
	store, mock := newResourceTestStore(t)
	recordReadOnlyTransaction(t, store)
	want := HTTPRoutePage{
		TotalCount: 2,
		Items: []HTTPRoute{
			{
				ID:          1,
				Name:        "first",
				DisplayName: "订单路由",
				Parents: []HTTPRouteParent{
					{Name: "edge", SectionName: "z"},
					{Name: "edge", SectionName: "a"},
				},
				Rules: []HTTPRouteRule{
					{
						Name: "z-rule",
						Matches: []HTTPRouteMatch{
							{PathType: "Exact", PathValue: "/z"},
							{PathType: "Exact", PathValue: "/a"},
						},
						BackendRefs: []HTTPRouteBackendRef{{Name: "z-backend"}},
					},
					{
						Name:        "a-rule",
						Matches:     []HTTPRouteMatch{{PathType: "PathPrefix", PathValue: "/"}},
						BackendRefs: []HTTPRouteBackendRef{{Name: "a-backend"}},
					},
				},
			},
			{
				ID:          2,
				Name:        "second",
				DisplayName: "另一个路由",
				Parents:     []HTTPRouteParent{{Name: "other", SectionName: "http"}},
				Rules: []HTTPRouteRule{
					{
						Name:        "second-rule",
						Matches:     []HTTPRouteMatch{{PathType: "Exact", PathValue: "/second"}},
						BackendRefs: []HTTPRouteBackendRef{{Name: "second-backend"}},
					},
				},
			},
		},
	}

	route := want.Items[0]
	second := want.Items[1]
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `httproutes`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery("FROM `httproutes` ORDER BY name LIMIT").WithArgs(20).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "display_name", "parents", "rules"}).
			AddRow(route.ID, route.Name, route.DisplayName, projectionJSON(t, route.Parents), projectionJSON(t, route.Rules)).
			AddRow(second.ID, second.Name, second.DisplayName, projectionJSON(t, second.Parents), projectionJSON(t, second.Rules)))
	mock.ExpectCommit()

	got, err := store.ListHTTPRoutes(t.Context(), HTTPRouteListParams{PageSize: 20, PageNum: 1})
	if err != nil {
		t.Fatalf("ListHTTPRoutes(page 1) error = %v, want nil", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ListHTTPRoutes(page 1) mismatch (-want +got):\n%s", diff)
	}
}

func TestGetHTTPRouteRejectsInvalidJSONWithoutPartialState(t *testing.T) {
	store, mock := newResourceTestStore(t)
	mock.ExpectQuery("FROM `httproutes` WHERE name =").WithArgs("broken", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "parents", "rules"}).
			AddRow(1, "broken", `[{"name":"edge"}]`, `[{"matches":`))

	got, err := store.GetHTTPRoute(t.Context(), "broken")
	if err == nil {
		t.Fatal("GetHTTPRoute(broken) error = nil, want JSON decoding error")
	}
	if diff := cmp.Diff(HTTPRoute{}, got); diff != "" {
		t.Errorf("GetHTTPRoute(broken) returned partial state (-want +got):\n%s", diff)
	}
}

func TestUpsertHTTPRouteReplacesJSONAndClearsDisplayName(t *testing.T) {
	store, mock := newResourceTestStore(t)
	route := HTTPRoute{
		Name:      "edge",
		Version:   "9",
		CreatedAt: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC),
		Parents:   []HTTPRouteParent{{Name: "gateway", SectionName: "http"}},
		Rules: []HTTPRouteRule{
			{
				Matches:     []HTTPRouteMatch{{PathType: "PathPrefix", PathValue: "/"}},
				BackendRefs: []HTTPRouteBackendRef{{Name: "backend"}},
			},
		},
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT `id` FROM `httproutes` WHERE name =").WithArgs("edge", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(41))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `httproutes` SET `display_name`=?,`version`=?,`created_at`=?,`updated_at`=?,`parents`=?,`rules`=? WHERE `id` = ?")).
		WithArgs("", "9", route.CreatedAt, sqlmock.AnyArg(), projectionJSON(t, route.Parents), projectionJSON(t, route.Rules), 41).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := store.UpsertHTTPRoute(t.Context(), route); err != nil {
		t.Errorf("UpsertHTTPRoute(edge) error = %v, want nil", err)
	}
}

func TestUpsertHTTPRouteRollsBackWriteFailure(t *testing.T) {
	store, mock := newResourceTestStore(t)
	wantErr := errors.New("projection write failed")
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT `id` FROM `httproutes` WHERE name =").WithArgs("edge", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(41))
	mock.ExpectExec("UPDATE `httproutes` SET").WillReturnError(wantErr)
	mock.ExpectRollback()

	err := store.UpsertHTTPRoute(t.Context(), HTTPRoute{Name: "edge"})
	if !errors.Is(err, wantErr) {
		t.Errorf("UpsertHTTPRoute(edge) error = %v, want wrapped %v", err, wantErr)
	}
}

func TestListHTTPRoutesCombinesLiteralNameAndGatewayFilters(t *testing.T) {
	store, mock := newResourceTestStore(t)
	condition := "display_name LIKE ? ESCAPE '!' AND JSON_CONTAINS(parents, JSON_OBJECT('name', ?))"
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `httproutes` WHERE "+condition)).
		WithArgs("%订单!%!_!!%", "edge").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `httproutes` WHERE "+condition+" ORDER BY name LIMIT ?")).
		WithArgs("%订单!%!_!!%", "edge", 20).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
	mock.ExpectCommit()

	got, err := store.ListHTTPRoutes(t.Context(), HTTPRouteListParams{
		PageSize:    20,
		PageNum:     1,
		DisplayName: "订单%_!",
		GatewayName: "edge",
	})
	if err != nil {
		t.Fatalf("ListHTTPRoutes(filtered) error = %v, want nil", err)
	}
	if len(got.Items) != 0 || got.TotalCount != 0 {
		t.Errorf("ListHTTPRoutes(no match) = %+v, want empty page and total 0", got)
	}
}

func projectionJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(%T) error = %v, want nil", value, err)
	}
	return string(data)
}
