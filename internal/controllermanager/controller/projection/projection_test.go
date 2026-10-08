package projection

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/go-cmp/cmp"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// 三种投影共享恢复契约；SQL 期望分别固定各自的字段与子记录。
type projectionTestCase struct {
	name            string
	table           string
	initialSyncName string
	object          client.Object
	newReconciler   func(client.Client, *mysql.Store) reconcile.Reconciler
	expectCreate    func(sqlmock.Sqlmock, string, string)
	expectUpdate    func(sqlmock.Sqlmock, string, string)
	expectDelete    func(sqlmock.Sqlmock, string)
	expectDeleteErr func(sqlmock.Sqlmock, string, error)
}

func TestReconcileProjectsVersionAfterStatusOnlyUpdate(t *testing.T) {
	for _, tc := range projectionTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			store, mock := newProjectionTestStore(t)
			apiClient := newProjectionTestClient(t, tc.object)
			r := tc.newReconciler(apiClient, store)
			tc.expectUpdate(mock, tc.object.GetName(), "8")

			result, err := r.Reconcile(t.Context(), projectionRequest(tc.object.GetName()))
			if err != nil {
				t.Fatalf("Reconcile(%q) error = %v, want nil", tc.object.GetName(), err)
			}
			if diff := cmp.Diff(ctrl.Result{}, result); diff != "" {
				t.Errorf("Reconcile(%q) result mismatch (-want +got):\n%s", tc.object.GetName(), diff)
			}

			current := tc.object.DeepCopyObject().(client.Object)
			if err := apiClient.Get(t.Context(), client.ObjectKeyFromObject(current), current); err != nil {
				t.Fatalf("Get(%q) error = %v, want nil", current.GetName(), err)
			}
			condition := metav1.Condition{
				Type: "Accepted", Status: metav1.ConditionTrue, Reason: "Accepted",
				ObservedGeneration: current.GetGeneration(), LastTransitionTime: metav1.Now(),
			}
			switch object := current.(type) {
			case *gatewayv1alpha1.Gateway:
				object.Status.Conditions = []metav1.Condition{condition}
			case *gatewayv1alpha1.Route:
				object.Status.Conditions = []metav1.Condition{condition}
			case *gatewayv1alpha1.Upstream:
				object.Status.Conditions = []metav1.Condition{condition}
			}
			if err := apiClient.Status().Update(t.Context(), current); err != nil {
				t.Fatalf("Status.Update(%q) error = %v, want nil", current.GetName(), err)
			}
			if current.GetResourceVersion() == "8" || current.GetGeneration() != tc.object.GetGeneration() {
				t.Fatalf("Status.Update(%q) version/generation = %q/%d, want new version and generation %d",
					current.GetName(), current.GetResourceVersion(), current.GetGeneration(), tc.object.GetGeneration())
			}

			tc.expectUpdate(mock, current.GetName(), current.GetResourceVersion())
			if _, err := r.Reconcile(t.Context(), projectionRequest(current.GetName())); err != nil {
				t.Errorf("Reconcile(%q, status-only update) error = %v, want nil", current.GetName(), err)
			}
		})
	}
}

func TestReconcileDeletesMissingResource(t *testing.T) {
	for _, tc := range projectionTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			store, mock := newProjectionTestStore(t)
			r := tc.newReconciler(newProjectionTestClient(t), store)
			tc.expectDelete(mock, "deleted")

			if _, err := r.Reconcile(t.Context(), projectionRequest("deleted")); err != nil {
				t.Errorf("Reconcile(deleted) error = %v, want nil", err)
			}
		})
	}
}

func TestInitialSyncFillsCurrentResourcesBeforeDeletingStaleRows(t *testing.T) {
	for _, tc := range projectionTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			store, mock := newProjectionTestStore(t)
			r := tc.newReconciler(newProjectionTestClient(t, tc.object), store)
			tc.expectCreate(mock, tc.object.GetName(), "8")
			expectProjectedNames(mock, tc.table, tc.object.GetName(), "stale")
			tc.expectDelete(mock, "stale")

			if _, err := r.Reconcile(t.Context(), projectionRequest(tc.initialSyncName)); err != nil {
				t.Errorf("Reconcile(%q) error = %v, want nil", tc.initialSyncName, err)
			}
		})
	}
}

