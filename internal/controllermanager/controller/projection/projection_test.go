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
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// 已实现投影共享恢复契约；SQL 期望分别固定各自的字段与子记录。
type projectionTestCase struct {
	name            string
	table           string
	initialSyncName string
	object          client.Object
	newReconciler   func(client.Client, *mysql.Store) reconcile.Reconciler
	expectCreate    func(sqlmock.Sqlmock, types.NamespacedName, string)
	expectUpdate    func(sqlmock.Sqlmock, types.NamespacedName, string)
	expectDelete    func(sqlmock.Sqlmock, types.NamespacedName)
	expectDeleteErr func(sqlmock.Sqlmock, types.NamespacedName, error)
}

func TestReconcileProjectsVersionAfterStatusOnlyUpdate(t *testing.T) {
	for _, tc := range projectionTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			store, mock := newProjectionTestStore(t)
			apiClient := newProjectionTestClient(t, tc.object)
			r := tc.newReconciler(apiClient, store)
			tc.expectUpdate(mock, client.ObjectKeyFromObject(tc.object), "8")

			result, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(tc.object)})
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
			case *gatewayv1alpha1.HTTPRoute:
				object.Status.Parents = []gatewayv1alpha1.RouteParentStatus{{
					ParentRef:      gatewayv1alpha1.ParentReference{Name: "edge"},
					ControllerName: "gateway.ingate.io/test",
					Conditions:     []metav1.Condition{condition},
				}}
			case *gatewayv1alpha1.Backend:
				object.Status.Conditions = []metav1.Condition{condition}
			case *gatewayv1alpha1.AIBackend:
				object.Status.Conditions = []metav1.Condition{condition}
			case *gatewayv1alpha1.ModelRoute:
				object.Status.Parents = []gatewayv1alpha1.RouteParentStatus{{
					ParentRef:      gatewayv1alpha1.ParentReference{Name: "edge", SectionName: "http"},
					ControllerName: "gateway.ingate.io/test",
					Conditions:     []metav1.Condition{condition},
				}}
			case *gatewayv1alpha1.MCPRoute:
				object.Status.Parents = []gatewayv1alpha1.RouteParentStatus{{
					ParentRef:      gatewayv1alpha1.ParentReference{Name: "edge", SectionName: "http"},
					ControllerName: "gateway.ingate.io/test",
					Conditions:     []metav1.Condition{condition},
				}}
			}
			if err := apiClient.Status().Update(t.Context(), current); err != nil {
				t.Fatalf("Status.Update(%q) error = %v, want nil", current.GetName(), err)
			}
			if current.GetResourceVersion() == "8" || current.GetGeneration() != tc.object.GetGeneration() {
				t.Fatalf("Status.Update(%q) version/generation = %q/%d, want new version and generation %d",
					current.GetName(), current.GetResourceVersion(), current.GetGeneration(), tc.object.GetGeneration())
			}

			tc.expectUpdate(mock, client.ObjectKeyFromObject(current), current.GetResourceVersion())
			if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(current)}); err != nil {
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
			tc.expectDelete(mock, types.NamespacedName{Namespace: "team-a", Name: "deleted"})

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
			tc.expectCreate(mock, client.ObjectKeyFromObject(tc.object), "8")
			expectProjectedKeys(
				mock, tc.table,
				client.ObjectKeyFromObject(tc.object),
				types.NamespacedName{Namespace: "team-a", Name: "stale"},
			)
			tc.expectDelete(mock, types.NamespacedName{Namespace: "team-a", Name: "stale"})

			if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Name: tc.initialSyncName}}); err != nil {
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
			expectProjectedKeys(
				mock, tc.table,
				types.NamespacedName{Namespace: "team-a", Name: "stale-a"},
				types.NamespacedName{Namespace: "team-a", Name: "stale-b"},
			)
			tc.expectDelete(mock, types.NamespacedName{Namespace: "team-a", Name: "stale-a"})
			tc.expectDelete(mock, types.NamespacedName{Namespace: "team-a", Name: "stale-b"})

			if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Name: tc.initialSyncName}}); err != nil {
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

			_, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Name: tc.initialSyncName}})
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
			expectProjectionLookupFailure(mock, tc.table, client.ObjectKeyFromObject(tc.object), writeErr)

			_, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Name: tc.initialSyncName}})
			if !errors.Is(err, writeErr) {
				t.Fatalf("Reconcile(%q) error = %v, want MySQL failure", tc.initialSyncName, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("failed initial sync SQL expectations error = %v, want nil", err)
			}

			tc.expectCreate(mock, client.ObjectKeyFromObject(tc.object), "8")
			expectProjectedKeys(
				mock, tc.table,
				client.ObjectKeyFromObject(tc.object),
				types.NamespacedName{Namespace: "team-a", Name: "stale"},
			)
			tc.expectDelete(mock, types.NamespacedName{Namespace: "team-a", Name: "stale"})
			if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Name: tc.initialSyncName}}); err != nil {
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
			tc.expectCreate(mock, client.ObjectKeyFromObject(tc.object), "8")
			expectProjectedKeys(
				mock, tc.table,
				client.ObjectKeyFromObject(tc.object),
				types.NamespacedName{Namespace: "team-a", Name: "stale-a"},
				types.NamespacedName{Namespace: "team-a", Name: "stale-b"},
			)
			tc.expectDelete(mock, types.NamespacedName{Namespace: "team-a", Name: "stale-a"})
			tc.expectDeleteErr(mock, types.NamespacedName{Namespace: "team-a", Name: "stale-b"}, deleteErr)

			_, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Name: tc.initialSyncName}})
			if !errors.Is(err, deleteErr) {
				t.Fatalf("Reconcile(%q) error = %v, want cleanup failure", tc.initialSyncName, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("partially completed initial sync SQL expectations error = %v, want nil", err)
			}

			tc.expectUpdate(mock, client.ObjectKeyFromObject(tc.object), "8")
			expectProjectedKeys(
				mock, tc.table,
				client.ObjectKeyFromObject(tc.object),
				types.NamespacedName{Namespace: "team-a", Name: "stale-b"},
			)
			tc.expectDelete(mock, types.NamespacedName{Namespace: "team-a", Name: "stale-b"})
			if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Name: tc.initialSyncName}}); err != nil {
				t.Errorf("Reconcile(%q, retry after partial cleanup) error = %v, want nil", tc.initialSyncName, err)
			}
		})
	}
}

