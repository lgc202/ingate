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

func TestGetAIBackend(t *testing.T) {
	store, mock := newResourceTestStore(t)
	want := AIBackend{
		ID:           41,
		Namespace:    "team-a",
		Name:         "models",
		DisplayName:  "模型服务",
		BackendName:  "network",
		SchemaName:   "OpenAI",
		SchemaPrefix: "/v1",
		Version:      "9",
		CreatedAt:    time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC),
	}
	mock.ExpectQuery("FROM `aibackends` WHERE namespace = \\? AND name =").WithArgs("team-a", "models", 1).
		WillReturnRows(sqlmock.NewRows([]string{"namespace", "id", "name", "display_name", "backend_name", "schema_name", "schema_prefix", "version", "created_at"}).
			AddRow("team-a", want.ID, want.Name, want.DisplayName, want.BackendName, want.SchemaName, want.SchemaPrefix, want.Version, want.CreatedAt))

	got, err := store.GetAIBackend(t.Context(), "team-a", "models")
	if err != nil {
		t.Fatalf("GetAIBackend(models) error = %v, want nil", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("GetAIBackend(models) mismatch (-want +got):\n%s", diff)
	}
}

func TestGetAIBackendNotFound(t *testing.T) {
	store, mock := newResourceTestStore(t)
	mock.ExpectQuery("FROM `aibackends` WHERE namespace = \\? AND name =").WithArgs("team-a", "missing", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
	got, err := store.GetAIBackend(t.Context(), "team-a", "missing")
	if !errors.Is(err, ErrNotFound) || !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetAIBackend(missing) error = %v, want ErrNotFound and GORM cause", err)
	}
	if diff := cmp.Diff(AIBackend{}, got); diff != "" {
		t.Errorf("GetAIBackend(missing) returned partial state (-want +got):\n%s", diff)
	}
}

func TestUpsertAIBackendReplacesEmptyValues(t *testing.T) {
	store, mock := newResourceTestStore(t)
	backend := AIBackend{
		Namespace:   "team-a",
		Name:        "models",
		BackendName: "other-network",
		SchemaName:  "OpenAI",
		Version:     "9",
		CreatedAt:   time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC),
	}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT `id` FROM `aibackends` WHERE namespace = \\? AND name =").WithArgs("team-a", "models", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(41))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `aibackends` SET `display_name`=?,`backend_name`=?,`schema_name`=?,`schema_prefix`=?,`version`=?,`created_at`=?,`updated_at`=? WHERE `id` = ?")).
		WithArgs("", "other-network", "OpenAI", "", "9", backend.CreatedAt, sqlmock.AnyArg(), 41).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := store.UpsertAIBackend(t.Context(), backend); err != nil {
		t.Errorf("UpsertAIBackend(empty editable values) error = %v, want nil", err)
	}
}

func TestUpsertAIBackendRollsBackWriteFailure(t *testing.T) {
	store, mock := newResourceTestStore(t)
	wantErr := errors.New("write unavailable")
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT `id` FROM `aibackends` WHERE namespace = \\? AND name =").WithArgs("team-a", "models", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(41))
	mock.ExpectExec("UPDATE `aibackends` SET").WillReturnError(wantErr)
	mock.ExpectRollback()
	if err := store.UpsertAIBackend(t.Context(), AIBackend{Namespace: "team-a", Name: "models"}); !errors.Is(err, wantErr) {
		t.Errorf("UpsertAIBackend(write failure) error = %v, want wrapped %v", err, wantErr)
	}
}

func TestListAIBackendsFiltersAndPaginates(t *testing.T) {
	store, mock := newResourceTestStore(t)
	recordReadOnlyTransaction(t, store)
	want := AIBackendPage{
		TotalCount: 3,
		Items: []AIBackend{{
			Namespace:    "team-a",
			Name:         "second",
			DisplayName:  "模型%_!",
			BackendName:  "network",
			SchemaName:   "OpenAI",
			SchemaPrefix: "/v1",
			Version:      "9",
		}},
	}
	condition := "display_name LIKE ? ESCAPE '!'"
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `aibackends` WHERE namespace = ? AND "+condition)).
		WithArgs("team-a", "%模型!%!_!!%").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `aibackends` WHERE namespace = ? AND "+condition+" ORDER BY name LIMIT ? OFFSET ?")).
		WithArgs("team-a", "%模型!%!_!!%", 1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"namespace", "name", "display_name", "backend_name", "schema_name", "schema_prefix", "version"}).
			AddRow("team-a", "second", "模型%_!", "network", "OpenAI", "/v1", "9"))
	mock.ExpectCommit()
	got, err := store.ListAIBackends(t.Context(), ResourceListParams{Namespace: "team-a", PageSize: 1, PageNum: 2, DisplayName: "模型%_!"})
	if err != nil {
		t.Fatalf("ListAIBackends(filtered page 2) error = %v, want nil", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ListAIBackends(filtered page 2) mismatch (-want +got):\n%s", diff)
	}
}

func TestListAIBackendsReturnsNoPartialPageOnFailure(t *testing.T) {
	store, mock := newResourceTestStore(t)
	wantErr := errors.New("page unavailable")
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `aibackends` WHERE namespace = ?")).WithArgs("team-a").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectQuery("FROM `aibackends` WHERE namespace = \\? ORDER BY name LIMIT").WithArgs("team-a", 50).WillReturnError(wantErr)
	mock.ExpectRollback()
	got, err := store.ListAIBackends(t.Context(), ResourceListParams{Namespace: "team-a", PageSize: 50, PageNum: 1})
	if !errors.Is(err, wantErr) {
		t.Errorf("ListAIBackends(page failure) error = %v, want wrapped %v", err, wantErr)
	}
	if diff := cmp.Diff(AIBackendPage{}, got); diff != "" {
		t.Errorf("ListAIBackends(page failure) returned partial page (-want +got):\n%s", diff)
	}
}

func TestListAIBackendsNoMatch(t *testing.T) {
	store, mock := newResourceTestStore(t)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `aibackends` WHERE namespace = ? AND display_name LIKE ? ESCAPE '!'")).
		WithArgs("team-a", "%missing%").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `aibackends` WHERE namespace = ? AND display_name LIKE ? ESCAPE '!' ORDER BY name LIMIT ?")).
		WithArgs("team-a", "%missing%", 50).WillReturnRows(sqlmock.NewRows([]string{"name"}))
	mock.ExpectCommit()
	got, err := store.ListAIBackends(t.Context(), ResourceListParams{Namespace: "team-a", PageSize: 50, PageNum: 1, DisplayName: "missing"})
	if err != nil {
		t.Fatalf("ListAIBackends(no match) error = %v, want nil", err)
	}
	if len(got.Items) != 0 || got.TotalCount != 0 {
		t.Errorf("ListAIBackends(no match) = %+v, want empty page and total 0", got)
	}
}