func TestInitialSyncClearsStaleRowsWhenSourceIsEmpty(t *testing.T) {
	for _, tc := range projectionTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			store, mock := newProjectionTestStore(t)
			r := tc.newReconciler(newProjectionTestClient(t), store)
			expectProjectedNames(mock, tc.table, "stale-a", "stale-b")
			tc.expectDelete(mock, "stale-a")
			tc.expectDelete(mock, "stale-b")

			if _, err := r.Reconcile(t.Context(), projectionRequest(tc.initialSyncName)); err != nil {
				t.Errorf("Reconcile(%q, empty source) error = %v, want nil", tc.initialSyncName, err)
			}
		})
	}
}

func TestReconcileReadFailureDoesNotDeleteProjection(t *testing.T) {
	for _, tc := range projectionTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			store, _ := newProjectionTestStore(t)
			readErr := errors.New("API read unavailable")
			apiClient := newProjectionTestClient(t)
			failingClient := interceptor.NewClient(apiClient, interceptor.Funcs{
				Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
					return readErr
				},
			})
			r := tc.newReconciler(failingClient, store)

			_, err := r.Reconcile(t.Context(), projectionRequest("current"))
			if !errors.Is(err, readErr) {
				t.Errorf("Reconcile(current) error = %v, want API read failure", err)
			}
		})
	}
}

func TestInitialSyncListFailureDoesNotModifyProjection(t *testing.T) {
	for _, tc := range projectionTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			store, _ := newProjectionTestStore(t)
			listErr := errors.New("API list unavailable")
			apiClient := newProjectionTestClient(t)
			failingClient := interceptor.NewClient(apiClient, interceptor.Funcs{
				List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
					return listErr
				},
			})
			r := tc.newReconciler(failingClient, store)

			_, err := r.Reconcile(t.Context(), projectionRequest(tc.initialSyncName))
			if !errors.Is(err, listErr) {
				t.Errorf("Reconcile(%q) error = %v, want API list failure", tc.initialSyncName, err)
			}
		})
	}
}

func TestInitialSyncUpsertFailureStopsCleanupAndCanRetry(t *testing.T) {
	for _, tc := range projectionTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			store, mock := newProjectionTestStore(t)
			r := tc.newReconciler(newProjectionTestClient(t, tc.object), store)
			writeErr := errors.New("MySQL write unavailable")
			expectProjectionLookupFailure(mock, tc.table, tc.object.GetName(), writeErr)

			_, err := r.Reconcile(t.Context(), projectionRequest(tc.initialSyncName))
			if !errors.Is(err, writeErr) {
				t.Fatalf("Reconcile(%q) error = %v, want MySQL failure", tc.initialSyncName, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("failed initial sync SQL expectations error = %v, want nil", err)
			}

			tc.expectCreate(mock, tc.object.GetName(), "8")
			expectProjectedNames(mock, tc.table, tc.object.GetName(), "stale")
			tc.expectDelete(mock, "stale")
			if _, err := r.Reconcile(t.Context(), projectionRequest(tc.initialSyncName)); err != nil {
				t.Errorf("Reconcile(%q, retry) error = %v, want nil", tc.initialSyncName, err)
			}
		})
	}
}

func TestInitialSyncCleanupFailureCanRetryAfterPartialProgress(t *testing.T) {
	for _, tc := range projectionTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			store, mock := newProjectionTestStore(t)
			r := tc.newReconciler(newProjectionTestClient(t, tc.object), store)
			deleteErr := errors.New("MySQL delete unavailable")
			tc.expectCreate(mock, tc.object.GetName(), "8")
			expectProjectedNames(mock, tc.table, tc.object.GetName(), "stale-a", "stale-b")
			tc.expectDelete(mock, "stale-a")
			tc.expectDeleteErr(mock, "stale-b", deleteErr)

			_, err := r.Reconcile(t.Context(), projectionRequest(tc.initialSyncName))
			if !errors.Is(err, deleteErr) {
				t.Fatalf("Reconcile(%q) error = %v, want cleanup failure", tc.initialSyncName, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("partially completed initial sync SQL expectations error = %v, want nil", err)
			}

			tc.expectUpdate(mock, tc.object.GetName(), "8")
			expectProjectedNames(mock, tc.table, tc.object.GetName(), "stale-b")
			tc.expectDelete(mock, "stale-b")
			if _, err := r.Reconcile(t.Context(), projectionRequest(tc.initialSyncName)); err != nil {
				t.Errorf("Reconcile(%q, retry after partial cleanup) error = %v, want nil", tc.initialSyncName, err)
			}
		})
	}
}