func TestReconcileDoesNotReadSameNameFromOtherNamespace(t *testing.T) {
	for _, tc := range projectionTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			store, mock := newProjectionTestStore(t)
			other := tc.object.DeepCopyObject().(client.Object)
			other.SetNamespace("team-b")
			apiClient := newProjectionTestClient(t, other)
			r := tc.newReconciler(apiClient, store)
			key := types.NamespacedName{Namespace: "team-a", Name: other.GetName()}
			tc.expectDelete(mock, key)

			if _, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: key}); err != nil {
				t.Errorf("Reconcile(%s, only team-b exists) error = %v, want nil", key, err)
			}
		})
	}
}

func TestInitialSyncSeparatesSameNameKeys(t *testing.T) {
	for _, tc := range projectionTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			store, mock := newProjectionTestStore(t)
			other := tc.object.DeepCopyObject().(client.Object)
			other.SetNamespace("team-b")
			r := tc.newReconciler(newProjectionTestClient(t, other), store)
			current := client.ObjectKeyFromObject(other)
			stale := types.NamespacedName{Namespace: "team-a", Name: other.GetName()}
			tc.expectCreate(mock, current, "8")
			expectProjectedKeys(mock, tc.table, stale, current)
			tc.expectDelete(mock, stale)

			request := ctrl.Request{NamespacedName: types.NamespacedName{Name: tc.initialSyncName}}
			if _, err := r.Reconcile(t.Context(), request); err != nil {
				t.Errorf("Reconcile(%s, stale=%s/current=%s) error = %v, want nil", tc.initialSyncName, stale, current, err)
			}
		})
	}
}

