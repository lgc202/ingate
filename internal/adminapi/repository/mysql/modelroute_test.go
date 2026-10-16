package mysql

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/go-cmp/cmp"
)

func TestListModelRoutesPreservesJSONOrder(t *testing.T) {
	store, mock := newResourceTestStore(t)
	recordReadOnlyTransaction(t, store)
	want := ModelRoutePage{
		TotalCount: 2,
		Items: []ModelRoute{
			{
				ID:          1,
				Namespace:   "team-a",
				Name:        "first",
				DisplayName: "模型路由",
				Parents: []ModelRouteParent{
					{Name: "edge", SectionName: "z"},
					{Name: "edge", SectionName: "a"},
				},
				Rules: []ModelRouteRule{
					{
						Name: "z-rule",
						Matches: []ModelRouteMatch{
							{Headers: []ModelRouteHeaderMatch{{
								Name:  "X-Ai-Eg-Model",
								Type:  "Exact",
								Value: " Model-Z ",
							}}},
							{},
						},
						BackendRefs: []ModelRouteBackendRef{{Name: "z-backend", ModelNameOverride: "provider-model"}},
					},
					{
						Name:        "a-rule",
						Matches:     []ModelRouteMatch{{}},
						BackendRefs: []ModelRouteBackendRef{{Name: "a-backend"}},
					},
				},
			},
			{
				ID:          2,
				Namespace:   "team-a",
				Name:        "second",
				DisplayName: "另一模型路由",
				Parents:     []ModelRouteParent{{Name: "other", SectionName: "http"}},
				Rules: []ModelRouteRule{
					{
						Name: "second-rule",
						Matches: []ModelRouteMatch{{Headers: []ModelRouteHeaderMatch{{
							Name:  "x-ai-eg-model",
							Type:  "Exact",
							Value: "model-a",
						}}}},
						BackendRefs: []ModelRouteBackendRef{{Name: "second-backend"}},
					},
				},
			},
		},
	}

	route := want.Items[0]
	second := want.Items[1]
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `modelroutes` WHERE namespace = \\?").WithArgs("team-a").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery("FROM `modelroutes` WHERE namespace = \\? ORDER BY name LIMIT").WithArgs("team-a", 20).
		WillReturnRows(sqlmock.NewRows([]string{"namespace", "id", "name", "display_name", "parents", "rules"}).
			AddRow("team-a", route.ID, route.Name, route.DisplayName, projectionJSON(t, route.Parents), projectionJSON(t, route.Rules)).
			AddRow("team-a", second.ID, second.Name, second.DisplayName, projectionJSON(t, second.Parents), projectionJSON(t, second.Rules)))
	mock.ExpectCommit()

	got, err := store.ListModelRoutes(t.Context(), ModelRouteListParams{Namespace: "team-a", PageSize: 20, PageNum: 1})
	if err != nil {
		t.Fatalf("ListModelRoutes(page 1) error = %v, want nil", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ListModelRoutes(page 1) mismatch (-want +got):\n%s", diff)
	}
}

func TestGetModelRouteRejectsInvalidJSONWithoutPartialState(t *testing.T) {
	store, mock := newResourceTestStore(t)
	mock.ExpectQuery("FROM `modelroutes` WHERE namespace = \\? AND name =").WithArgs("team-a", "broken", 1).
		WillReturnRows(sqlmock.NewRows([]string{"namespace", "id", "name", "parents", "rules"}).
			AddRow("team-a", 1, "broken", `[{"name":"edge"}]`, `[{"matches":`))

	got, err := store.GetModelRoute(t.Context(), "team-a", "broken")
	if err == nil {
		t.Fatal("GetModelRoute(broken) error = nil, want JSON decoding error")
	}

	if diff := cmp.Diff(ModelRoute{}, got); diff != "" {
		t.Errorf("GetModelRoute(broken) returned partial state (-want +got):\n%s", diff)
	}
}

func TestUpsertModelRouteReplacesJSONAndClearsDisplayName(t *testing.T) {
	store, mock := newResourceTestStore(t)
	route := ModelRoute{
		Namespace: "team-a",
		Name:      "edge",
		Version:   "9",
		CreatedAt: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC),
		Parents:   []ModelRouteParent{{Name: "gateway", SectionName: "http"}},
		Rules: []ModelRouteRule{
			{
				Matches:     []ModelRouteMatch{{}},
				BackendRefs: []ModelRouteBackendRef{{Name: "backend"}},
			},
		},
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT `id` FROM `modelroutes` WHERE namespace = \\? AND name =").WithArgs("team-a", "edge", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(41))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `modelroutes` SET `display_name`=?,`version`=?,`created_at`=?,`updated_at`=?,`parents`=?,`rules`=? WHERE `id` = ?")).
		WithArgs("", "9", route.CreatedAt, sqlmock.AnyArg(), projectionJSON(t, route.Parents), projectionJSON(t, route.Rules), 41).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := store.UpsertModelRoute(t.Context(), route); err != nil {
		t.Errorf("UpsertModelRoute(edge) error = %v, want nil", err)
	}
}