func projectionTestCases() []projectionTestCase {
	createdAt := time.Date(2026, 9, 26, 1, 2, 3, 0, time.UTC)
	metadata := metav1.ObjectMeta{Name: "current", ResourceVersion: "8", Generation: 1, CreationTimestamp: metav1.NewTime(createdAt)}
	return []projectionTestCase{
		{
			name: "Gateway", table: "gateways", initialSyncName: gatewayInitialSyncName,
			object: &gatewayv1alpha1.Gateway{
				ObjectMeta: metadata,
				Spec: gatewayv1alpha1.GatewaySpec{
					DisplayName: "Edge",
					Listeners: []gatewayv1alpha1.Listener{
						{Name: "http", Protocol: gatewayv1alpha1.ProtocolHTTP, Port: 8080, Hostname: "example.com"},
					},
				},
			},
			newReconciler: func(c client.Client, s *mysql.Store) reconcile.Reconciler { return NewGateway(c, s) },
			expectCreate: func(mock sqlmock.Sqlmock, name, version string) {
				expectProjectionLookup(mock, "gateways", name, false)
				mock.ExpectExec("INSERT INTO `gateways` (`name`,`display_name`,`resource_version`,`created_at`,`updated_at`) VALUES (?,?,?,?,?)").
					WithArgs(name, "Edge", version, createdAt, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(41, 1))
				expectGatewayListeners(mock)
				mock.ExpectCommit()
			},
			expectUpdate: func(mock sqlmock.Sqlmock, name, version string) {
				expectProjectionLookup(mock, "gateways", name, true)
				mock.ExpectExec("UPDATE `gateways` SET `created_at`=?,`display_name`=?,`resource_version`=?,`updated_at`=? WHERE `id` = ?").
					WithArgs(createdAt, "Edge", version, sqlmock.AnyArg(), 41).WillReturnResult(sqlmock.NewResult(0, 1))
				expectGatewayListeners(mock)
				mock.ExpectCommit()
			},
			expectDelete: func(mock sqlmock.Sqlmock, name string) {
				expectProjectionLookup(mock, "gateways", name, true)
				mock.ExpectExec("DELETE FROM `gateway_listeners` WHERE gateway_id = ?").WithArgs(41).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("DELETE FROM `gateways` WHERE `gateways`.`id` = ?").WithArgs(41).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectDeleteErr: func(mock sqlmock.Sqlmock, name string, err error) {
				expectProjectionLookupFailure(mock, "gateways", name, err)
			},
		},
		{
			name: "Route", table: "routes", initialSyncName: routeInitialSyncName,
			object: &gatewayv1alpha1.Route{
				ObjectMeta: metadata,
				Spec: gatewayv1alpha1.RouteSpec{
					GatewayName: "edge", ListenerName: "http", UpstreamName: "backend",
					Path: gatewayv1alpha1.PathMatch{Type: gatewayv1alpha1.PathMatchPrefix, Value: "/api"},
				},
			},
			newReconciler: func(c client.Client, s *mysql.Store) reconcile.Reconciler { return NewRoute(c, s) },
			expectCreate: func(mock sqlmock.Sqlmock, name, version string) {
				expectProjectionLookup(mock, "routes", name, false)
				mock.ExpectExec("INSERT INTO `routes` (`name`,`gateway_name`,`listener_name`,`path_type`,`path_value`,`upstream_name`,`resource_version`,`created_at`,`updated_at`) VALUES (?,?,?,?,?,?,?,?,?)").
					WithArgs(name, "edge", "http", "PathPrefix", "/api", "backend", version, createdAt, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(41, 1))
				mock.ExpectCommit()
			},
			expectUpdate: func(mock sqlmock.Sqlmock, name, version string) {
				expectProjectionLookup(mock, "routes", name, true)
				mock.ExpectExec("UPDATE `routes` SET `created_at`=?,`gateway_name`=?,`listener_name`=?,`path_type`=?,`path_value`=?,`resource_version`=?,`upstream_name`=?,`updated_at`=? WHERE `id` = ?").
					WithArgs(createdAt, "edge", "http", "PathPrefix", "/api", version, "backend", sqlmock.AnyArg(), 41).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectDelete: func(mock sqlmock.Sqlmock, name string) {
				mock.ExpectBegin()
				mock.ExpectExec("DELETE FROM `routes` WHERE name = ?").WithArgs(name).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectDeleteErr: func(mock sqlmock.Sqlmock, name string, err error) {
				mock.ExpectBegin()
				mock.ExpectExec("DELETE FROM `routes` WHERE name = ?").WithArgs(name).WillReturnError(err)
				mock.ExpectRollback()
			},
		},
		{
			name: "Upstream", table: "upstreams", initialSyncName: upstreamInitialSyncName,
			object: &gatewayv1alpha1.Upstream{
				ObjectMeta: metadata,
				Spec: gatewayv1alpha1.UpstreamSpec{Endpoints: []gatewayv1alpha1.Endpoint{
					{Address: "127.0.0.1", Port: 9090, Weight: 3},
				}},
			},
			newReconciler: func(c client.Client, s *mysql.Store) reconcile.Reconciler { return NewUpstream(c, s) },
			expectCreate: func(mock sqlmock.Sqlmock, name, version string) {
				expectProjectionLookup(mock, "upstreams", name, false)
				mock.ExpectExec("INSERT INTO `upstreams` (`name`,`resource_version`,`created_at`,`updated_at`) VALUES (?,?,?,?)").
					WithArgs(name, version, createdAt, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(41, 1))
				expectUpstreamEndpoints(mock)
				mock.ExpectCommit()
			},
			expectUpdate: func(mock sqlmock.Sqlmock, name, version string) {
				expectProjectionLookup(mock, "upstreams", name, true)
				mock.ExpectExec("UPDATE `upstreams` SET `created_at`=?,`resource_version`=?,`updated_at`=? WHERE `id` = ?").
					WithArgs(createdAt, version, sqlmock.AnyArg(), 41).WillReturnResult(sqlmock.NewResult(0, 1))
				expectUpstreamEndpoints(mock)
				mock.ExpectCommit()
			},
			expectDelete: func(mock sqlmock.Sqlmock, name string) {
				expectProjectionLookup(mock, "upstreams", name, true)
				mock.ExpectExec("DELETE FROM `upstream_endpoints` WHERE upstream_id = ?").WithArgs(41).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("DELETE FROM `upstreams` WHERE `upstreams`.`id` = ?").WithArgs(41).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectDeleteErr: func(mock sqlmock.Sqlmock, name string, err error) {
				expectProjectionLookupFailure(mock, "upstreams", name, err)
			},
		},
	}
}

func newProjectionTestStore(t *testing.T) (*mysql.Store, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		mock.ExpectClose()
		if err := db.Close(); err != nil {
			t.Errorf("SQL pool Close() error = %v, want nil", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("projection SQL expectations error = %v, want nil", err)
		}
	})

	gormDB, err := gorm.Open(gormmysql.New(gormmysql.Config{
		Conn: db, SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v, want nil", err)
	}
	return mysql.New(gormDB), mock
}

func newProjectionTestClient(t *testing.T, objects ...client.Object) client.WithWatch {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := gatewayv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v, want nil", err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&gatewayv1alpha1.Gateway{}, &gatewayv1alpha1.Route{}, &gatewayv1alpha1.Upstream{}).
		WithObjects(objects...).Build()
}

func projectionRequest(name string) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: name}}
}

func expectProjectionLookup(mock sqlmock.Sqlmock, table, name string, exists bool) {
	mock.ExpectBegin()
	rows := sqlmock.NewRows([]string{"id"})
	if exists {
		rows.AddRow(41)
	}
	mock.ExpectQuery("SELECT `id` FROM `"+table+"` WHERE name = ? LIMIT ?").WithArgs(name, 1).WillReturnRows(rows)
}

func expectProjectionLookupFailure(mock sqlmock.Sqlmock, table, name string, err error) {
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT `id` FROM `"+table+"` WHERE name = ? LIMIT ?").WithArgs(name, 1).WillReturnError(err)
	mock.ExpectRollback()
}

func expectProjectedNames(mock sqlmock.Sqlmock, table string, names ...string) {
	rows := sqlmock.NewRows([]string{"name"})
	for _, name := range names {
		rows.AddRow(name)
	}
	mock.ExpectQuery("SELECT `name` FROM `" + table + "` ORDER BY name").WillReturnRows(rows)
}

func expectGatewayListeners(mock sqlmock.Sqlmock) {
	mock.ExpectExec("DELETE FROM `gateway_listeners` WHERE gateway_id = ?").WithArgs(41).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO `gateway_listeners` (`gateway_id`,`name`,`protocol`,`port`,`hostname`) VALUES (?,?,?,?,?)").
		WithArgs(41, "http", "HTTP", 8080, "example.com").WillReturnResult(sqlmock.NewResult(51, 1))
}

func expectUpstreamEndpoints(mock sqlmock.Sqlmock) {
	mock.ExpectExec("DELETE FROM `upstream_endpoints` WHERE upstream_id = ?").WithArgs(41).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO `upstream_endpoints` (`upstream_id`,`address`,`port`,`weight`) VALUES (?,?,?,?)").
		WithArgs(41, "127.0.0.1", 9090, 3).WillReturnResult(sqlmock.NewResult(51, 1))
}