func projectionTestCases() []projectionTestCase {
	createdAt := time.Date(2026, 9, 26, 1, 2, 3, 0, time.UTC)
	metadata := metav1.ObjectMeta{
		Namespace:         "team-a",
		Name:              "current",
		ResourceVersion:   "8",
		Generation:        1,
		CreationTimestamp: metav1.NewTime(createdAt),
	}
	parentsJSON := `[{"name":"edge","sectionName":"http"}]`
	rulesJSON := `[{"matches":[{"pathType":"PathPrefix","pathValue":"/api"}],"backendRefs":[{"name":"backend"}]}]`
	modelRulesJSON := `[{"matches":[{"headers":[{"name":"x-ai-eg-model","type":"Exact","value":"model"}]}],"backendRefs":[{"name":"models","modelNameOverride":"provider-model"}]}]`
	return []projectionTestCase{
		{
			name: "ModelRoute", table: "modelroutes", initialSyncName: modelRouteInitialSyncName,
			object: &gatewayv1alpha1.ModelRoute{
				ObjectMeta: metadata,
				Spec: gatewayv1alpha1.ModelRouteSpec{
					DisplayName: "模型路由",
					ParentRefs:  []gatewayv1alpha1.ParentReference{{Name: "edge", SectionName: "http"}},
					Rules: []gatewayv1alpha1.ModelRouteRule{{
						Matches: []gatewayv1alpha1.ModelRouteMatch{{
							Headers: []gatewayv1alpha1.HTTPHeaderMatch{{
								Name:  "x-ai-eg-model",
								Type:  gatewayv1alpha1.HeaderMatchExact,
								Value: "model",
							}},
						}},
						BackendRefs: []gatewayv1alpha1.AIBackendReference{{Name: "models", ModelNameOverride: "provider-model"}},
					}},
				},
			},
			newReconciler: func(c client.Client, s *mysql.Store) reconcile.Reconciler { return NewModelRoute(c, s) },
			expectCreate: func(mock sqlmock.Sqlmock, key types.NamespacedName, version string) {
				expectProjectionLookup(mock, "modelroutes", key, false)
				mock.ExpectExec("INSERT INTO `modelroutes` (`namespace`,`name`,`display_name`,`version`,`created_at`,`updated_at`,`parents`,`rules`) VALUES (?,?,?,?,?,?,?,?)").
					WithArgs(key.Namespace, key.Name, "模型路由", version, createdAt, sqlmock.AnyArg(), parentsJSON, modelRulesJSON).
					WillReturnResult(sqlmock.NewResult(41, 1))
				mock.ExpectCommit()
			},
			expectUpdate: func(mock sqlmock.Sqlmock, key types.NamespacedName, version string) {
				expectProjectionLookup(mock, "modelroutes", key, true)
				mock.ExpectExec("UPDATE `modelroutes` SET `display_name`=?,`version`=?,`created_at`=?,`updated_at`=?,`parents`=?,`rules`=? WHERE `id` = ?").
					WithArgs("模型路由", version, createdAt, sqlmock.AnyArg(), parentsJSON, modelRulesJSON, 41).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectDelete: func(mock sqlmock.Sqlmock, key types.NamespacedName) {
				mock.ExpectBegin()
				mock.ExpectExec("DELETE FROM `modelroutes` WHERE namespace = ? AND name = ?").WithArgs(key.Namespace, key.Name).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectDeleteErr: func(mock sqlmock.Sqlmock, key types.NamespacedName, err error) {
				mock.ExpectBegin()
				mock.ExpectExec("DELETE FROM `modelroutes` WHERE namespace = ? AND name = ?").WithArgs(key.Namespace, key.Name).WillReturnError(err)
				mock.ExpectRollback()
			},
		},
		{
			name: "AIBackend", table: "aibackends", initialSyncName: aiBackendInitialSyncName,
			object: &gatewayv1alpha1.AIBackend{
				ObjectMeta: metadata,
				Spec: gatewayv1alpha1.AIBackendSpec{
					DisplayName: "模型服务",
					BackendRef:  gatewayv1alpha1.BackendReference{Name: "network"},
					Schema:      gatewayv1alpha1.APISchema{Name: gatewayv1alpha1.APISchemaOpenAI, Prefix: "/v1"},
				},
			},
			newReconciler: func(c client.Client, s *mysql.Store) reconcile.Reconciler { return NewAIBackend(c, s) },
			expectCreate: func(mock sqlmock.Sqlmock, key types.NamespacedName, version string) {
				expectProjectionLookup(mock, "aibackends", key, false)
				mock.ExpectExec("INSERT INTO `aibackends` (`namespace`,`name`,`display_name`,`backend_name`,`schema_name`,`schema_prefix`,`version`,`created_at`,`updated_at`) VALUES (?,?,?,?,?,?,?,?,?)").
					WithArgs(key.Namespace, key.Name, "模型服务", "network", "OpenAI", "/v1", version, createdAt, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(41, 1))
				mock.ExpectCommit()
			},
			expectUpdate: func(mock sqlmock.Sqlmock, key types.NamespacedName, version string) {
				expectProjectionLookup(mock, "aibackends", key, true)
				mock.ExpectExec("UPDATE `aibackends` SET `display_name`=?,`backend_name`=?,`schema_name`=?,`schema_prefix`=?,`version`=?,`created_at`=?,`updated_at`=? WHERE `id` = ?").
					WithArgs("模型服务", "network", "OpenAI", "/v1", version, createdAt, sqlmock.AnyArg(), 41).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectDelete: func(mock sqlmock.Sqlmock, key types.NamespacedName) {
				mock.ExpectBegin()
				mock.ExpectExec("DELETE FROM `aibackends` WHERE namespace = ? AND name = ?").WithArgs(key.Namespace, key.Name).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectDeleteErr: func(mock sqlmock.Sqlmock, key types.NamespacedName, err error) {
				mock.ExpectBegin()
				mock.ExpectExec("DELETE FROM `aibackends` WHERE namespace = ? AND name = ?").WithArgs(key.Namespace, key.Name).WillReturnError(err)
				mock.ExpectRollback()
			},
		},
		{
			name: "MCPRoute", table: "mcproutes", initialSyncName: mcpRouteInitialSyncName,
			object: &gatewayv1alpha1.MCPRoute{
				ObjectMeta: metadata,
				Spec: gatewayv1alpha1.MCPRouteSpec{
					DisplayName: "工具路由",
					ParentRefs:  []gatewayv1alpha1.ParentReference{{Name: "edge", SectionName: "http"}},
					Path:        ptr.To("/tools/mcp/"),
					BackendRefs: []gatewayv1alpha1.MCPBackendReference{{Name: "network", Path: ptr.To("/upstream/mcp/")}},
				},
			},
			newReconciler: func(c client.Client, s *mysql.Store) reconcile.Reconciler { return NewMCPRoute(c, s) },
			expectCreate: func(mock sqlmock.Sqlmock, key types.NamespacedName, version string) {
				expectProjectionLookup(mock, "mcproutes", key, false)
				mock.ExpectExec("INSERT INTO `mcproutes` (`namespace`,`name`,`display_name`,`version`,`created_at`,`updated_at`,`parents`,`path`,`backend_name`,`backend_path`) VALUES (?,?,?,?,?,?,?,?,?,?)").
					WithArgs(key.Namespace, key.Name, "工具路由", version, createdAt, sqlmock.AnyArg(), parentsJSON, "/tools/mcp/", "network", "/upstream/mcp/").
					WillReturnResult(sqlmock.NewResult(41, 1))
				mock.ExpectCommit()
			},
			expectUpdate: func(mock sqlmock.Sqlmock, key types.NamespacedName, version string) {
				expectProjectionLookup(mock, "mcproutes", key, true)
				mock.ExpectExec("UPDATE `mcproutes` SET `display_name`=?,`version`=?,`created_at`=?,`updated_at`=?,`parents`=?,`path`=?,`backend_name`=?,`backend_path`=? WHERE `id` = ?").
					WithArgs("工具路由", version, createdAt, sqlmock.AnyArg(), parentsJSON, "/tools/mcp/", "network", "/upstream/mcp/", 41).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectDelete: func(mock sqlmock.Sqlmock, key types.NamespacedName) {
				mock.ExpectBegin()
				mock.ExpectExec("DELETE FROM `mcproutes` WHERE namespace = ? AND name = ?").WithArgs(key.Namespace, key.Name).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectDeleteErr: func(mock sqlmock.Sqlmock, key types.NamespacedName, err error) {
				mock.ExpectBegin()
				mock.ExpectExec("DELETE FROM `mcproutes` WHERE namespace = ? AND name = ?").WithArgs(key.Namespace, key.Name).WillReturnError(err)
				mock.ExpectRollback()
			},
		},
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
			expectCreate: func(mock sqlmock.Sqlmock, key types.NamespacedName, version string) {
				expectProjectionLookup(mock, "gateways", key, false)
				mock.ExpectExec("INSERT INTO `gateways` (`namespace`,`name`,`display_name`,`version`,`created_at`,`updated_at`) VALUES (?,?,?,?,?,?)").
					WithArgs(key.Namespace, key.Name, "Edge", version, createdAt, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(41, 1))
				expectGatewayListeners(mock)
				mock.ExpectCommit()
			},
			expectUpdate: func(mock sqlmock.Sqlmock, key types.NamespacedName, version string) {
				expectProjectionLookup(mock, "gateways", key, true)
				mock.ExpectExec("UPDATE `gateways` SET `created_at`=?,`display_name`=?,`version`=?,`updated_at`=? WHERE `id` = ?").
					WithArgs(createdAt, "Edge", version, sqlmock.AnyArg(), 41).WillReturnResult(sqlmock.NewResult(0, 1))
				expectGatewayListeners(mock)
				mock.ExpectCommit()
			},
			expectDelete: func(mock sqlmock.Sqlmock, key types.NamespacedName) {
				expectProjectionLookup(mock, "gateways", key, true)
				mock.ExpectExec("DELETE FROM `gateway_listeners` WHERE gateway_id = ?").WithArgs(41).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("DELETE FROM `gateways` WHERE `gateways`.`id` = ?").WithArgs(41).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectDeleteErr: func(mock sqlmock.Sqlmock, key types.NamespacedName, err error) {
				expectProjectionLookupFailure(mock, "gateways", key, err)
			},
		},
		{
			name: "HTTPRoute", table: "httproutes", initialSyncName: routeInitialSyncName,
			object: &gatewayv1alpha1.HTTPRoute{
				ObjectMeta: metadata,
				Spec: gatewayv1alpha1.HTTPRouteSpec{
					DisplayName: "订单路由",
					ParentRefs:  []gatewayv1alpha1.ParentReference{{Name: "edge", SectionName: "http"}},
					Rules: []gatewayv1alpha1.HTTPRouteRule{{
						Matches: []gatewayv1alpha1.HTTPRouteMatch{{
							Path: &gatewayv1alpha1.PathMatch{Type: gatewayv1alpha1.PathMatchPrefix, Value: "/api"},
						}},
						BackendRefs: []gatewayv1alpha1.BackendReference{{Name: "backend"}},
					}},
				},
			},
			newReconciler: func(c client.Client, s *mysql.Store) reconcile.Reconciler { return NewHTTPRoute(c, s) },
			expectCreate: func(mock sqlmock.Sqlmock, key types.NamespacedName, version string) {
				expectProjectionLookup(mock, "httproutes", key, false)
				mock.ExpectExec("INSERT INTO `httproutes` (`namespace`,`name`,`display_name`,`version`,`created_at`,`updated_at`,`parents`,`rules`) VALUES (?,?,?,?,?,?,?,?)").
					WithArgs(key.Namespace, key.Name, "订单路由", version, createdAt, sqlmock.AnyArg(), parentsJSON, rulesJSON).
					WillReturnResult(sqlmock.NewResult(41, 1))
				mock.ExpectCommit()
			},
			expectUpdate: func(mock sqlmock.Sqlmock, key types.NamespacedName, version string) {
				expectProjectionLookup(mock, "httproutes", key, true)
				mock.ExpectExec("UPDATE `httproutes` SET `display_name`=?,`version`=?,`created_at`=?,`updated_at`=?,`parents`=?,`rules`=? WHERE `id` = ?").
					WithArgs("订单路由", version, createdAt, sqlmock.AnyArg(), parentsJSON, rulesJSON, 41).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectDelete: func(mock sqlmock.Sqlmock, key types.NamespacedName) {
				mock.ExpectBegin()
				mock.ExpectExec("DELETE FROM `httproutes` WHERE namespace = ? AND name = ?").WithArgs(key.Namespace, key.Name).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectDeleteErr: func(mock sqlmock.Sqlmock, key types.NamespacedName, err error) {
				mock.ExpectBegin()
				mock.ExpectExec("DELETE FROM `httproutes` WHERE namespace = ? AND name = ?").WithArgs(key.Namespace, key.Name).WillReturnError(err)
				mock.ExpectRollback()
			},
		},
		{
			name: "Backend", table: "backends", initialSyncName: backendInitialSyncName,
			object: &gatewayv1alpha1.Backend{
				ObjectMeta: metadata,
				Spec: gatewayv1alpha1.BackendSpec{DisplayName: "订单后端", Endpoints: []gatewayv1alpha1.Endpoint{
					{Address: "127.0.0.1", Port: 9090, Weight: 3},
				}},
			},
			newReconciler: func(c client.Client, s *mysql.Store) reconcile.Reconciler { return NewBackend(c, s) },
			expectCreate: func(mock sqlmock.Sqlmock, key types.NamespacedName, version string) {
				expectProjectionLookup(mock, "backends", key, false)
				mock.ExpectExec("INSERT INTO `backends` (`namespace`,`name`,`display_name`,`version`,`created_at`,`updated_at`) VALUES (?,?,?,?,?,?)").
					WithArgs(key.Namespace, key.Name, "订单后端", version, createdAt, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(41, 1))
				expectBackendEndpoints(mock)
				mock.ExpectCommit()
			},
			expectUpdate: func(mock sqlmock.Sqlmock, key types.NamespacedName, version string) {
				expectProjectionLookup(mock, "backends", key, true)
				mock.ExpectExec("UPDATE `backends` SET `created_at`=?,`display_name`=?,`version`=?,`updated_at`=? WHERE `id` = ?").
					WithArgs(createdAt, "订单后端", version, sqlmock.AnyArg(), 41).WillReturnResult(sqlmock.NewResult(0, 1))
				expectBackendEndpoints(mock)
				mock.ExpectCommit()
			},
			expectDelete: func(mock sqlmock.Sqlmock, key types.NamespacedName) {
				expectProjectionLookup(mock, "backends", key, true)
				mock.ExpectExec("DELETE FROM `backend_endpoints` WHERE backend_id = ?").WithArgs(41).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec("DELETE FROM `backends` WHERE `backends`.`id` = ?").WithArgs(41).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
			expectDeleteErr: func(mock sqlmock.Sqlmock, key types.NamespacedName, err error) {
				expectProjectionLookupFailure(mock, "backends", key, err)
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
		WithStatusSubresource(&gatewayv1alpha1.Gateway{}, &gatewayv1alpha1.HTTPRoute{}, &gatewayv1alpha1.Backend{}, &gatewayv1alpha1.AIBackend{}, &gatewayv1alpha1.ModelRoute{}, &gatewayv1alpha1.MCPRoute{}).
		WithObjects(objects...).Build()
}

func projectionRequest(name string) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "team-a", Name: name}}
}

func expectProjectionLookup(mock sqlmock.Sqlmock, table string, key types.NamespacedName, exists bool) {
	mock.ExpectBegin()
	rows := sqlmock.NewRows([]string{"id"})
	if exists {
		rows.AddRow(41)
	}
	mock.ExpectQuery("SELECT `id` FROM `"+table+"` WHERE namespace = ? AND name = ? LIMIT ?").WithArgs(key.Namespace, key.Name, 1).WillReturnRows(rows)
}

func expectProjectionLookupFailure(mock sqlmock.Sqlmock, table string, key types.NamespacedName, err error) {
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT `id` FROM `"+table+"` WHERE namespace = ? AND name = ? LIMIT ?").WithArgs(key.Namespace, key.Name, 1).WillReturnError(err)
	mock.ExpectRollback()
}

func expectProjectedKeys(mock sqlmock.Sqlmock, table string, keys ...types.NamespacedName) {
	rows := sqlmock.NewRows([]string{"namespace", "name"})
	for _, key := range keys {
		rows.AddRow(key.Namespace, key.Name)
	}
	mock.ExpectQuery("SELECT `namespace`,`name` FROM `" + table + "` ORDER BY namespace, name").WillReturnRows(rows)
}

func expectGatewayListeners(mock sqlmock.Sqlmock) {
	mock.ExpectExec("DELETE FROM `gateway_listeners` WHERE gateway_id = ?").WithArgs(41).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO `gateway_listeners` (`gateway_id`,`name`,`protocol`,`port`,`hostname`) VALUES (?,?,?,?,?)").
		WithArgs(41, "http", "HTTP", 8080, "example.com").WillReturnResult(sqlmock.NewResult(51, 1))
}

func expectBackendEndpoints(mock sqlmock.Sqlmock) {
	mock.ExpectExec("DELETE FROM `backend_endpoints` WHERE backend_id = ?").WithArgs(41).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO `backend_endpoints` (`backend_id`,`address`,`port`,`weight`) VALUES (?,?,?,?)").
		WithArgs(41, "127.0.0.1", 9090, 3).WillReturnResult(sqlmock.NewResult(51, 1))
}