func TestListModelRoutesRejectsInvalidJSONWithoutPartialPage(t *testing.T) {
	store, mock := newResourceTestStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `modelroutes` WHERE namespace = \\?").WithArgs("team-a").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("FROM `modelroutes` WHERE namespace = \\? ORDER BY name LIMIT").WithArgs("team-a", 50).
		WillReturnRows(sqlmock.NewRows([]string{"namespace", "name", "parents", "rules"}).
			AddRow("team-a", "broken", `[{"name":"edge"}]`, `[{"matches":`))
	mock.ExpectRollback()
	got, err := store.ListModelRoutes(t.Context(), ModelRouteListParams{Namespace: "team-a", PageSize: 50, PageNum: 1})
	if err == nil {
		t.Fatal("ListModelRoutes(broken JSON) error = nil, want decoding error")
	}

	if diff := cmp.Diff(ModelRoutePage{}, got); diff != "" {
		t.Errorf("ListModelRoutes(broken JSON) returned partial page (-want +got):\n%s", diff)
	}
}

func TestUpsertModelRouteRollsBackWriteFailure(t *testing.T) {
	store, mock := newResourceTestStore(t)
	wantErr := errors.New("projection write failed")
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT `id` FROM `modelroutes` WHERE namespace = \\? AND name =").WithArgs("team-a", "edge", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(41))
	mock.ExpectExec("UPDATE `modelroutes` SET").WillReturnError(wantErr)
	mock.ExpectRollback()

	err := store.UpsertModelRoute(t.Context(), ModelRoute{Namespace: "team-a", Name: "edge"})
	if !errors.Is(err, wantErr) {
		t.Errorf("UpsertModelRoute(edge) error = %v, want wrapped %v", err, wantErr)
	}
}

func TestListModelRoutesCombinesLiteralNameAndGatewayFilters(t *testing.T) {
	store, mock := newResourceTestStore(t)
	condition := "display_name LIKE ? ESCAPE '!' AND JSON_CONTAINS(parents, JSON_OBJECT('name', ?))"
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `modelroutes` WHERE namespace = ? AND "+condition)).
		WithArgs("team-a", "%订单!%!_!!%", "edge").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `modelroutes` WHERE namespace = ? AND "+condition+" ORDER BY name LIMIT ?")).
		WithArgs("team-a", "%订单!%!_!!%", "edge", 20).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
	mock.ExpectCommit()

	got, err := store.ListModelRoutes(t.Context(), ModelRouteListParams{
		Namespace:   "team-a",
		PageSize:    20,
		PageNum:     1,
		DisplayName: "订单%_!",
		GatewayName: "edge",
	})
	if err != nil {
		t.Fatalf("ListModelRoutes(filtered) error = %v, want nil", err)
	}

	if len(got.Items) != 0 || got.TotalCount != 0 {
		t.Errorf("ListModelRoutes(no match) = %+v, want empty page and total 0", got)
	}
}
